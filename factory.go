package typemux

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrFactoryNotFound is returned when no factory is found for the given key.
var ErrFactoryNotFound = errors.New("factory not found")

// ErrDataTypeNotSupported is returned when CreateType is called with not supported DATA type.
var ErrDataTypeNotSupported = errors.New("data type not supported")

type factoryFuncAny func(data any) (any, error)

type factoryRegistry interface {
	registerFactory(key any, factory factoryFuncAny)
}

type factoryResolver interface {
	getFactory(key any) (factoryFuncAny, bool)
}

// RegisterFactory registers a factory function that creates values of type T from data of type DATA,
// associated with the given key.
//
// The key can be any comparable type (string, int, custom enum, etc.).
// If a factory for the same key has already been registered, it will be replaced.
func RegisterFactory[KEY comparable, DATA any, T any](reg factoryRegistry, key KEY, factory func(DATA) (T, error)) {
	reg.registerFactory(key, func(data any) (any, error) {
		d, ok := data.(DATA)
		if !ok {
			var zero DATA
			return nil, fmt.Errorf("typemux: %w: %T, got %T", ErrDataTypeNotSupported, zero, data)
		}
		return factory(d)
	})
}

// CreateType looks up a factory by key and uses it to create a value from the provided data.
// It returns ErrFactoryNotFound if no factory is registered for the given key.
func CreateType[KEY comparable, DATA any](reg factoryResolver, key KEY, data DATA) (any, error) {
	factory, ok := reg.getFactory(key)
	if !ok {
		return nil, fmt.Errorf("typemux: %w for key %v", ErrFactoryNotFound, key)
	}
	return factory(data)
}

// JSONFactory returns a factory function that unmarshals JSON data into type T.
// Use with RegisterFactory for convenient JSON-based type creation.
//
// Example:
//
//	RegisterFactory(reg, "user_created", JSONFactory[UserCreated]())
func JSONFactory[T any]() func([]byte) (T, error) {
	return func(data []byte) (T, error) {
		var v T
		return v, json.Unmarshal(data, &v)
	}
}

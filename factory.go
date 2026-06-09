package typemux

import (
	"errors"
	"fmt"
	"reflect"
)

// ErrFactoryNotFound is returned when no factory is registered under the given key
// (regardless of DATA type).
var ErrFactoryNotFound = errors.New("factory not found")

// ErrDataTypeNotSupported is returned when factories are registered under the given
// key, but none of them accept the DATA type passed to CreateType.
var ErrDataTypeNotSupported = errors.New("data type not supported")

type factoryFuncAny func(data any) (any, error)

type factoryResolver interface {
	getFactory(key any, dataType reflect.Type) (factoryFuncAny, bool)
	keyRegistered(key any) bool
}

// CreateType looks up the codec's unmarshal half for (key, DATA-type) and uses
// it to create a value from the provided data.
//
// Returns:
//   - ErrFactoryNotFound if no codec is registered under the key at all
//   - ErrDataTypeNotSupported if the key has codecs but none accepting DATA
//   - ErrUnsupported if the codec's unmarshal half was Unsupported
func CreateType[KEY comparable, DATA any](reg factoryResolver, key KEY, data DATA) (any, error) {
	dataType := reflect.TypeOf((*DATA)(nil)).Elem()
	factory, ok := reg.getFactory(key, dataType)
	if !ok {
		if reg.keyRegistered(key) {
			return nil, fmt.Errorf("typemux: %w: key %v has no factory accepting %v", ErrDataTypeNotSupported, key, dataType)
		}
		return nil, fmt.Errorf("typemux: %w for key %v", ErrFactoryNotFound, key)
	}
	return factory(data)
}

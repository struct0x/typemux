package typemux

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// ErrSerializerNotFound is returned when no serializer is registered for the value's type.
var ErrSerializerNotFound = errors.New("serializer not found")

// ErrKeyTypeMismatch is returned when Serialize is invoked with a KEY type parameter
// that does not match the key type used at registration time.
var ErrKeyTypeMismatch = errors.New("key type mismatch")

// ErrDataTypeMismatch is returned when Serialize is invoked with a DATA type parameter
// that does not match the data type the registered marshal function produces.
var ErrDataTypeMismatch = errors.New("data type mismatch")

type serializerFuncAny func(v any) (any, error)

type serializerEntry struct {
	key any
	fn  serializerFuncAny
}

type serializerRegistry interface {
	registerSerializer(typ reflect.Type, entry serializerEntry)
}

type serializerResolver interface {
	getSerializer(typ reflect.Type) (serializerEntry, bool)
}

// Codec couples a marshal/unmarshal pair for type T over wire format DATA.
// DATA is typically []byte for transport, but can be any type — e.g.,
// map[string]any when bridging to an adapter library that traffics in maps.
type Codec[DATA, T any] struct {
	Marshal   func(T) (DATA, error)
	Unmarshal func(DATA) (T, error)
}

// JSONCodec returns a Codec[[]byte, T] backed by encoding/json.
//
// Example:
//
//	RegisterCodec(reg, "user_created", JSONCodec[UserCreated]())
func JSONCodec[T any]() Codec[[]byte, T] {
	return Codec[[]byte, T]{
		Marshal: func(v T) ([]byte, error) {
			return json.Marshal(v)
		},
		Unmarshal: func(data []byte) (T, error) {
			var v T
			return v, json.Unmarshal(data, &v)
		},
	}
}

// RegisterSerializer registers a marshal function that produces values of
// type DATA from values of type T, associated with the given key.
//
// The key can be any comparable type (string, int, custom enum, etc.).
// If a serializer for the same type T has already been registered, it will
// be replaced.
func RegisterSerializer[KEY comparable, DATA, T any](reg serializerRegistry, key KEY, marshal func(T) (DATA, error)) {
	typ := reflect.TypeOf((*T)(nil)).Elem()
	reg.registerSerializer(typ, serializerEntry{
		key: key,
		fn: func(v any) (any, error) {
			tv, ok := v.(T)
			if !ok {
				var zero T
				return nil, fmt.Errorf("typemux: expected %T, got %T", zero, v)
			}
			return marshal(tv)
		},
	})
}

// codecRegistry is the combined interface satisfied by *Registry, allowing
// RegisterCodec to register both halves of a codec in one call.
type codecRegistry interface {
	factoryRegistry
	serializerRegistry
}

// RegisterCodec registers both halves of a codec for type T over wire format DATA.
//
// It is sugar over RegisterFactory(reg, key, codec.Unmarshal) +
// RegisterSerializer(reg, key, codec.Marshal). Use it when you want a
// round-trip codec registered in a single call.
//
// If a codec for the same key or type has already been registered, both
// halves are replaced.
func RegisterCodec[KEY comparable, DATA, T any](reg codecRegistry, key KEY, codec Codec[DATA, T]) {
	RegisterFactory(reg, key, codec.Unmarshal)
	RegisterSerializer(reg, key, codec.Marshal)
}

// Serialize looks up the serializer for v's concrete type and produces the
// registered key and the marshaled data.
//
// If v is a pointer to a type registered by value, Serialize dereferences and
// serializes the underlying value.
//
// Returns:
//   - ErrSerializerNotFound if no serializer is registered for the type
//   - ErrKeyTypeMismatch if the requested KEY type doesn't match the registered key type
//   - ErrDataTypeMismatch if the requested DATA type doesn't match what the marshal function produces
func Serialize[KEY comparable, DATA any](reg serializerResolver, v any) (KEY, DATA, error) {
	var zeroK KEY
	var zeroD DATA

	typ := reflect.TypeOf(v)
	if typ == nil {
		return zeroK, zeroD, fmt.Errorf("typemux: %w for nil value", ErrSerializerNotFound)
	}

	entry, ok := reg.getSerializer(typ)
	if !ok {
		if typ.Kind() == reflect.Ptr {
			if e, ok2 := reg.getSerializer(typ.Elem()); ok2 {
				entry = e
				v = reflect.ValueOf(v).Elem().Interface()
				ok = true
			}
		}
		if !ok {
			return zeroK, zeroD, fmt.Errorf("typemux: %w for type %v", ErrSerializerNotFound, typ)
		}
	}

	k, ok := entry.key.(KEY)
	if !ok {
		return zeroK, zeroD, fmt.Errorf("typemux: %w: registered key is %T, requested %T", ErrKeyTypeMismatch, entry.key, zeroK)
	}

	result, err := entry.fn(v)
	if err != nil {
		return zeroK, zeroD, err
	}

	data, ok := result.(DATA)
	if !ok {
		return zeroK, zeroD, fmt.Errorf("typemux: %w: registered data is %T, requested %T", ErrDataTypeMismatch, result, zeroD)
	}

	return k, data, nil
}
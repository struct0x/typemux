package typemux

import (
	"errors"
	"fmt"
	"reflect"
)

// ErrSerializerNotFound is returned when no serializer is registered for the value's
// type (regardless of DATA type).
var ErrSerializerNotFound = errors.New("serializer not found")

// ErrKeyTypeMismatch is returned when Serialize is invoked with a KEY type parameter
// that does not match the key type used at registration time.
var ErrKeyTypeMismatch = errors.New("key type mismatch")

// ErrDataTypeMismatch is returned when serializers are registered for the value's
// type, but none of them produce the DATA type requested by Serialize.
var ErrDataTypeMismatch = errors.New("data type mismatch")

type serializerFuncAny func(v any) (any, error)

type serializerEntry struct {
	key any
	fn  serializerFuncAny
}

type serializerResolver interface {
	getSerializer(typ, dataType reflect.Type) (serializerEntry, bool)
	typeRegistered(typ reflect.Type) bool
}

// Serialize looks up the codec's marshal half for v's concrete type and the
// requested DATA, then produces the registered key and the marshaled data.
//
// DATA must be specified at the call site — a single registry may hold
// codecs producing different DATA types for the same value type, and the
// caller picks which one is expected. This mirrors CreateType[KEY, DATA].
//
//	name, data, err := typemux.Marshal[string, []byte](sealed, value)
//
// If v is a pointer to a type registered by value, Serialize dereferences and
// serializes the underlying value.
//
// Returns:
//   - ErrSerializerNotFound if no codec is registered for the value's type
//   - ErrDataTypeMismatch if the type is registered but not for the requested DATA
//   - ErrKeyTypeMismatch if the requested KEY type doesn't match the registered key
//   - ErrUnsupported if the codec's marshal half was Unsupported
func Serialize[KEY comparable, DATA any](reg serializerResolver, v any) (KEY, DATA, error) {
	var zeroK KEY
	var zeroD DATA

	typ := reflect.TypeOf(v)
	if typ == nil {
		return zeroK, zeroD, fmt.Errorf("typemux: %w for nil value", ErrSerializerNotFound)
	}

	dataType := reflect.TypeOf((*DATA)(nil)).Elem()

	entry, ok := reg.getSerializer(typ, dataType)
	if !ok && typ.Kind() == reflect.Ptr {
		if e, ok2 := reg.getSerializer(typ.Elem(), dataType); ok2 {
			entry = e
			v = reflect.ValueOf(v).Elem().Interface()
			ok = true
		}
	}

	if !ok {
		// Differentiate "type unknown" from "type known but DATA mismatch".
		probe := typ
		if typ.Kind() == reflect.Ptr && reg.typeRegistered(typ.Elem()) {
			probe = typ.Elem()
		}
		if reg.typeRegistered(probe) {
			return zeroK, zeroD, fmt.Errorf("typemux: %w: type %v has no serializer producing %v", ErrDataTypeMismatch, probe, dataType)
		}
		return zeroK, zeroD, fmt.Errorf("typemux: %w for type %v", ErrSerializerNotFound, typ)
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

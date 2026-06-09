package typemux

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"sync"
)

// ErrUnsupported is returned by Unsupported when invoked. Plug Unsupported
// into a Codec half to declare that direction as not supported — Serialize or
// CreateType against that half will then surface ErrUnsupported.
var ErrUnsupported = errors.New("codec direction not supported")

// Codec couples a marshal/unmarshal pair for type T over wire format DATA.
// DATA is typically []byte for transport, but can be any type — e.g.
// map[string]any when bridging to an adapter library that traffics in maps.
//
// Either half may be Unsupported (the sentinel function below) to declare a
// one-directional codec.
type Codec[DATA, T any] struct {
	Marshal   func(T) (DATA, error)
	Unmarshal func(DATA) (T, error)
}

// NewCodec constructs a Codec from a marshal/unmarshal pair. Go infers DATA
// and T from the function signatures, so call sites don't repeat them.
//
// For one-directional codecs, pass Unsupported into the unwanted half:
//
//	read := typemux.NewCodec(typemux.Unsupported[UserCreated, []byte], unmarshalUser)
//	write := typemux.NewCodec(marshalUser, typemux.Unsupported[[]byte, UserCreated])
func NewCodec[DATA, T any](
	marshal func(T) (DATA, error),
	unmarshal func(DATA) (T, error),
) Codec[DATA, T] {
	return Codec[DATA, T]{Marshal: marshal, Unmarshal: unmarshal}
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

// Unsupported is a placeholder for one half of a Codec. It returns the zero
// value of Y and ErrUnsupported when invoked.
//
// Pass it (uninstantiated, as a function value) to NewCodec or directly into
// Codec.Marshal / Codec.Unmarshal:
//
//	codec := typemux.NewCodec(typemux.Unsupported[UserCreated, []byte], realUnmarshal)
//
// The type parameters match the signature of the half it replaces:
//   - For Codec.Marshal (func(T) (DATA, error)): Unsupported[T, DATA]
//   - For Codec.Unmarshal (func(DATA) (T, error)): Unsupported[DATA, T]
func Unsupported[X, Y any](X) (Y, error) {
	var zero Y
	return zero, ErrUnsupported
}

// CodecRegistry holds registered codecs. Both halves are heterogeneous over
// DATA — a single CodecRegistry can hold codecs producing/consuming
// different DATA types simultaneously. The DATA type is chosen at each
// Serialize / CreateType call site.
//
// The factory side is keyed by (key, DATA-type); the serializer side by
// (T, DATA-type). Use NewCodecRegistry() to create one, then RegisterCodec().
type CodecRegistry struct {
	mu          sync.RWMutex
	factories   map[any]map[reflect.Type]factoryFuncAny
	serializers map[reflect.Type]map[reflect.Type]serializerEntry
}

// NewCodecRegistry creates a new empty CodecRegistry.
func NewCodecRegistry() *CodecRegistry {
	return &CodecRegistry{
		factories:   make(map[any]map[reflect.Type]factoryFuncAny),
		serializers: make(map[reflect.Type]map[reflect.Type]serializerEntry),
	}
}

func (r *CodecRegistry) registerFactory(key any, dataType reflect.Type, factory factoryFuncAny) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.factories == nil {
		r.factories = make(map[any]map[reflect.Type]factoryFuncAny)
	}

	inner, ok := r.factories[key]
	if !ok {
		inner = make(map[reflect.Type]factoryFuncAny)
		r.factories[key] = inner
	}
	inner[dataType] = factory
}

func (r *CodecRegistry) registerSerializer(typ, dataType reflect.Type, entry serializerEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.serializers == nil {
		r.serializers = make(map[reflect.Type]map[reflect.Type]serializerEntry)
	}

	inner, ok := r.serializers[typ]
	if !ok {
		inner = make(map[reflect.Type]serializerEntry)
		r.serializers[typ] = inner
	}
	inner[dataType] = entry
}

func (r *CodecRegistry) getFactory(key any, dataType reflect.Type) (factoryFuncAny, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inner, ok := r.factories[key]
	if !ok {
		return nil, false
	}
	f, ok := inner[dataType]
	return f, ok
}

func (r *CodecRegistry) keyRegistered(key any) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[key]
	return ok
}

func (r *CodecRegistry) getSerializer(typ, dataType reflect.Type) (serializerEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inner, ok := r.serializers[typ]
	if !ok {
		return serializerEntry{}, false
	}
	e, ok := inner[dataType]
	return e, ok
}

func (r *CodecRegistry) typeRegistered(typ reflect.Type) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.serializers[typ]
	return ok
}

// Seal finalizes the CodecRegistry and returns a SealedCodecRegistry.
func (r *CodecRegistry) Seal() *SealedCodecRegistry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factories := make(map[any]map[reflect.Type]factoryFuncAny, len(r.factories))
	for k, inner := range r.factories {
		factories[k] = maps.Clone(inner)
	}
	serializers := make(map[reflect.Type]map[reflect.Type]serializerEntry, len(r.serializers))
	for t, inner := range r.serializers {
		serializers[t] = maps.Clone(inner)
	}
	return &SealedCodecRegistry{factories: factories, serializers: serializers}
}

// SealedCodecRegistry is an immutable codec resolver.
type SealedCodecRegistry struct {
	factories   map[any]map[reflect.Type]factoryFuncAny
	serializers map[reflect.Type]map[reflect.Type]serializerEntry
}

func (s *SealedCodecRegistry) getFactory(key any, dataType reflect.Type) (factoryFuncAny, bool) {
	inner, ok := s.factories[key]
	if !ok {
		return nil, false
	}
	f, ok := inner[dataType]
	return f, ok
}

func (s *SealedCodecRegistry) keyRegistered(key any) bool {
	_, ok := s.factories[key]
	return ok
}

func (s *SealedCodecRegistry) getSerializer(typ, dataType reflect.Type) (serializerEntry, bool) {
	inner, ok := s.serializers[typ]
	if !ok {
		return serializerEntry{}, false
	}
	e, ok := inner[dataType]
	return e, ok
}

func (s *SealedCodecRegistry) typeRegistered(typ reflect.Type) bool {
	_, ok := s.serializers[typ]
	return ok
}

// codecRegistrar is the interface satisfied by *CodecRegistry and *Registry,
// letting RegisterCodec write into either.
type codecRegistrar interface {
	registerFactory(key any, dataType reflect.Type, factory factoryFuncAny)
	registerSerializer(typ, dataType reflect.Type, entry serializerEntry)
}

// RegisterCodec registers both halves of a codec for type T over wire format
// DATA. The unmarshal half powers CreateType[KEY, DATA]; the marshal half
// powers Serialize[KEY, DATA].
//
// Both halves are keyed by their (key/T, DATA-type) pair, so the same key+T
// can host multiple codecs over different wire formats simultaneously.
// Registering with the same (key, DATA) or (T, DATA) replaces the prior one.
//
// The key can be any comparable type (string, int, custom enum, etc.).
func RegisterCodec[KEY comparable, DATA, T any](reg codecRegistrar, key KEY, codec Codec[DATA, T]) {
	dataType := reflect.TypeOf((*DATA)(nil)).Elem()
	typ := reflect.TypeOf((*T)(nil)).Elem()

	unmarshal := codec.Unmarshal
	reg.registerFactory(key, dataType, func(data any) (any, error) {
		d, ok := data.(DATA)
		if !ok {
			// Unreachable via CreateType — lookup enforces the DATA match.
			var zero DATA
			return nil, fmt.Errorf("typemux: %w: %T, got %T", ErrDataTypeNotSupported, zero, data)
		}
		return unmarshal(d)
	})

	marshal := codec.Marshal
	reg.registerSerializer(typ, dataType, serializerEntry{
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

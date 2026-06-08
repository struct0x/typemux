package typemux

import (
	"maps"
	"reflect"
	"sync"
)

// SerializerRegistry holds registered marshal functions keyed by reflect.Type.
// Use NewSerializerRegistry() to create one, then RegisterCodec().
type SerializerRegistry struct {
	mu          sync.RWMutex
	serializers map[reflect.Type]serializerEntry
}

// NewSerializerRegistry creates a new empty SerializerRegistry.
func NewSerializerRegistry() *SerializerRegistry {
	return &SerializerRegistry{
		serializers: make(map[reflect.Type]serializerEntry),
	}
}

func (r *SerializerRegistry) registerSerializer(typ reflect.Type, entry serializerEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.serializers == nil {
		r.serializers = make(map[reflect.Type]serializerEntry)
	}

	r.serializers[typ] = entry
}

func (r *SerializerRegistry) getSerializer(typ reflect.Type) (serializerEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.serializers[typ]
	return e, ok
}

// Seal finalizes the SerializerRegistry and returns a SealedSerializerRegistry.
func (r *SerializerRegistry) Seal() *SealedSerializerRegistry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return &SealedSerializerRegistry{serializers: maps.Clone(r.serializers)}
}

// SealedSerializerRegistry is an immutable serializer resolver.
type SealedSerializerRegistry struct {
	serializers map[reflect.Type]serializerEntry
}

func (s *SealedSerializerRegistry) getSerializer(typ reflect.Type) (serializerEntry, bool) {
	e, ok := s.serializers[typ]
	return e, ok
}

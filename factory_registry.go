package typemux

import (
	"maps"
	"sync"
)

// FactoryRegistry holds registered type factories.
// Use NewFactoryRegistry() to create one, then RegisterFactory().
type FactoryRegistry struct {
	mu        sync.RWMutex
	factories map[any]factoryFuncAny
}

// NewFactoryRegistry creates a new empty FactoryRegistry.
func NewFactoryRegistry() *FactoryRegistry {
	return &FactoryRegistry{
		factories: make(map[any]factoryFuncAny),
	}
}

func (r *FactoryRegistry) registerFactory(key any, factory factoryFuncAny) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.factories == nil {
		r.factories = make(map[any]factoryFuncAny)
	}

	r.factories[key] = factory
}

func (r *FactoryRegistry) getFactory(key any) (factoryFuncAny, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	f, ok := r.factories[key]
	return f, ok
}

// Seal finalizes the FactoryRegistry and returns a SealedFactoryRegistry.
func (r *FactoryRegistry) Seal() *SealedFactoryRegistry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return &SealedFactoryRegistry{factories: maps.Clone(r.factories)}
}

// SealedFactoryRegistry is an immutable factory resolver.
type SealedFactoryRegistry struct {
	factories map[any]factoryFuncAny
}

func (s *SealedFactoryRegistry) getFactory(key any) (factoryFuncAny, bool) {
	f, ok := s.factories[key]
	return f, ok
}

package typemux

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"sync"
)

// ErrHandlerNotFound is returned when no handler is found for the given value's type.
var ErrHandlerNotFound = errors.New("handler not found")

// DispatchRegistry holds registered type-safe handlers.
// Use NewDispatchRegistry() to create one, then RegisterDispatch() handlers.
type DispatchRegistry struct {
	mu sync.RWMutex
	h  map[reflect.Type]handlerFuncAny
}

// NewDispatchRegistry creates a new empty DispatchRegistry.
//
// DispatchRegistry holds registered type-safe handlers.
func NewDispatchRegistry() *DispatchRegistry {
	return &DispatchRegistry{
		h: make(map[reflect.Type]handlerFuncAny),
	}
}

func (r *DispatchRegistry) call(typ reflect.Type, ctx context.Context, v any) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return call(typ, ctx, v, r.h)
}

func (r *DispatchRegistry) registerDispatch(typ reflect.Type, funcAny handlerFuncAny) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.h == nil {
		r.h = make(map[reflect.Type]handlerFuncAny)
	}

	r.h[typ] = funcAny
}

// Seal finalizes the DispatchRegistry and returns a SealedDispatchRegistry.
func (r *DispatchRegistry) Seal() *SealedDispatchRegistry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return &SealedDispatchRegistry{h: maps.Clone(r.h)}
}

// SealedDispatchRegistry is an immutable, thread-safe dispatcher.
type SealedDispatchRegistry struct {
	h map[reflect.Type]handlerFuncAny
}

func (s *SealedDispatchRegistry) call(typ reflect.Type, ctx context.Context, v any) error {
	return call(typ, ctx, v, s.h)
}

// Registry is a composite registry that combines dispatch handlers and codecs.
//
// The codec half is heterogeneous over DATA — a single Registry can hold
// codecs producing/consuming different DATA types simultaneously. The DATA
// type is chosen at each Serialize / CreateType call site.
//
// Use NewRegistry() to create one; register codecs with RegisterCodec and
// handlers with RegisterDispatch.
type Registry struct {
	*DispatchRegistry
	*CodecRegistry
}

// NewRegistry creates a new composite Registry with handler + codec support.
func NewRegistry() *Registry {
	return &Registry{
		DispatchRegistry: NewDispatchRegistry(),
		CodecRegistry:    NewCodecRegistry(),
	}
}

// Seal finalizes the Registry and returns a SealedRegistry.
//
// The resulting SealedRegistry is immutable and safe for concurrent use
// with no mutex overhead.
func (r *Registry) Seal() *SealedRegistry {
	return &SealedRegistry{
		SealedDispatchRegistry: r.DispatchRegistry.Seal(),
		SealedCodecRegistry:    r.CodecRegistry.Seal(),
	}
}

// SealedRegistry is an immutable composite registry for runtime use.
type SealedRegistry struct {
	*SealedDispatchRegistry
	*SealedCodecRegistry
}

func call(typ reflect.Type, ctx context.Context, v any, h map[reflect.Type]handlerFuncAny) error {
	if handler, ok := h[typ]; ok {
		return handler(ctx, v)
	}

	// Fallback: if v is a pointer, try the element type
	if typ.Kind() == reflect.Ptr {
		if handler, ok := h[typ.Elem()]; ok {
			return handler(ctx, reflect.ValueOf(v).Elem().Interface())
		}
	}

	return fmt.Errorf("typemux: %w for type %v", ErrHandlerNotFound, typ)
}

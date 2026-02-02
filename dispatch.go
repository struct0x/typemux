package typemux

import (
	"context"
	"fmt"
	"reflect"
)

// HandlerFunc is a type-safe handler for values of type T.
// It receives a context and a value, and may return an error.
type HandlerFunc[T any] func(ctx context.Context, val T) error

// Middleware is a type-safe wrapper around a HandlerFunc.
// It allows injecting logic before/after the handler.
type Middleware[T any] func(next HandlerFunc[T]) HandlerFunc[T]

type dispatchRegistry interface {
	registerDispatch(reflect.Type, handlerFuncAny)
}

type dispatcher interface {
	call(p reflect.Type, ctx context.Context, v any) error
}

type handlerFuncAny func(ctx context.Context, val any) error

// DispatchMiddleware wraps a dispatch call with access to the event as any.
// Use for cross-cutting concerns like logging, timing, and tracing that don't
// need type-specific access to the event.
type DispatchMiddleware func(ctx context.Context, event any, next func(context.Context) error) error

// Dispatch dispatches the given value to a registered handler based on its concrete type.
// Optional generic middleware is applied outermost-first, wrapping the typed middleware chain.
// It returns ErrHandlerNotFound if no handler is registered for the value's type.
func Dispatch(disp dispatcher, ctx context.Context, v any, middleware ...DispatchMiddleware) error {
	typ := reflect.TypeOf(v)

	if len(middleware) == 0 {
		return disp.call(typ, ctx, v)
	}

	call := func(ctx context.Context) error {
		return disp.call(typ, ctx, v)
	}

	for i := len(middleware) - 1; i >= 0; i-- {
		mw := middleware[i]
		next := call
		call = func(ctx context.Context) error {
			return mw(ctx, v, next)
		}
	}

	return call(ctx)
}

// RegisterDispatch adds a handler for values of type T, with optional middleware.
//
// If a handler for the same type T has already been registered, it will be
// replaced by the new handler and middleware chain.
//
// Middleware is applied outermost first (i.e., the last middleware wraps the others).
func RegisterDispatch[T any](reg dispatchRegistry, handler HandlerFunc[T], middleware ...Middleware[T]) {
	typ := reflect.TypeOf((*T)(nil)).Elem()
	finalTyped := applyMiddleware(handler, middleware...)

	reg.registerDispatch(typ, wrapTypedHandler(finalTyped))
}

func applyMiddleware[T any](base HandlerFunc[T], middleware ...Middleware[T]) HandlerFunc[T] {
	final := base
	for i := len(middleware) - 1; i >= 0; i-- {
		final = middleware[i](final)
	}
	return final
}

func wrapTypedHandler[T any](h HandlerFunc[T]) handlerFuncAny {
	var zero T
	return func(ctx context.Context, v any) error {
		val, ok := v.(T)
		if !ok {
			return fmt.Errorf("typemux: expected %T, got %T", zero, v)
		}
		return h(ctx, val)
	}
}

// MiddlewareFunc is a simple convenience function to create middleware
// from a function that optionally short-circuits the call chain.
func MiddlewareFunc[T any](f func(context.Context, T) (cont bool, err error)) Middleware[T] {
	return func(next HandlerFunc[T]) HandlerFunc[T] {
		return func(ctx context.Context, val T) error {
			cont, err := f(ctx, val)
			if err != nil {
				return err
			}
			if !cont {
				return nil
			}
			return next(ctx, val)
		}
	}
}

# Typemux

[![Go Reference](https://pkg.go.dev/badge/github.com/struct0x/typemux.svg)](https://pkg.go.dev/github.com/struct0x/typemux)
[![Go Report Card](https://goreportcard.com/badge/github.com/struct0x/typemux)](https://goreportcard.com/report/github.com/struct0x/typemux)
![Coverage](https://img.shields.io/badge/Coverage-58.9%25-yellow)

## Overview

Typemux is a Go library that provides a fast and efficient way to route values to their appropriate handlers based on type.
It's designed with performance in mind, offering both thread-safe and immutable variants with different performance characteristics.

## Features

- **Type Safety**: Compile-time type safety with generic handlers
- **Zero Allocations**: Dispatch operations produce zero allocations in steady state
- **High Performance**: Optimized for concurrent access patterns
- **Middleware Support**: Both typed and generic middleware for cross-cutting concerns
- **Factory System**: Create typed values from raw data (JSON, etc.) using registered factories
- **Multiple Registry Types**:
    - `Registry`: Thread-safe composite registry (dispatch + factory)
    - `SealedRegistry`: Immutable composite with zero mutex overhead
    - `DispatchRegistry` / `FactoryRegistry`: Specialized single-purpose registries

## Installation

```bash
go get github.com/struct0x/typemux
```

## Usage

See the [example/](example/) directory for a complete HTTP server demonstrating the factory + dispatch pattern.

### Basic Example

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/struct0x/typemux"
)

type UserCreated struct {
	ID   int
	Name string
}

type OrderPlaced struct {
	OrderID string
	Amount  float64
}

func main() {
	// Create a new registry
	reg := typemux.NewRegistry()

	// RegisterDispatch handlers for different types
	typemux.RegisterDispatch[UserCreated](reg, func(ctx context.Context, event UserCreated) error {
		fmt.Printf("User created: %s (ID: %d)\n", event.Name, event.ID)
		return nil
	})

	typemux.RegisterDispatch[OrderPlaced](reg, func(ctx context.Context, event OrderPlaced) error {
		fmt.Printf("Order placed: %s for $%.2f\n", event.OrderID, event.Amount)
		return nil
	})

	// Dispatch events
	ctx := context.Background()

	if err := typemux.Dispatch(reg, ctx, UserCreated{ID: 1, Name: "Alice"}); err != nil {
		log.Fatal(err)
	}

	if err := typemux.Dispatch(reg, ctx, OrderPlaced{OrderID: "ORD-001", Amount: 99.99}); err != nil {
		log.Fatal(err)
	}
}
```

### Typed Middleware (Applied at Registration)

```go
package main

import (
	"context"
	"fmt"

	"github.com/struct0x/typemux"
)

func main() {
	reg := typemux.NewRegistry()

	// Define typed middleware - has access to the concrete event type
	loggingMiddleware := func(next typemux.HandlerFunc[UserCreated]) typemux.HandlerFunc[UserCreated] {
		return func(ctx context.Context, event UserCreated) error {
			fmt.Printf("Processing user: %s\n", event.Name)
			err := next(ctx, event)
			fmt.Printf("Finished processing user: %s\n", event.Name)
			return err
		}
	}

	// Register handler with typed middleware
	typemux.RegisterDispatch(reg, handler, loggingMiddleware)
}
```

### Generic Middleware (Applied at Dispatch)

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/struct0x/typemux"
)

func main() {
	reg := typemux.NewRegistry()
	// ... register handlers ...

	// Define generic middleware - works across all event types
	timingMiddleware := func(ctx context.Context, event any, next func(context.Context) error) error {
		start := time.Now()
		err := next(ctx)
		fmt.Printf("Dispatch took %v\n", time.Since(start))
		return err
	}

	loggingMiddleware := func(ctx context.Context, event any, next func(context.Context) error) error {
		fmt.Printf("Dispatching event: %T\n", event)
		return next(ctx)
	}

	// Apply generic middleware at dispatch time
	typemux.Dispatch(reg, ctx, event, loggingMiddleware, timingMiddleware)
}
```

### Sealed Registry for Maximum Performance

Concurrent access to map is fine as long as it's read-only. 
That way we can avoid mutex overhead when the registry is constructed at runtime.

Use `Seal()` method to get immutable registry. 

```go
package main

import (
	"log"

	"github.com/struct0x/typemux"
)

func main() {
	// ...
	// After registering all handlers, seal the registry for better performance
	sealedReg := reg.Seal()

	// SealedRegistry has zero mutex overhead
	if err := typemux.Dispatch(sealedReg, ctx, UserCreated{ID: 2, Name: "Bob"}); err != nil {
		log.Fatal(err)
	}
}
```

### Factory System

Create typed values from raw data using registered factories. Useful for deserializing
messages from wire formats and then dispatching them.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/struct0x/typemux"
)

type UserCreated struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	reg := typemux.NewRegistry()

	// Register a handler for UserCreated
	typemux.RegisterDispatch(reg, func(ctx context.Context, event UserCreated) error {
		fmt.Printf("User created: %s\n", event.Name)
		return nil
	})

	// Register a JSON factory for creating UserCreated from bytes
	typemux.RegisterFactory(reg, "user_created", typemux.JSONFactory[UserCreated]())

	// Later: create typed value from JSON and dispatch
	jsonData := []byte(`{"id": 1, "name": "Alice"}`)
	value, err := typemux.CreateType(reg, "user_created", jsonData)
	if err != nil {
		log.Fatal(err)
	}

	if err := typemux.Dispatch(reg, context.Background(), value); err != nil {
		log.Fatal(err)
	}
}
```

#### Custom Factories

```go

// Register a custom factory with any data type
typemux.RegisterFactory(reg, "user_from_map", func(data map[string]any) (UserCreated, error) {
	return UserCreated{
		ID:   data["id"].(int),
		Name: data["name"].(string),
	}, nil
})

// Keys can be any comparable type
type EventType int
const UserCreatedEvent EventType = 1

typemux.RegisterFactory(reg, UserCreatedEvent, typemux.JSONFactory[UserCreated]())
```

## Performance

Typemux is optimized for high-performance scenarios:

- **Zero Allocations**: Dispatch operations after initialization produce zero allocations
- **Concurrent Safe**: `Registry` can be used safely across goroutines
- **Sealed Optimization**: `SealedRegistry` eliminates all mutex overhead
- **Benchmark Results** (Apple M2 Max):
    - `Registry`:
        - 1 CPU: 21.46 ns/op
        - 4 CPU: 64.80 ns/op
        - 8 CPU: 122.0 ns/op
    - `SealedRegistry`:
        - 1 CPU: 14.59 ns/op
        - 4 CPU: 28.95 ns/op
        - 8 CPU: 45.53 ns/op

*Note: Performance may vary based on your system and workload. Run benchmarks on your target system for
accurate measurements.*

Run benchmarks with:

```bash
go test -bench=. -benchmem
```

## How It Works

1. **Registration**: Handlers and factories are registered using Go generics for type safety
2. **Type Mapping**: Types are mapped to handlers using `reflect.Type` as keys
3. **Dispatch**: Values are routed to appropriate handlers based on their runtime type
4. **Factory Creation**: Raw data (JSON, etc.) is converted to typed values using registered factories
5. **Middleware Chains**:
   - Typed middleware is applied at registration.
   - Generic middleware is applied at dispatch time.
6. **Sealing**: Registries can be sealed for immutable, zero-mutex runtime use

## API Reference

### Registry Types

| Type | Description |
|------|-------------|
| `Registry` | Thread-safe composite registry (dispatch + factory) |
| `SealedRegistry` | Immutable composite registry with zero mutex overhead |
| `DispatchRegistry` | Thread-safe registry for dispatch handlers only |
| `SealedDispatchRegistry` | Immutable dispatch-only registry |
| `FactoryRegistry` | Thread-safe registry for factories only |
| `SealedFactoryRegistry` | Immutable factory-only registry |

### Handler & Middleware Types

| Type | Signature |
|------|-----------|
| `HandlerFunc[T]` | `func(ctx context.Context, val T) error` |
| `Middleware[T]` | `func(next HandlerFunc[T]) HandlerFunc[T]` |
| `DispatchMiddleware` | `func(ctx context.Context, event any, next func(context.Context) error) error` |

### Functions

**Registry Creation:**
- `NewRegistry()` - Creates a composite registry (dispatch + factory)
- `NewDispatchRegistry()` - Creates a dispatch-only registry
- `NewFactoryRegistry()` - Creates a factory-only registry

**Dispatch:**
- `RegisterDispatch[T](reg, handler, middleware...)` - Registers a handler for type T
    - ⚠️ Later registrations for the same type overwrite earlier ones
- `Dispatch(reg, ctx, value, middleware...)` - Dispatches a value to its handler
- `MiddlewareFunc[T](f)` - Creates middleware from a simple validation function

**Factory:**
- `RegisterFactory[KEY, DATA, T](reg, key, factory)` - Registers a factory for a key
- `CreateType[KEY, DATA](reg, key, data)` - Creates a typed value using a registered factory
- `JSONFactory[T]()` - Returns a factory that unmarshals JSON into type T

**Sealing:**
- `registry.Seal()` - Returns an immutable sealed copy of the registry

### Error Types

| Error | Description |
|-------|-------------|
| `ErrHandlerNotFound` | No handler registered for the dispatched value's type |
| `ErrFactoryNotFound` | No factory registered for the given key |
| `ErrDataTypeNotSupported` | Data type doesn't match the factory's expected input type |

### Pointer/Value Dispatch

When dispatching a pointer, if no handler is registered for the pointer type,
typemux automatically falls back to the element type's handler:

```go
typemux.RegisterDispatch(reg, func(ctx context.Context, e UserCreated) error {
	return nil
})

// Both work:
typemux.Dispatch(reg, ctx, UserCreated{})   // Direct match
typemux.Dispatch(reg, ctx, &UserCreated{})  // Falls back to value handler
```

## Use Cases

- **Event-driven architectures**
- **Message routing systems**
- **Command handlers in CQRS**
- **Plugin systems**
- **Any scenario requiring type-based dispatch**

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

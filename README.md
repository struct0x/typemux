# Typemux

[![Go Reference](https://pkg.go.dev/badge/github.com/struct0x/typemux.svg)](https://pkg.go.dev/github.com/struct0x/typemux)
[![Go Report Card](https://goreportcard.com/badge/github.com/struct0x/typemux)](https://goreportcard.com/report/github.com/struct0x/typemux)
![Coverage](https://img.shields.io/badge/Coverage-93.8%25-brightgreen)

## Overview

Typemux is a Go library that provides a fast and efficient way to route values to their appropriate handlers based on type.
It's designed with performance in mind, offering both thread-safe and immutable variants with different performance characteristics.

## Features

- **Type Safety**: Compile-time type safety with generic handlers
- **Zero Allocations**: Dispatch operations produce zero allocations in steady state
- **High Performance**: Optimized for concurrent access patterns
- **Middleware Support**: Both typed and generic middleware for cross-cutting concerns
- **Factory System**: Create typed values from raw data (JSON, etc.) using registered factories
- **Serialization**: Convert typed values back into a wire-format `(name, []byte)` pair via registered codecs
- **Multiple Registry Types**:
    - `Registry`: Thread-safe composite registry (dispatch + factory + serializer)
    - `SealedRegistry`: Immutable composite with zero mutex overhead
    - `DispatchRegistry` / `CodecRegistry`: Specialized single-purpose registries

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

### Codecs (Read / Write / Round-Trip)

All encoding/decoding goes through `RegisterCodec`. A `Codec[DATA, T]` is a
marshal + unmarshal pair; register one and you get the read side (`CreateType`)
and the write side (`Serialize`) wired up at once.

`DATA` is chosen at each call site, so a single `Registry` can hold codecs
producing/consuming different `DATA` types simultaneously — `[]byte` for one
event, `map[string]any` for another:

```go
package main

import (
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

	// One call registers both the factory and the serializer
	typemux.RegisterCodec(reg, "user_created", typemux.JSONCodec[UserCreated]())

	sealed := reg.Seal()

	// Read side: bytes -> typed value (via CreateType)
	jsonData := []byte(`{"id": 1, "name": "Alice"}`)
	value, err := typemux.CreateType(sealed, "user_created", jsonData)
	if err != nil {
		log.Fatal(err)
	}

	// Write side: typed value -> (name, bytes)
	name, data, err := typemux.Serialize[string, []byte](sealed, value.(UserCreated))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s -> %s\n", name, data)
}
```

`Marshal`'s `KEY` type parameter matches the type used at registration. With
a custom comparable key:

```go
type EventKind int
const UserCreatedEvent EventKind = 1

typemux.RegisterCodec(reg, UserCreatedEvent, typemux.JSONCodec[UserCreated]())
name, data, _ := typemux.Serialize[EventKind, []byte](reg, UserCreated{ID: 1, Name: "Alice"})
// name == UserCreatedEvent
```

#### Non-byte wire formats

For adapter boundaries that traffic in `map[string]any` (or any other type),
use `NewCodec` so Go can infer `DATA` and `T` from the function signatures.
The same registry can host both byte and map codecs under the same key:

```go
typemux.RegisterCodec(reg, "user_created", typemux.NewCodec(
    func(u UserCreated) (map[string]any, error) {
        return map[string]any{"id": u.ID, "name": u.Name}, nil
    },
    func(m map[string]any) (UserCreated, error) {
        return UserCreated{ID: m["id"].(string), Name: m["name"].(string)}, nil
    },
))
name, m, _ := typemux.Serialize[string, map[string]any](reg.Seal(), UserCreated{ID: 1})
// m has type map[string]any
```

#### One-directional codecs

If you only need read (or only write) for a given codec, plug `Unsupported`
into the other half. It returns `ErrUnsupported` when invoked:

```go
// Read-only: Marshal[...] returns ErrUnsupported, CreateType works.
typemux.RegisterCodec(reg, "user_created", typemux.NewCodec(
    typemux.Unsupported[UserCreated, []byte],
    func(data []byte) (UserCreated, error) { /* ... */ },
))

// Write-only: CreateType returns ErrUnsupported, Marshal[...] works.
typemux.RegisterCodec(reg, "user_created", typemux.NewCodec(
    func(u UserCreated) ([]byte, error) { /* ... */ },
    typemux.Unsupported[[]byte, UserCreated],
))
```

If the requested `KEY` type doesn't match the registered key, `Marshal`
returns `ErrKeyTypeMismatch`. If no codec is registered for the value's type,
it returns `ErrSerializerNotFound`. Pointer-to-value lookup falls back to the
value's type, matching the dispatcher's behavior.

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
| `Registry` | Thread-safe composite registry (dispatch + codec); heterogeneous over `DATA` |
| `SealedRegistry` | Immutable composite registry with zero mutex overhead |
| `DispatchRegistry` | Thread-safe registry for dispatch handlers only |
| `SealedDispatchRegistry` | Immutable dispatch-only registry |
| `CodecRegistry` | Thread-safe registry for codecs only (heterogeneous `DATA` per codec) |
| `SealedCodecRegistry` | Immutable codec-only registry |

### Handler & Middleware Types

| Type | Signature |
|------|-----------|
| `HandlerFunc[T]` | `func(ctx context.Context, val T) error` |
| `Middleware[T]` | `func(next HandlerFunc[T]) HandlerFunc[T]` |
| `DispatchMiddleware` | `func(ctx context.Context, event any, next func(context.Context) error) error` |

### Functions

**Registry Creation:**
- `NewRegistry()` - Creates a composite registry (dispatch + codec)
- `NewDispatchRegistry()` - Creates a dispatch-only registry
- `NewCodecRegistry()` - Creates a codec-only registry

**Dispatch:**
- `RegisterDispatch[T](reg, handler, middleware...)` - Registers a handler for type T
    - ⚠️ Later registrations for the same type overwrite earlier ones
- `Dispatch(reg, ctx, value, middleware...)` - Dispatches a value to its handler
- `MiddlewareFunc[T](f)` - Creates middleware from a simple validation function

**Codecs (read + write):**
- `RegisterCodec[KEY, DATA, T](reg, key, codec)` - Registers a codec (factory + serializer in one call)
- `CreateType[KEY, DATA](reg, key, data)` - Creates a typed value via the codec's unmarshal half
- `Marshal[KEY, DATA](reg, value)` - Produces `(key, data)` via the codec's marshal half
- `Codec[DATA, T]` - A marshal/unmarshal pair for type T over wire format DATA
- `NewCodec(marshal, unmarshal)` - Constructor with type-parameter inference
- `JSONCodec[T]()` - Returns a `Codec[[]byte, T]` backed by `encoding/json`
- `Unsupported[X, Y](X) (Y, error)` - Placeholder for unused codec half; returns `ErrUnsupported`

**Sealing:**
- `registry.Seal()` - Returns an immutable sealed copy of the registry

### Error Types

| Error                     | Description                                                                             |
|---------------------------|-----------------------------------------------------------------------------------------|
| `ErrHandlerNotFound`      | No handler registered for the dispatched value's type                                   |
| `ErrFactoryNotFound`      | No codec registered under the given key                                                 |
| `ErrDataTypeNotSupported` | Key has codecs but none accepting the requested `DATA` type                             |
| `ErrSerializerNotFound`   | No codec registered for the value's type                                                |
| `ErrDataTypeMismatch`     | Type has codecs but none producing the requested `DATA` type                            |
| `ErrKeyTypeMismatch`      | `Serialize[KEY, DATA]` was called with a KEY type that doesn't match the registered key |
| `ErrUnsupported`          | The codec half being invoked was `Unsupported`                                          |

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

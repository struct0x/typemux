package typemux_test

import (
	"context"
	"fmt"

	"github.com/struct0x/typemux"
)

func loggingMiddleware[T any]() typemux.Middleware[T] {
	return func(next typemux.HandlerFunc[T]) typemux.HandlerFunc[T] {
		return func(ctx context.Context, event T) error {
			fmt.Printf("Processing event: %T\n", event)
			err := next(ctx, event)
			if err != nil {
				fmt.Printf("Error processing event: %v\n", err)
				return err
			}
			fmt.Printf("Successfully processed event: %T\n", event)
			return nil
		}
	}
}

func ExampleDispatch() {
	reg := typemux.NewRegistry()

	// Define event types
	type UserCreated struct {
		ID   int
		Name string
	}

	type OrderPlaced struct {
		OrderID string
		Amount  float64
	}

	type Unknown struct {
		Foo string
	}

	// Create middleware using the helper function
	validationMiddleware := typemux.MiddlewareFunc(func(ctx context.Context, event UserCreated) (bool, error) {
		if event.ID <= 0 {
			return false, fmt.Errorf("invalid user ID: %d", event.ID)
		}
		if event.Name == "" {
			return false, fmt.Errorf("user name cannot be empty")
		}
		return true, nil // Continue processing
	})

	// RegisterDispatch handlers for different types
	typemux.RegisterDispatch(
		reg,
		func(ctx context.Context, event UserCreated) error {
			fmt.Printf("User created: %s (ID: %d)\n", event.Name, event.ID)
			return nil
		},
		validationMiddleware,
		loggingMiddleware[UserCreated](),
	)

	typemux.RegisterDispatch(
		reg,
		func(ctx context.Context, event OrderPlaced) error {
			fmt.Printf("Order placed: %s for $%.2f\n", event.OrderID, event.Amount)
			return nil
		},
		loggingMiddleware[OrderPlaced](),
	)

	// Dispatch events
	ctx := context.Background()

	_ = typemux.Dispatch(reg, ctx, UserCreated{ID: 1, Name: "Alice"})
	_ = typemux.Dispatch(reg, ctx, OrderPlaced{OrderID: "ORD-001", Amount: 99.99})

	sealedReg := reg.Seal()
	_ = typemux.Dispatch(sealedReg, ctx, UserCreated{ID: 2, Name: "Alice"})
	_ = typemux.Dispatch(sealedReg, ctx, OrderPlaced{OrderID: "ORD-002", Amount: 99.99})

	if err := typemux.Dispatch(reg, ctx, Unknown{Foo: "bar"}); err != nil {
		fmt.Printf("Dispatch err: %v", err)
	}

	// Output:
	// Processing event: typemux_test.UserCreated
	// User created: Alice (ID: 1)
	// Successfully processed event: typemux_test.UserCreated
	// Processing event: typemux_test.OrderPlaced
	// Order placed: ORD-001 for $99.99
	// Successfully processed event: typemux_test.OrderPlaced
	// Processing event: typemux_test.UserCreated
	// User created: Alice (ID: 2)
	// Successfully processed event: typemux_test.UserCreated
	// Processing event: typemux_test.OrderPlaced
	// Order placed: ORD-002 for $99.99
	// Successfully processed event: typemux_test.OrderPlaced
	// Dispatch err: typemux: handler not found for type typemux_test.Unknown
}

func ExampleCreateType() {
	// Event types
	type UserCreated struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	type OrderPlaced struct {
		OrderID string `json:"order_id"`
		Amount  int    `json:"amount"`
	}

	// Create registry and register factories + handlers
	reg := typemux.NewRegistry()

	// Register factories using JSONFactory helper
	typemux.RegisterFactory(reg, "user_created", typemux.JSONFactory[UserCreated]())
	typemux.RegisterFactory(reg, "order_placed", typemux.JSONFactory[OrderPlaced]())

	// Register handlers
	typemux.RegisterDispatch(reg, func(ctx context.Context, e UserCreated) error {
		fmt.Printf("User created: %s (ID: %s)\n", e.Name, e.ID)
		return nil
	})

	typemux.RegisterDispatch(reg, func(ctx context.Context, e OrderPlaced) error {
		fmt.Printf("Order placed: %s for $%d\n", e.OrderID, e.Amount)
		return nil
	})

	// Seal for production use
	sealed := reg.Seal()
	ctx := context.Background()

	// Simulate receiving envelopes (e.g., from a message queue)
	envelopes := []struct {
		Type string
		Data []byte
	}{
		{"user_created", []byte(`{"id": "u1", "name": "Alice"}`)},
		{"order_placed", []byte(`{"order_id": "ORD-001", "amount": 150}`)},
		{"user_created", []byte(`{"id": "u2", "name": "Bob"}`)},
	}

	// Process each envelope: create typed value, then dispatch
	for _, env := range envelopes {
		value, err := typemux.CreateType(sealed, env.Type, env.Data)
		if err != nil {
			fmt.Printf("Failed to create type: %v\n", err)
			continue
		}

		if err := typemux.Dispatch(sealed, ctx, value); err != nil {
			fmt.Printf("Failed to dispatch: %v\n", err)
		}
	}

	// Output:
	// User created: Alice (ID: u1)
	// Order placed: ORD-001 for $150
	// User created: Bob (ID: u2)
}

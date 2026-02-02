package typemux_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/struct0x/typemux"
)

func TestRegister_ReplacesExisting(t *testing.T) {
	reg := typemux.NewRegistry()

	var out string

	typemux.RegisterDispatch(reg, func(ctx context.Context, s string) error {
		out = "first"
		return nil
	})

	typemux.RegisterDispatch(reg, func(ctx context.Context, s string) error {
		out = "second"
		return nil
	})

	err := typemux.Dispatch(reg, context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if out != "second" {
		t.Errorf("expected handler to be replaced, got: %s", out)
	}
}

type testEvent struct {
	Name string
}

func TestDispatch_PointerToValueHandler(t *testing.T) {
	reg := typemux.NewRegistry()

	var received string
	typemux.RegisterDispatch(reg, func(ctx context.Context, e testEvent) error {
		received = e.Name
		return nil
	})

	// Dispatch pointer when value handler is registered
	err := typemux.Dispatch(reg, context.Background(), &testEvent{Name: "pointer"})
	if err != nil {
		t.Fatalf("expected pointer dispatch to work with value handler, got: %v", err)
	}
	if received != "pointer" {
		t.Errorf("expected 'pointer', got: %s", received)
	}
}

func TestDispatch_ValueToValueHandler(t *testing.T) {
	reg := typemux.NewRegistry()

	var received string
	typemux.RegisterDispatch(reg, func(ctx context.Context, e testEvent) error {
		received = e.Name
		return nil
	})

	// Dispatch value when value handler is registered (no regression)
	err := typemux.Dispatch(reg, context.Background(), testEvent{Name: "value"})
	if err != nil {
		t.Fatalf("expected value dispatch to work, got: %v", err)
	}
	if received != "value" {
		t.Errorf("expected 'value', got: %s", received)
	}
}

func TestDispatch_ValueToPointerHandler_Fails(t *testing.T) {
	reg := typemux.NewRegistry()

	typemux.RegisterDispatch(reg, func(ctx context.Context, e *testEvent) error {
		return nil
	})

	// Dispatch value when pointer handler is registered - should fail
	err := typemux.Dispatch(reg, context.Background(), testEvent{Name: "value"})
	if err == nil {
		t.Fatal("expected error when dispatching value to pointer handler")
	}
}

func TestDispatch_PointerToValueHandler_Sealed(t *testing.T) {
	reg := typemux.NewRegistry()

	var received string
	typemux.RegisterDispatch(reg, func(ctx context.Context, e testEvent) error {
		received = e.Name
		return nil
	})

	sealed := reg.Seal()

	// Dispatch pointer when value handler is registered
	err := typemux.Dispatch(sealed, context.Background(), &testEvent{Name: "sealed-pointer"})
	if err != nil {
		t.Fatalf("expected pointer dispatch to work with sealed registry, got: %v", err)
	}
	if received != "sealed-pointer" {
		t.Errorf("expected 'sealed-pointer', got: %s", received)
	}
}

func TestDispatch_GenericMiddleware(t *testing.T) {
	reg := typemux.NewRegistry()

	var order []string

	typemux.RegisterDispatch(reg, func(ctx context.Context, e testEvent) error {
		order = append(order, "handler")
		return nil
	})

	// Generic middleware that logs event type
	loggingMiddleware := func(ctx context.Context, event any, next func(context.Context) error) error {
		order = append(order, "before:"+reflect.TypeOf(event).Name())
		err := next(ctx)
		order = append(order, "after")
		return err
	}

	err := typemux.Dispatch(reg, context.Background(), testEvent{Name: "test"}, loggingMiddleware)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"before:testEvent", "handler", "after"}
	if !reflect.DeepEqual(order, expected) {
		t.Errorf("expected %v, got %v", expected, order)
	}
}

func TestDispatch_MultipleGenericMiddleware(t *testing.T) {
	reg := typemux.NewRegistry()

	var order []string

	typemux.RegisterDispatch(reg, func(ctx context.Context, e testEvent) error {
		order = append(order, "handler")
		return nil
	})

	mw1 := func(ctx context.Context, event any, next func(context.Context) error) error {
		order = append(order, "mw1-before")
		err := next(ctx)
		order = append(order, "mw1-after")
		return err
	}

	mw2 := func(ctx context.Context, event any, next func(context.Context) error) error {
		order = append(order, "mw2-before")
		err := next(ctx)
		order = append(order, "mw2-after")
		return err
	}

	// mw1 is outermost, mw2 is inner
	err := typemux.Dispatch(reg, context.Background(), testEvent{Name: "test"}, mw1, mw2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if !reflect.DeepEqual(order, expected) {
		t.Errorf("expected %v, got %v", expected, order)
	}
}

package typemux_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/struct0x/typemux"
)

type UserCreated struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type OrderPlaced struct {
	OrderID string `json:"order_id"`
	Amount  int    `json:"amount"`
}

func TestFactory_Create(t *testing.T) {
	reg := typemux.NewFactoryRegistry()

	typemux.RegisterFactory(reg, "user_created", func(data []byte) (UserCreated, error) {
		var e UserCreated
		return e, json.Unmarshal(data, &e)
	})

	data := []byte(`{"id": "123", "name": "John"}`)

	t.Run("standard_registry", func(t *testing.T) {
		result, err := typemux.CreateType(reg, "user_created", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		user, ok := result.(UserCreated)
		if !ok {
			t.Fatalf("expected UserCreated, got %T", result)
		}

		if user.ID != "123" || user.Name != "John" {
			t.Errorf("unexpected user: %+v", user)
		}
	})

	t.Run("sealed_registry", func(t *testing.T) {
		sealed := reg.Seal()

		result, err := typemux.CreateType(sealed, "user_created", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		user, ok := result.(UserCreated)
		if !ok {
			t.Fatalf("expected UserCreated, got %T", result)
		}

		if user.ID != "123" || user.Name != "John" {
			t.Errorf("unexpected user: %+v", user)
		}
	})
}

func TestFactoryNotFound(t *testing.T) {
	reg := typemux.NewFactoryRegistry()

	_, err := typemux.CreateType(reg, "unknown", []byte{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, typemux.ErrFactoryNotFound) {
		t.Errorf("expected ErrFactoryNotFound, got %v", err)
	}
}

func TestFactoryReplacesExisting(t *testing.T) {
	reg := typemux.NewRegistry()

	typemux.RegisterFactory(reg, "event", func(data []byte) (UserCreated, error) {
		return UserCreated{ID: "first"}, nil
	})

	typemux.RegisterFactory(reg, "event", func(data []byte) (UserCreated, error) {
		return UserCreated{ID: "second"}, nil
	})

	sealed := reg.Seal()

	result, err := typemux.CreateType(sealed, "event", []byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	user := result.(UserCreated)
	if user.ID != "second" {
		t.Errorf("expected second factory to be used, got ID=%s", user.ID)
	}
}

func TestFactoryDifferentKeyTypes(t *testing.T) {
	reg := typemux.NewRegistry()

	// String key
	typemux.RegisterFactory(reg, "string_key", func(data []byte) (UserCreated, error) {
		return UserCreated{ID: "from_string"}, nil
	})

	// Int key
	typemux.RegisterFactory(reg, 42, func(data []byte) (OrderPlaced, error) {
		return OrderPlaced{OrderID: "from_int"}, nil
	})

	sealed := reg.Seal()

	result1, err := typemux.CreateType(sealed, "string_key", []byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result1.(UserCreated).ID != "from_string" {
		t.Errorf("unexpected result: %+v", result1)
	}

	result2, err := typemux.CreateType(sealed, 42, []byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.(OrderPlaced).OrderID != "from_int" {
		t.Errorf("unexpected result: %+v", result2)
	}
}

func TestFactoryWrongDataType(t *testing.T) {
	reg := typemux.NewRegistry()

	typemux.RegisterFactory(reg, "user", func(data []byte) (UserCreated, error) {
		var e UserCreated
		return e, json.Unmarshal(data, &e)
	})

	sealed := reg.Seal()

	// Pass string instead of []byte
	_, err := typemux.CreateType(sealed, "user", "not bytes")
	if err == nil {
		t.Fatal("expected error for wrong data type, got nil")
	}
	if !errors.Is(err, typemux.ErrDataTypeNotSupported) {
		t.Fatalf("expected %q got %q", typemux.ErrDataTypeNotSupported, err)
	}
}

func TestJSONFactory(t *testing.T) {
	reg := typemux.NewRegistry()

	// Use JSONFactory helper instead of manual unmarshaling
	typemux.RegisterFactory(reg, "user_created", typemux.JSONFactory[*UserCreated]())
	typemux.RegisterFactory(reg, "order_placed", typemux.JSONFactory[*OrderPlaced]())

	sealed := reg.Seal()

	// Test UserCreated
	userData := []byte(`{"id": "u1", "name": "Alice"}`)
	result, err := typemux.CreateType(sealed, "user_created", userData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	user := result.(*UserCreated)
	if user.ID != "u1" || user.Name != "Alice" {
		t.Errorf("unexpected user: %+v", user)
	}

	// Test OrderPlaced
	orderData := []byte(`{"order_id": "o1", "amount": 100}`)
	result, err = typemux.CreateType(sealed, "order_placed", orderData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := result.(*OrderPlaced)
	if order.OrderID != "o1" || order.Amount != 100 {
		t.Errorf("unexpected order: %+v", order)
	}
}

func TestJSONFactory_InvalidJSON(t *testing.T) {
	reg := typemux.NewRegistry()
	typemux.RegisterFactory(reg, "user", typemux.JSONFactory[UserCreated]())
	sealed := reg.Seal()

	_, err := typemux.CreateType(sealed, "user", []byte(`{invalid json}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

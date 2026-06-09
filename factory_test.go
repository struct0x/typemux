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

// readOnly[DATA, T] is a helper that builds a Codec with Serialize=Unsupported.
// Most factory tests only exercise the unmarshal/read side.
func readOnly[DATA, T any](unmarshal func(DATA) (T, error)) typemux.Codec[DATA, T] {
	return typemux.NewCodec(typemux.Unsupported[T, DATA], unmarshal)
}

func TestFactory_Create(t *testing.T) {
	reg := typemux.NewRegistry()

	typemux.RegisterCodec(reg, "user_created", readOnly(func(data []byte) (UserCreated, error) {
		var e UserCreated
		return e, json.Unmarshal(data, &e)
	}))

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
	reg := typemux.NewRegistry()

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

	typemux.RegisterCodec(reg, "event", readOnly(func(data []byte) (UserCreated, error) {
		return UserCreated{ID: "first"}, nil
	}))
	typemux.RegisterCodec(reg, "event", readOnly(func(data []byte) (UserCreated, error) {
		return UserCreated{ID: "second"}, nil
	}))

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

	typemux.RegisterCodec(reg, "string_key", readOnly(func(data []byte) (UserCreated, error) {
		return UserCreated{ID: "from_string"}, nil
	}))
	typemux.RegisterCodec(reg, 42, readOnly(func(data []byte) (OrderPlaced, error) {
		return OrderPlaced{OrderID: "from_int"}, nil
	}))

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

	typemux.RegisterCodec(reg, "user", readOnly(func(data []byte) (UserCreated, error) {
		var e UserCreated
		return e, json.Unmarshal(data, &e)
	}))

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

func TestFactoryMultipleDataTypesPerKey(t *testing.T) {
	reg := typemux.NewRegistry()

	typemux.RegisterCodec(reg, "user", readOnly(func(data []byte) (UserCreated, error) {
		var v UserCreated
		return v, json.Unmarshal(data, &v)
	}))
	typemux.RegisterCodec(reg, "user", readOnly(func(data map[string]any) (UserCreated, error) {
		return UserCreated{
			ID:   data["id"].(string),
			Name: data["name"].(string),
		}, nil
	}))

	sealed := reg.Seal()

	bytesResult, err := typemux.CreateType(sealed, "user", []byte(`{"id":"b1","name":"FromBytes"}`))
	if err != nil {
		t.Fatalf("bytes path: %v", err)
	}
	if got := bytesResult.(UserCreated); got.ID != "b1" || got.Name != "FromBytes" {
		t.Errorf("bytes path: %+v", got)
	}

	mapResult, err := typemux.CreateType(sealed, "user", map[string]any{"id": "m1", "name": "FromMap"})
	if err != nil {
		t.Fatalf("map path: %v", err)
	}
	if got := mapResult.(UserCreated); got.ID != "m1" || got.Name != "FromMap" {
		t.Errorf("map path: %+v", got)
	}

	// Unregistered DATA type returns ErrDataTypeNotSupported (key is known).
	_, err = typemux.CreateType(sealed, "user", "raw string")
	if err == nil {
		t.Fatal("expected error for unregistered DATA type")
	}
	if !errors.Is(err, typemux.ErrDataTypeNotSupported) {
		t.Errorf("expected ErrDataTypeNotSupported, got %v", err)
	}

	// Unknown key returns ErrFactoryNotFound.
	_, err = typemux.CreateType(sealed, "unknown", []byte("{}"))
	if !errors.Is(err, typemux.ErrFactoryNotFound) {
		t.Errorf("expected ErrFactoryNotFound, got %v", err)
	}
}

func TestUnsupportedMarshal_CreateTypeUnaffected(t *testing.T) {
	reg := typemux.NewRegistry()
	typemux.RegisterCodec(reg, "user", readOnly(func(data []byte) (UserCreated, error) {
		return UserCreated{ID: "via-unmarshal"}, nil
	}))
	sealed := reg.Seal()

	// Read side still works.
	got, err := typemux.CreateType(sealed, "user", []byte{})
	if err != nil {
		t.Fatalf("CreateType: %v", err)
	}
	if got.(UserCreated).ID != "via-unmarshal" {
		t.Errorf("got %+v", got)
	}

	// Write side returns ErrUnsupported.
	_, _, err = typemux.Serialize[string, []byte](sealed, UserCreated{ID: "u1"})
	if !errors.Is(err, typemux.ErrUnsupported) {
		t.Errorf("expected ErrUnsupported, got %v", err)
	}
}

func TestUnsupportedUnmarshal_MarshalUnaffected(t *testing.T) {
	reg := typemux.NewRegistry()
	typemux.RegisterCodec(reg, "user", typemux.NewCodec(
		func(u UserCreated) ([]byte, error) { return []byte(u.ID), nil },
		typemux.Unsupported[[]byte, UserCreated],
	))
	sealed := reg.Seal()

	// Write side still works.
	name, data, err := typemux.Serialize[string, []byte](sealed, UserCreated{ID: "u1"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if name != "user" || string(data) != "u1" {
		t.Errorf("name=%q data=%q", name, data)
	}

	// Read side returns ErrUnsupported.
	_, err = typemux.CreateType(sealed, "user", []byte("anything"))
	if !errors.Is(err, typemux.ErrUnsupported) {
		t.Errorf("expected ErrUnsupported, got %v", err)
	}
}

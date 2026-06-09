package typemux_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/struct0x/typemux"
)

func TestSerialize_HappyPath(t *testing.T) {
	reg := typemux.NewRegistry()
	typemux.RegisterCodec(reg, "user_created", typemux.JSONCodec[UserCreated]())

	value := UserCreated{ID: "u1", Name: "Alice"}

	t.Run("standard_registry", func(t *testing.T) {
		name, data, err := typemux.Serialize[string, []byte](reg, value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "user_created" {
			t.Errorf("expected name %q, got %q", "user_created", name)
		}

		var got UserCreated
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("invalid JSON output: %v", err)
		}
		if got != value {
			t.Errorf("round-trip mismatch: got %+v, want %+v", got, value)
		}
	})

	t.Run("sealed_registry", func(t *testing.T) {
		sealed := reg.Seal()

		name, data, err := typemux.Serialize[string, []byte](sealed, value)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "user_created" {
			t.Errorf("expected name %q, got %q", "user_created", name)
		}
		if len(data) == 0 {
			t.Error("expected non-empty data")
		}
	})
}

func TestSerialize_NotFound(t *testing.T) {
	reg := typemux.NewRegistry()

	_, _, err := typemux.Serialize[string, []byte](reg, UserCreated{ID: "u1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, typemux.ErrSerializerNotFound) {
		t.Errorf("expected ErrSerializerNotFound, got %v", err)
	}
}

func TestSerialize_NilValue(t *testing.T) {
	reg := typemux.NewRegistry()

	_, _, err := typemux.Serialize[string, []byte](reg, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, typemux.ErrSerializerNotFound) {
		t.Errorf("expected ErrSerializerNotFound, got %v", err)
	}
}

func TestSerialize_PointerFallback(t *testing.T) {
	reg := typemux.NewRegistry()
	typemux.RegisterCodec(reg, "user_created", typemux.JSONCodec[UserCreated]())

	value := &UserCreated{ID: "u1", Name: "Alice"}

	name, data, err := typemux.Serialize[string, []byte](reg, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user_created" {
		t.Errorf("expected name %q, got %q", "user_created", name)
	}

	var got UserCreated
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if got != *value {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, *value)
	}
}

func TestSerialize_KeyTypeMismatch(t *testing.T) {
	reg := typemux.NewRegistry()
	typemux.RegisterCodec(reg, "user_created", typemux.JSONCodec[UserCreated]())

	_, _, err := typemux.Serialize[int, []byte](reg, UserCreated{ID: "u1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, typemux.ErrKeyTypeMismatch) {
		t.Errorf("expected ErrKeyTypeMismatch, got %v", err)
	}
}

func TestSerialize_RoundTripWithCreateType(t *testing.T) {
	reg := typemux.NewRegistry()
	typemux.RegisterCodec(reg, "user_created", typemux.JSONCodec[UserCreated]())
	sealed := reg.Seal()

	original := UserCreated{ID: "u1", Name: "Alice"}

	name, data, err := typemux.Serialize[string, []byte](sealed, original)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	got, err := typemux.CreateType(sealed, name, data)
	if err != nil {
		t.Fatalf("CreateType failed: %v", err)
	}

	user, ok := got.(UserCreated)
	if !ok {
		t.Fatalf("expected UserCreated, got %T", got)
	}
	if user != original {
		t.Errorf("round-trip mismatch: got %+v, want %+v", user, original)
	}
}

func TestSerialize_NonStringKey(t *testing.T) {
	reg := typemux.NewRegistry()
	typemux.RegisterCodec(reg, 42, typemux.JSONCodec[OrderPlaced]())
	sealed := reg.Seal()

	name, data, err := typemux.Serialize[int, []byte](sealed, OrderPlaced{OrderID: "o1", Amount: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != 42 {
		t.Errorf("expected name 42, got %d", name)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
}

func TestRegisterCodec_ReplacesExisting(t *testing.T) {
	reg := typemux.NewRegistry()

	typemux.RegisterCodec(reg, "user", typemux.Codec[[]byte, UserCreated]{
		Marshal:   func(v UserCreated) ([]byte, error) { return []byte("first"), nil },
		Unmarshal: func(data []byte) (UserCreated, error) { return UserCreated{ID: "first"}, nil },
	})
	typemux.RegisterCodec(reg, "user", typemux.Codec[[]byte, UserCreated]{
		Marshal:   func(v UserCreated) ([]byte, error) { return []byte("second"), nil },
		Unmarshal: func(data []byte) (UserCreated, error) { return UserCreated{ID: "second"}, nil },
	})

	sealed := reg.Seal()

	_, data, err := typemux.Serialize[string, []byte](sealed, UserCreated{})
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}
	if string(data) != "second" {
		t.Errorf("expected second marshaler to be used, got %q", string(data))
	}

	got, err := typemux.CreateType(sealed, "user", []byte{})
	if err != nil {
		t.Fatalf("CreateType failed: %v", err)
	}
	if got.(UserCreated).ID != "second" {
		t.Errorf("expected second unmarshaler to be used, got ID=%s", got.(UserCreated).ID)
	}
}

func TestSerialize_MultipleDataTypesPerType(t *testing.T) {
	reg := typemux.NewRegistry()

	typemux.RegisterCodec(reg, "user", typemux.JSONCodec[UserCreated]())
	typemux.RegisterCodec(reg, "user", typemux.Codec[map[string]any, UserCreated]{
		Marshal: func(u UserCreated) (map[string]any, error) {
			return map[string]any{"id": u.ID, "name": u.Name}, nil
		},
		Unmarshal: func(m map[string]any) (UserCreated, error) {
			return UserCreated{ID: m["id"].(string), Name: m["name"].(string)}, nil
		},
	})

	sealed := reg.Seal()
	value := UserCreated{ID: "u1", Name: "Alice"}

	// []byte path
	name, bytes, err := typemux.Serialize[string, []byte](sealed, value)
	if err != nil {
		t.Fatalf("bytes path: %v", err)
	}
	if name != "user" {
		t.Errorf("bytes path name: %s", name)
	}
	if len(bytes) == 0 {
		t.Error("bytes path: empty output")
	}

	// map[string]any path
	name, m, err := typemux.Serialize[string, map[string]any](sealed, value)
	if err != nil {
		t.Fatalf("map path: %v", err)
	}
	if name != "user" {
		t.Errorf("map path name: %s", name)
	}
	if m["id"] != "u1" || m["name"] != "Alice" {
		t.Errorf("map path payload: %+v", m)
	}

	// Round-trip both ways through CreateType.
	bytesBack, err := typemux.CreateType(sealed, "user", bytes)
	if err != nil {
		t.Fatalf("CreateType bytes: %v", err)
	}
	if bytesBack.(UserCreated) != value {
		t.Errorf("bytes round-trip: got %+v want %+v", bytesBack, value)
	}

	mapBack, err := typemux.CreateType(sealed, "user", m)
	if err != nil {
		t.Fatalf("CreateType map: %v", err)
	}
	if mapBack.(UserCreated) != value {
		t.Errorf("map round-trip: got %+v want %+v", mapBack, value)
	}
}

func TestSerialize_DataTypeMismatch(t *testing.T) {
	reg := typemux.NewRegistry()
	typemux.RegisterCodec(reg, "user", typemux.JSONCodec[UserCreated]())
	sealed := reg.Seal()

	// Type is registered, but only for []byte. Asking for map[string]any → ErrDataTypeMismatch.
	_, _, err := typemux.Serialize[string, map[string]any](sealed, UserCreated{ID: "u1"})
	if err == nil {
		t.Fatal("expected error for unregistered DATA type")
	}
	if !errors.Is(err, typemux.ErrDataTypeMismatch) {
		t.Errorf("expected ErrDataTypeMismatch, got %v", err)
	}
}

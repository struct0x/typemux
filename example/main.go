package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/struct0x/typemux"
)

// Event types
type UserCreated struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type OrderPlaced struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	Amount     int    `json:"amount"`
}

type PaymentReceived struct {
	PaymentID string `json:"payment_id"`
	OrderID   string `json:"order_id"`
	Amount    int    `json:"amount"`
	Method    string `json:"method"`
}

func main() {
	h := &Handler{}

	// Create and configure the registry
	reg := typemux.NewRegistry()

	// Register factories for each event type
	typemux.RegisterFactory(reg, "user_created", typemux.JSONFactory[UserCreated]())
	typemux.RegisterDispatch(reg, h.handleUserCreated)

	typemux.RegisterFactory(reg, "order_placed", typemux.JSONFactory[OrderPlaced]())
	typemux.RegisterDispatch(reg, h.handleOrderPlaced)

	typemux.RegisterFactory(reg, "payment_received", typemux.JSONFactory[PaymentReceived]())
	typemux.RegisterDispatch(reg, h.handlePaymentReceived)

	// Seal the registry for production use
	sealed := reg.Seal()

	// Create HTTP handler
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse envelope
		var envelope struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}

		// Create typed value from envelope
		value, err := typemux.CreateType(sealed, envelope.Type, []byte(envelope.Data))
		if err != nil {
			http.Error(w, fmt.Sprintf("Unknown event type: %s", envelope.Type), http.StatusBadRequest)
			return
		}

		// Dispatch to handler with generic middleware (logging, timing)
		ctx := r.Context()
		if err := typemux.Dispatch(sealed, ctx, value, loggingMiddleware(), timingMiddleware); err != nil {
			http.Error(w, fmt.Sprintf("Handler error: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "Event %s processed successfully\n", envelope.Type)
	})

	// Start server
	addr := ":8080"
	log.Printf("Starting server on %s", addr)
	log.Printf("Try: curl -X POST http://localhost%s/events -d '{\"type\":\"user_created\",\"data\":{\"id\":\"u1\",\"name\":\"Alice\",\"email\":\"alice@example.com\"}}'", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

type Handler struct {
	// DB Conn
	// External System Conn
	// ...
}

// Handlers
func (h *Handler) handleUserCreated(ctx context.Context, e UserCreated) error {
	log.Printf("  -> Creating user: %s (%s) - %s", e.Name, e.ID, e.Email)
	// In real app: save to database, send welcome email, etc.
	return nil
}

func (h *Handler) handleOrderPlaced(ctx context.Context, e OrderPlaced) error {
	log.Printf("  -> Processing order: %s for customer %s, amount: $%d", e.OrderID, e.CustomerID, e.Amount)
	// In real app: reserve inventory, notify warehouse, etc.
	return nil
}

func (h *Handler) handlePaymentReceived(ctx context.Context, e PaymentReceived) error {
	log.Printf("  -> Payment received: %s for order %s, $%d via %s", e.PaymentID, e.OrderID, e.Amount, e.Method)
	// In real app: update order status, send receipt, etc.
	return nil
}

// Dispatch Middleware (applies to all event types at dispatch time)
func loggingMiddleware() typemux.DispatchMiddleware {
	return func(ctx context.Context, event any, next func(context.Context) error) error {
		log.Printf("Processing event: %T", event)
		err := next(ctx)
		if err != nil {
			log.Printf("Error processing %T: %v", event, err)
		} else {
			log.Printf("Successfully processed: %T", event)
		}
		return err
	}
}

func timingMiddleware(ctx context.Context, event any, next func(context.Context) error) error {
	start := time.Now()
	err := next(ctx)
	log.Printf("Event %T took %v", event, time.Since(start))
	return err
}

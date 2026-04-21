package checkout

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wydrox/martmart-cli/internal/session"
)

func TestFriscoPreview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/app/commerce/api/v1/users/u1/express-checkout/cart" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Fatalf("authorization = %q", got)
		}
		if got := r.Header.Get("X-Frisco-VisitorId"); got == "" {
			t.Fatal("expected X-Frisco-VisitorId header")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cartId":   "cart-1",
			"items":    []any{map[string]any{"id": "a"}, map[string]any{"id": "b"}},
			"total":    123.45,
			"currency": "PLN",
			"reservation": map[string]any{
				"startsAt":       "2026-04-22T06:00:00Z",
				"endsAt":         "2026-04-22T07:00:00Z",
				"deliveryMethod": "Van",
			},
			"payment": map[string]any{
				"method":  "CARD",
				"channel": "Dotpay",
				"status":  "Ready",
			},
		})
	}))
	defer server.Close()

	client := NewFriscoClientForTests(server.Client())
	s := &session.Session{BaseURL: server.URL, Token: "tok", UserID: "u1"}
	preview, err := client.Preview(s, PreviewOptions{})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if preview.Provider != session.ProviderFrisco || preview.UserID != "u1" {
		t.Fatalf("unexpected preview identity: %+v", preview)
	}
	if preview.CartID != "cart-1" || preview.ItemCount != 2 || preview.Total == nil || preview.Total.Amount != 123.45 {
		t.Fatalf("unexpected preview summary: %+v", preview)
	}
	if !preview.ReadyToFinalize {
		t.Fatalf("expected ready preview: %+v", preview)
	}
	if preview.Payment == nil || preview.Payment.Method != "CARD" {
		t.Fatalf("expected parsed payment: %+v", preview.Payment)
	}
}

func TestFriscoPreviewUnsupportedProvider(t *testing.T) {
	client := NewFriscoClient()
	_, err := client.Preview(&session.Session{BaseURL: session.DefaultDelioBaseURL, UserID: "u1"}, PreviewOptions{})
	var target *UnsupportedProviderError
	if !errors.As(err, &target) {
		t.Fatalf("expected UnsupportedProviderError, got %v", err)
	}
}

func TestFriscoFinalizeGuardMismatch(t *testing.T) {
	var postCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/app/commerce/api/v1/users/u1/express-checkout/cart":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"cartId":      "cart-1",
				"items":       []any{map[string]any{"id": "a"}},
				"total":       10.00,
				"reservation": map[string]any{"startsAt": "2026-04-22T06:00:00Z", "endsAt": "2026-04-22T07:00:00Z"},
				"payment":     map[string]any{"method": "CARD"},
			})
		case "/app/commerce/api/v1/users/u1/express-checkout/cart/order":
			postCalled = true
			t.Fatal("finalize POST should not be called on guard mismatch")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewFriscoClientForTests(server.Client())
	s := &session.Session{BaseURL: server.URL, Token: "tok", UserID: "u1"}
	expected := 11.00
	_, err := client.Finalize(s, FinalizeOptions{Guard: &FinalizeGuard{ExpectedTotal: &expected}})
	if postCalled {
		t.Fatal("expected no POST on guard mismatch")
	}
	var target *GuardMismatchError
	if !errors.As(err, &target) {
		t.Fatalf("expected GuardMismatchError, got %v", err)
	}
}

func TestFriscoFinalizeActionRequired(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/app/commerce/api/v1/users/u1/express-checkout/cart":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"cartId":      "cart-1",
				"items":       []any{map[string]any{"id": "a"}},
				"total":       10.00,
				"reservation": map[string]any{"startsAt": "2026-04-22T06:00:00Z", "endsAt": "2026-04-22T07:00:00Z"},
				"payment":     map[string]any{"method": "CARD"},
			})
		case "/app/commerce/api/v1/users/u1/express-checkout/cart/order":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"orderId": "ord-1",
				"status":  "RedirectRequired",
				"payment": map[string]any{
					"redirectUrl": "https://bank.example/3ds",
					"method":      http.MethodGet,
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewFriscoClientForTests(server.Client())
	s := &session.Session{BaseURL: server.URL, Token: "tok", UserID: "u1"}
	result, err := client.Finalize(s, FinalizeOptions{})
	var target *ActionRequiredError
	if !errors.As(err, &target) {
		t.Fatalf("expected ActionRequiredError, got %v", err)
	}
	if result == nil || result.Status != FinalizeStatusRequiresAction || result.Action == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Action.URL != "https://bank.example/3ds" || result.Action.Kind != PaymentActionKindRedirect {
		t.Fatalf("unexpected action: %+v", result.Action)
	}
	if got := strings.Join(calls, " | "); strings.Contains(got, "/orders/") {
		t.Fatalf("readback should not happen for action-required finalize: %s", got)
	}
}

func TestFriscoFinalizeSuccessReadback(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/app/commerce/api/v1/users/u1/express-checkout/cart":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"cartId":      "cart-1",
				"items":       []any{map[string]any{"id": "a"}},
				"total":       10.00,
				"reservation": map[string]any{"startsAt": "2026-04-22T06:00:00Z", "endsAt": "2026-04-22T07:00:00Z"},
				"payment":     map[string]any{"method": "CARD", "status": "Ready"},
			})
		case "/app/commerce/api/v1/users/u1/express-checkout/cart/order":
			_ = json.NewEncoder(w).Encode(map[string]any{"orderId": "ord-123", "status": "Placed"})
		case "/app/commerce/api/v1/users/u1/orders/ord-123":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "ord-123", "status": "Placed", "total": 10.00})
		case "/app/commerce/api/v1/users/u1/orders/ord-123/payments":
			_ = json.NewEncoder(w).Encode([]any{map[string]any{"status": "Paid", "channelName": "Dotpay"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewFriscoClientForTests(server.Client())
	s := &session.Session{BaseURL: server.URL, Token: "tok", UserID: "u1"}
	result, err := client.Finalize(s, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if result.Status != FinalizeStatusPlaced || result.OrderID != "ord-123" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Readback == nil || result.Readback.OrderID != "ord-123" || len(result.Readback.Payments) != 1 {
		t.Fatalf("expected readback: %+v", result.Readback)
	}
	if got := strings.Join(calls, " | "); !strings.Contains(got, "GET /app/commerce/api/v1/users/u1/orders/ord-123") {
		t.Fatalf("expected order readback calls, got: %s", got)
	}
}

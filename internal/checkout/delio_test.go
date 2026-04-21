package checkout

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wydrox/martmart-cli/internal/session"
)

func TestDelioPreviewNormalizesCheckoutState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		switch body["operationName"] {
		case "CurrentCart":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"currentCart": map[string]any{
						"id": "cart-delio-1",
						"lineItems": []any{
							map[string]any{"id": "l1"},
							map[string]any{"id": "l2"},
						},
						"totalPrice": map[string]any{"centAmount": 14660.0, "currencyCode": "PLN"},
						"deliveryScheduleSlot": map[string]any{
							"dateFrom": "2026-04-22T18:00:00+02:00",
							"dateTo":   "2026-04-22T20:00:00+02:00",
						},
						"shippingMethod": map[string]any{"type": "courier", "name": "Courier"},
						"darkstoreKey":   "waw-1",
						"paymentInfo":    map[string]any{"payments": []any{}},
					},
				},
			})
		case "CustomerDefaultBillingAddress":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"customerDefaultBillingAddress": map[string]any{
						"id": "bill-1",
						"billingAddress": map[string]any{
							"firstName":  "Ada",
							"lastName":   "Lovelace",
							"email":      "ada@example.com",
							"streetName": "Main",
						},
					},
				},
			})
		case "PaymentSettings":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"paymentSettings": map[string]any{"adyenClientKey": "test_key"},
				},
			})
		default:
			t.Fatalf("unexpected operation: %v", body["operationName"])
		}
	}))
	defer server.Close()

	client := NewDelioClient()
	s := &session.Session{BaseURL: server.URL, UserID: "d-1", Headers: map[string]string{"Cookie": "authtoken=abc"}}
	preview, err := client.Preview(s, PreviewOptions{Provider: session.ProviderDelio})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if preview.Provider != session.ProviderDelio || preview.UserID != "d-1" {
		t.Fatalf("unexpected preview identity: %+v", preview)
	}
	if preview.CartID != "cart-delio-1" || preview.ItemCount != 2 {
		t.Fatalf("unexpected cart summary: %+v", preview)
	}
	if preview.Total == nil || preview.Total.Amount != 146.60 || preview.Total.Currency != "PLN" {
		t.Fatalf("unexpected total: %+v", preview.Total)
	}
	if preview.Reservation == nil || preview.Reservation.DeliveryMethod != "courier" {
		t.Fatalf("unexpected reservation: %+v", preview.Reservation)
	}
	if preview.Payment == nil || preview.Payment.Channel != "Adyen" || preview.Payment.Status != "ready" {
		t.Fatalf("unexpected payment state: %+v", preview.Payment)
	}
	if !preview.ReadyToFinalize || len(preview.Issues) != 0 {
		t.Fatalf("expected ready preview, got %+v", preview)
	}
}

func TestDelioFinalizeRedirectActionRequired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch body["operationName"] {
		case "CurrentCart":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"currentCart": map[string]any{
						"id":                   "cart-delio-2",
						"lineItems":            []any{map[string]any{"id": "l1"}},
						"totalPrice":           map[string]any{"centAmount": 2500.0, "currencyCode": "PLN"},
						"deliveryScheduleSlot": map[string]any{"dateFrom": "2026-04-22T18:00:00+02:00", "dateTo": "2026-04-22T20:00:00+02:00"},
						"shippingMethod":       map[string]any{"type": "courier"},
						"billingAddress":       map[string]any{"streetName": "Main"},
					},
				},
			})
		case "PaymentSettings":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"paymentSettings": map[string]any{"adyenClientKey": "test_key"}}})
		case "CreatePayment":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"createPayment": map[string]any{"paymentId": "pay-1"}}})
		case "PaymentMethods":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"getPaymentMethods": map[string]any{
						"adyenResponse": map[string]any{
							"storedPaymentMethods": []any{
								map[string]any{"type": "scheme", "storedPaymentMethodId": "pm_1"},
							},
						},
					},
				},
			})
		case "MakePayment":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"makePayment": map[string]any{
						"adyenResponse": map[string]any{
							"resultCode": "RedirectShopper",
							"action": map[string]any{
								"type":   "redirect",
								"method": "GET",
								"url":    "https://checkoutshopper-live.adyen.com/checkoutshopper/threeDS/redirect.shtml?redirectData=abc",
							},
							"paymentData": "pd-1",
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected operation: %v", body["operationName"])
		}
	}))
	defer server.Close()

	client := NewDelioClient()
	s := &session.Session{BaseURL: server.URL, Headers: map[string]string{"Cookie": "authtoken=abc"}}
	result, err := client.Finalize(s, FinalizeOptions{Provider: session.ProviderDelio})
	var target *ActionRequiredError
	if !errors.As(err, &target) {
		t.Fatalf("expected ActionRequiredError, got %v", err)
	}
	if result == nil || result.Status != FinalizeStatusRequiresAction || result.Action == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Action.Kind != PaymentActionKindRedirect || result.Action.URL == "" {
		t.Fatalf("unexpected action: %+v", result.Action)
	}
}

func TestDelioFinalizeAuthorisedStaysPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch body["operationName"] {
		case "CurrentCart":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"currentCart": map[string]any{
						"id":                   "cart-delio-3",
						"lineItems":            []any{map[string]any{"id": "l1"}},
						"totalPrice":           map[string]any{"centAmount": 2500.0, "currencyCode": "PLN"},
						"deliveryScheduleSlot": map[string]any{"dateFrom": "2026-04-22T18:00:00+02:00", "dateTo": "2026-04-22T20:00:00+02:00"},
						"shippingMethod":       map[string]any{"type": "courier"},
						"billingAddress":       map[string]any{"streetName": "Main"},
					},
				},
			})
		case "PaymentSettings":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"paymentSettings": map[string]any{"adyenClientKey": "test_key"}}})
		case "CreatePayment":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"createPayment": map[string]any{"paymentId": "pay-2"}}})
		case "PaymentMethods":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"getPaymentMethods": map[string]any{
						"adyenResponse": map[string]any{
							"storedPaymentMethods": []any{
								map[string]any{"type": "scheme", "storedPaymentMethodId": "pm_2"},
							},
						},
					},
				},
			})
		case "MakePayment":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"makePayment": map[string]any{
						"adyenResponse": map[string]any{"resultCode": "Authorised"},
					},
				},
			})
		default:
			t.Fatalf("unexpected operation: %v", body["operationName"])
		}
	}))
	defer server.Close()

	client := NewDelioClient()
	s := &session.Session{BaseURL: server.URL, Headers: map[string]string{"Cookie": "authtoken=abc"}}
	result, err := client.Finalize(s, FinalizeOptions{Provider: session.ProviderDelio})
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if result.Status != FinalizeStatusPending {
		t.Fatalf("status = %s, want %s", result.Status, FinalizeStatusPending)
	}
	if result.OrderID != "" {
		t.Fatalf("order_id = %q, want empty without explicit proof", result.OrderID)
	}
}

package delio

import "testing"

func TestExtractUpdatedCart(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		got, err := ExtractUpdatedCart(map[string]any{
			"data": map[string]any{"updateCart": map[string]any{"id": "cart-1"}},
		})
		if err != nil {
			t.Fatalf("ExtractUpdatedCart: %v", err)
		}
		if got["id"] != "cart-1" {
			t.Fatalf("id=%v want cart-1", got["id"])
		}
	})

	t.Run("graphql errors", func(t *testing.T) {
		_, err := ExtractUpdatedCart(map[string]any{
			"errors": []any{map[string]any{"message": "out of stock"}},
			"data":   map[string]any{"updateCart": map[string]any{"id": "cart-1"}},
		})
		if !IsUpdateCurrentCartBusinessError(err) {
			t.Fatalf("expected business error, got %v", err)
		}
	})

	t.Run("missing updateCart", func(t *testing.T) {
		_, err := ExtractUpdatedCart(map[string]any{
			"data": map[string]any{},
		})
		if !IsUpdateCurrentCartBusinessError(err) {
			t.Fatalf("expected business error, got %v", err)
		}
	})
}

func TestExtractPaymentSettings(t *testing.T) {
	payload := map[string]any{
		"data": map[string]any{
			"paymentSettings": map[string]any{"adyenClientKey": "test_key"},
		},
	}

	settings, err := ExtractPaymentSettings(payload)
	if err != nil {
		t.Fatalf("ExtractPaymentSettings: %v", err)
	}
	if got := settings["adyenClientKey"]; got != "test_key" {
		t.Fatalf("adyenClientKey=%v want test_key", got)
	}

	key, err := ExtractAdyenClientKey(payload)
	if err != nil {
		t.Fatalf("ExtractAdyenClientKey: %v", err)
	}
	if key != "test_key" {
		t.Fatalf("key=%q want test_key", key)
	}
}

func TestExtractPaymentID(t *testing.T) {
	t.Run("nested createPayment payload", func(t *testing.T) {
		paymentID, err := ExtractPaymentID(map[string]any{
			"data": map[string]any{
				"createPayment": map[string]any{"paymentId": "pay_123"},
			},
		})
		if err != nil {
			t.Fatalf("ExtractPaymentID: %v", err)
		}
		if paymentID != "pay_123" {
			t.Fatalf("paymentID=%q want pay_123", paymentID)
		}
	})

	t.Run("top-level paymentId fallback", func(t *testing.T) {
		paymentID, err := ExtractPaymentID(map[string]any{
			"data": map[string]any{"paymentId": "pay_456"},
		})
		if err != nil {
			t.Fatalf("ExtractPaymentID: %v", err)
		}
		if paymentID != "pay_456" {
			t.Fatalf("paymentID=%q want pay_456", paymentID)
		}
	})
}

func TestExtractPaymentMethods(t *testing.T) {
	payload := map[string]any{
		"data": map[string]any{
			"getPaymentMethods": map[string]any{"adyenResponse": "{\"paymentMethods\":[]}"},
		},
	}

	methods, err := ExtractPaymentMethods(payload)
	if err != nil {
		t.Fatalf("ExtractPaymentMethods: %v", err)
	}
	if got := methods["adyenResponse"]; got != "{\"paymentMethods\":[]}" {
		t.Fatalf("adyenResponse=%v want JSON payload", got)
	}

	response, err := ExtractAdyenResponse(payload)
	if err != nil {
		t.Fatalf("ExtractAdyenResponse: %v", err)
	}
	if response != "{\"paymentMethods\":[]}" {
		t.Fatalf("response=%v want JSON payload", response)
	}
}

func TestBuildCheckoutHelpers(t *testing.T) {
	action := BuildSetDeliveryScheduleSlotAction(" 2026-04-21T08:00:00Z ", "2026-04-21T10:00:00Z ")
	setSlot, ok := action["SetDeliveryScheduleSlot"].(map[string]any)
	if !ok {
		t.Fatalf("SetDeliveryScheduleSlot action missing: %#v", action)
	}
	deliverySlot, ok := setSlot["deliveryScheduleSlot"].(map[string]any)
	if !ok {
		t.Fatalf("deliveryScheduleSlot missing: %#v", setSlot)
	}
	if deliverySlot["dateFrom"] != "2026-04-21T08:00:00Z" || deliverySlot["dateTo"] != "2026-04-21T10:00:00Z" {
		t.Fatalf("unexpected slot action: %#v", deliverySlot)
	}

	config := BuildMakePaymentConfig(map[string]any{"type": "scheme", "storedPaymentMethodId": "pm_1"}, " https://delio.com.pl/checkout/payment ", true)
	if config["paymentChannel"] != "Web" {
		t.Fatalf("paymentChannel=%v want Web", config["paymentChannel"])
	}
	if config["returnUrl"] != "https://delio.com.pl/checkout/payment" {
		t.Fatalf("returnUrl=%v want trimmed url", config["returnUrl"])
	}
	if config["storePaymentMethod"] != true {
		t.Fatalf("storePaymentMethod=%v want true", config["storePaymentMethod"])
	}
	paymentMethod, ok := config["paymentMethod"].(map[string]any)
	if !ok {
		t.Fatalf("paymentMethod missing: %#v", config)
	}
	adyenPayload, ok := paymentMethod["adyenPayload"].(map[string]any)
	if !ok || adyenPayload["storedPaymentMethodId"] != "pm_1" {
		t.Fatalf("adyenPayload=%#v want stored method payload", paymentMethod["adyenPayload"])
	}
}

func TestPaymentWrapperValidation(t *testing.T) {
	tests := []struct {
		name string
		call func() error
		want string
	}{
		{name: "create payment missing cart", call: func() error { _, err := CreatePayment(nil, " "); return err }, want: "missing cartId"},
		{name: "payment methods missing cart", call: func() error { _, err := PaymentMethods(nil, "", "pay_1"); return err }, want: "missing cartId"},
		{name: "payment methods missing payment", call: func() error { _, err := PaymentMethods(nil, "cart_1", " "); return err }, want: "missing paymentId"},
		{name: "make payment missing cart", call: func() error { _, err := MakePayment(nil, "", "pay_1", map[string]any{"paymentChannel": "Web"}); return err }, want: "missing cartId"},
		{name: "make payment missing payment", call: func() error { _, err := MakePayment(nil, "cart_1", "", map[string]any{"paymentChannel": "Web"}); return err }, want: "missing paymentId"},
		{name: "make payment missing config", call: func() error { _, err := MakePayment(nil, "cart_1", "pay_1", nil); return err }, want: "missing paymentConfig"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil || err.Error() != tt.want {
				t.Fatalf("err=%v want %q", err, tt.want)
			}
		})
	}
}

func TestExtractMakePaymentResult(t *testing.T) {
	payload := map[string]any{
		"data": map[string]any{
			"makePayment": map[string]any{"adyenResponse": map[string]any{"resultCode": "Authorised"}},
		},
	}

	result, err := ExtractMakePaymentResult(payload)
	if err != nil {
		t.Fatalf("ExtractMakePaymentResult: %v", err)
	}
	adyenResponse, ok := result["adyenResponse"].(map[string]any)
	if !ok || adyenResponse["resultCode"] != "Authorised" {
		t.Fatalf("adyenResponse=%#v want resultCode Authorised", result["adyenResponse"])
	}

	response, err := ExtractMakePaymentAdyenResponse(payload)
	if err != nil {
		t.Fatalf("ExtractMakePaymentAdyenResponse: %v", err)
	}
	responseMap, ok := response.(map[string]any)
	if !ok || responseMap["resultCode"] != "Authorised" {
		t.Fatalf("response=%#v want resultCode Authorised", response)
	}
}

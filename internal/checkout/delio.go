package checkout

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/wydrox/martmart-cli/internal/delio"
	"github.com/wydrox/martmart-cli/internal/session"
)

const delioProvider = session.ProviderDelio

// DelioClient implements the Delio checkout core using internal/delio helpers.
type DelioClient struct{}

// NewDelioClient returns a checkout client backed by internal/delio helpers.
func NewDelioClient() *DelioClient {
	return &DelioClient{}
}

// Preview fetches and normalizes the current Delio checkout state.
func (c *DelioClient) Preview(s *session.Session, opts PreviewOptions) (*CheckoutPreview, error) {
	provider, uid, err := c.resolveDelio(s, opts.Provider, opts.UserID)
	if err != nil {
		return nil, err
	}

	currentCartPayload, err := delio.CurrentCart(s)
	if err != nil {
		return nil, err
	}
	cart, err := delio.ExtractCurrentCart(currentCartPayload)
	if err != nil {
		return nil, &MalformedResponseError{Operation: "CurrentCart", Message: err.Error()}
	}

	raw := map[string]any{"currentCart": currentCartPayload}
	preview := &CheckoutPreview{
		Provider: provider,
		UserID:   uid,
		CartID:   delioCartID(cart),
		ItemCount: delioCartItemCount(cart),
		Total:    delioCartTotal(cart),
		Reservation: delioCartReservation(cart),
		Payment:  nil,
		Issues:   nil,
		Raw:      raw,
	}

	billingAddress := delioBillingAddress(cart)
	billingReady := len(billingAddress) > 0
	if !billingReady {
		if billingPayload, billErr := delio.CustomerDefaultBillingAddress(s); billErr == nil {
			raw["defaultBillingAddress"] = billingPayload
			if extracted, extractErr := delio.ExtractCustomerDefaultBillingAddress(billingPayload); extractErr == nil {
				billingAddress = delioDefaultBillingAddress(extracted)
				billingReady = len(billingAddress) > 0
			}
		}
	}
	if len(billingAddress) > 0 {
		raw["effectiveBillingAddress"] = billingAddress
	}

	paymentReady := false
	paymentStatus := "missing"
	paymentSelection := &PaymentSelection{Channel: "Adyen", Status: paymentStatus}
	if paymentSettingsPayload, payErr := delio.PaymentSettings(s); payErr == nil {
		raw["paymentSettings"] = paymentSettingsPayload
		if clientKey, keyErr := delio.ExtractAdyenClientKey(paymentSettingsPayload); keyErr == nil && strings.TrimSpace(clientKey) != "" {
			paymentReady = true
			paymentSelection.Status = "ready"
		}
	}
	if paymentInfo := delioMapField(cart, "paymentInfo"); paymentInfo != nil {
		if payments := delioListField(paymentInfo, "payments"); len(payments) > 0 {
			paymentSelection.Method = firstNonEmpty(delioString(delioMapFromAny(payments[0]), "type"), paymentSelection.Method)
			if !paymentReady {
				paymentSelection.Status = "created"
			}
		}
	}
	if paymentSelection.Status != "missing" || paymentSelection.Channel != "" || paymentSelection.Method != "" {
		preview.Payment = paymentSelection
	}

	availableSlots := []any(nil)
	if preview.Reservation == nil {
		if slotsPayload, slotErr := delio.DeliveryScheduleSlots(s, nil); slotErr == nil {
			raw["deliveryScheduleSlots"] = slotsPayload
			if slots, extractErr := delio.ExtractDeliveryScheduleSlots(slotsPayload); extractErr == nil {
				availableSlots = slots
			}
		}
	}

	preview.Issues = delioPreviewIssues(preview, billingReady, paymentReady, availableSlots)
	preview.ReadyToFinalize = len(preview.Issues) == 0 && preview.ItemCount > 0 && preview.Reservation != nil && billingReady && paymentReady
	return preview, nil
}

// Finalize safely attempts the Delio payment step and only reports placed when
// the provider response clearly proves the order was placed.
func (c *DelioClient) Finalize(s *session.Session, opts FinalizeOptions) (*FinalizeResult, error) {
	provider, uid, err := c.resolveDelio(s, opts.Provider, opts.UserID)
	if err != nil {
		return nil, err
	}
	preview, err := c.Preview(s, PreviewOptions{Provider: provider, UserID: uid})
	if err != nil {
		return nil, err
	}
	if err := validateGuard(preview, opts.Guard); err != nil {
		return nil, err
	}

	result := &FinalizeResult{
		Provider: provider,
		UserID:   uid,
		Preview:  preview,
		Status:   FinalizeStatusPending,
		APIResponse: map[string]any{},
	}
	if !preview.ReadyToFinalize {
		result.APIResponse["reason"] = "checkout_not_ready"
		return result, nil
	}

	createPaymentPayload, err := delio.CreatePayment(s, preview.CartID)
	if err != nil {
		return nil, err
	}
	result.APIResponse["createPayment"] = createPaymentPayload
	paymentID, err := delio.ExtractPaymentID(createPaymentPayload)
	if err != nil {
		return nil, &MalformedResponseError{Operation: "CreatePayment", Message: err.Error()}
	}
	result.APIResponse["paymentId"] = paymentID

	paymentMethodsPayload, err := delio.PaymentMethods(s, preview.CartID, paymentID)
	if err != nil {
		return nil, err
	}
	result.APIResponse["paymentMethods"] = paymentMethodsPayload
	adyenMethods, err := delio.ExtractAdyenResponse(paymentMethodsPayload)
	if err != nil {
		return nil, &MalformedResponseError{Operation: "PaymentMethods", Message: err.Error()}
	}
	result.APIResponse["paymentMethodsAdyenResponse"] = adyenMethods

	paymentMethod := delioSelectStoredPaymentMethod(adyenMethods)
	if len(paymentMethod) == 0 {
		result.APIResponse["reason"] = "no_reusable_payment_method"
		return result, nil
	}

	makePaymentPayload, err := delio.MakePayment(s, preview.CartID, paymentID, delio.BuildMakePaymentConfig(paymentMethod, delioReturnURL(s), false))
	if err != nil {
		return nil, err
	}
	result.APIResponse["makePayment"] = makePaymentPayload
	makePaymentResult, err := delio.ExtractMakePaymentResult(makePaymentPayload)
	if err != nil {
		return nil, &MalformedResponseError{Operation: "MakePayment", Message: err.Error()}
	}

	result.OrderID = firstNonEmpty(delioString(makePaymentResult, "orderId"), delioString(makePaymentPayload, "orderId"))
	result.Action = detectDelioPaymentAction(makePaymentResult)
	if result.Action == nil {
		result.Action = detectDelioPaymentAction(delioMapFromAny(result.APIResponse["makePayment"]))
	}
	if result.Action != nil {
		result.Status = FinalizeStatusRequiresAction
		if !opts.AllowActionRequired {
			return result, &ActionRequiredError{Action: result.Action, Result: result}
		}
		return result, nil
	}

	result.Status = classifyDelioFinalizeStatus(makePaymentResult, result.OrderID)
	return result, nil
}

func (c *DelioClient) resolveDelio(s *session.Session, provider, userID string) (string, string, error) {
	provider = session.NormalizeProvider(strings.TrimSpace(provider))
	if provider == "" {
		provider = session.ProviderForSession(s, session.ProviderDelio)
	}
	if provider != delioProvider {
		return "", "", &UnsupportedProviderError{Provider: provider, Supported: []string{delioProvider}}
	}
	uid := strings.TrimSpace(userID)
	if uid == "" {
		uid = session.UserIDString(s)
	}
	return provider, uid, nil
}

func delioPreviewIssues(preview *CheckoutPreview, billingReady, paymentReady bool, availableSlots []any) []CheckoutIssue {
	issues := make([]CheckoutIssue, 0, 4)
	if preview.ItemCount == 0 {
		issues = append(issues, CheckoutIssue{Code: "empty_cart", Message: "cart has no items"})
	}
	if preview.Reservation == nil {
		code := "missing_delivery_slot"
		message := "delivery slot is not selected"
		if len(availableSlots) == 0 {
			code = "no_delivery_slots"
			message = "no delivery slots are currently available"
		}
		issues = append(issues, CheckoutIssue{Code: code, Message: message})
	}
	if !billingReady {
		issues = append(issues, CheckoutIssue{Code: "missing_billing_address", Message: "billing address is not configured"})
	}
	if !paymentReady {
		issues = append(issues, CheckoutIssue{Code: "missing_payment_configuration", Message: "payment configuration is not ready"})
	}
	return issues
}

func delioCartID(cart map[string]any) string {
	return firstNonEmpty(delioString(cart, "id"), delioString(cart, "cartId"))
}

func delioCartItemCount(cart map[string]any) int {
	return len(delioListField(cart, "lineItems"))
}

func delioCartTotal(cart map[string]any) *Money {
	total := delioMapField(cart, "totalPrice")
	if total == nil {
		return nil
	}
	centAmount, ok := delioNumber(total["centAmount"])
	if !ok {
		return nil
	}
	return &Money{Amount: centAmount / 100.0, Currency: firstNonEmpty(delioString(total, "currencyCode"), "PLN")}
}

func delioCartReservation(cart map[string]any) *ReservationWindow {
	slot := delioMapField(cart, "deliveryScheduleSlot")
	if slot == nil {
		return nil
	}
	shippingMethod := delioMapField(cart, "shippingMethod")
	return &ReservationWindow{
		StartsAt:       delioString(slot, "dateFrom"),
		EndsAt:         delioString(slot, "dateTo"),
		DeliveryMethod: firstNonEmpty(delioString(shippingMethod, "type"), delioString(shippingMethod, "name"), delioString(shippingMethod, "key")),
		Warehouse:      firstNonEmpty(delioString(cart, "darkstoreKey"), delioString(cart, "context")),
	}
}

func delioBillingAddress(cart map[string]any) map[string]any {
	return delioMapField(cart, "billingAddress")
}

func delioDefaultBillingAddress(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	if nested := delioMapField(payload, "billingAddress"); nested != nil {
		return nested
	}
	return payload
}

func delioReturnURL(s *session.Session) string {
	baseURL := strings.TrimRight(strings.TrimSpace(session.DefaultBaseURLForProvider(session.ProviderDelio)), "/")
	if s != nil && strings.TrimSpace(s.BaseURL) != "" {
		baseURL = strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
	}
	return baseURL + "/checkout/payment"
}

func delioSelectStoredPaymentMethod(adyenResponse any) map[string]any {
	parsed := delioDecodeAnyMap(adyenResponse)
	if parsed == nil {
		return nil
	}
	for _, key := range []string{"storedPaymentMethods", "paymentMethods"} {
		for _, item := range delioListField(parsed, key) {
			method := delioMapFromAny(item)
			if len(method) == 0 {
				continue
			}
			if firstNonEmpty(delioString(method, "storedPaymentMethodId"), delioString(method, "id")) != "" {
				return method
			}
		}
	}
	return nil
}

func detectDelioPaymentAction(payload map[string]any) *PaymentAction {
	if payload == nil {
		return nil
	}
	if adyenAction := delioActionFromMap(payload); adyenAction != nil {
		return adyenAction
	}
	if adyenResponse := delioDecodeAnyMap(payload["adyenResponse"]); adyenResponse != nil {
		if adyenAction := delioActionFromMap(adyenResponse); adyenAction != nil {
			return adyenAction
		}
		if generic := detectPaymentAction(adyenResponse); generic != nil {
			return generic
		}
	}
	return detectPaymentAction(payload)
}

func delioActionFromMap(payload map[string]any) *PaymentAction {
	action := delioMapField(payload, "action")
	if action == nil {
		return nil
	}
	kind := PaymentActionKindRedirect
	actionType := strings.ToLower(firstNonEmpty(delioString(action, "type"), delioString(action, "paymentMethodType")))
	if strings.Contains(actionType, "3ds") || strings.Contains(actionType, "three") || strings.Contains(actionType, "challenge") {
		kind = PaymentActionKind3DS
	}
	url := firstNonEmpty(delioString(action, "url"), delioString(action, "redirectUrl"), delioString(action, "acsUrl"))
	method := firstNonEmpty(delioString(action, "method"), http.MethodGet)
	if url == "" && len(action) == 0 {
		return nil
	}
	payloadCopy := map[string]any{}
	for k, v := range action {
		payloadCopy[k] = v
	}
	if paymentData := delioString(payload, "paymentData"); paymentData != "" {
		payloadCopy["paymentData"] = paymentData
	}
	if resultCode := delioString(payload, "resultCode"); resultCode != "" {
		payloadCopy["resultCode"] = resultCode
	}
	return &PaymentAction{
		Kind:    kind,
		URL:     url,
		Method:  method,
		Message: firstNonEmpty(delioString(payload, "status"), delioString(payload, "resultCode"), delioString(action, "type")),
		Payload: payloadCopy,
	}
}

func classifyDelioFinalizeStatus(payload map[string]any, orderID string) FinalizeStatus {
	if action := detectDelioPaymentAction(payload); action != nil {
		return FinalizeStatusRequiresAction
	}
	for _, candidate := range []string{
		delioString(payload, "status"),
		delioString(delioMapField(payload, "payment"), "status"),
		delioString(delioDecodeAnyMap(payload["adyenResponse"]), "resultCode"),
		delioString(payload, "resultCode"),
	} {
		switch strings.ToLower(strings.TrimSpace(candidate)) {
		case "placed", "completed", "confirmed":
			if strings.TrimSpace(orderID) != "" {
				return FinalizeStatusPlaced
			}
		case "requires_action", "requiresaction", "redirectshopper", "challengeshopper", "identifyshopper", "presenttoshopper":
			return FinalizeStatusRequiresAction
		case "authorised", "authorized", "pending", "received", "processing", "authorising", "authorizing":
			return FinalizeStatusPending
		}
	}
	if strings.TrimSpace(orderID) != "" && strings.EqualFold(delioString(payload, "status"), "placed") {
		return FinalizeStatusPlaced
	}
	return FinalizeStatusPending
}

func delioDecodeAnyMap(v any) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		return t
	case string:
		trimmed := strings.TrimSpace(t)
		if trimmed == "" || (!strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[")) {
			return nil
		}
		var decoded any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return nil
		}
		if m, ok := decoded.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func delioMapFromAny(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func delioMapField(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	out, _ := m[key].(map[string]any)
	return out
}

func delioListField(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	out, _ := m[key].([]any)
	return out
}

func delioString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(strings.TrimSpace(toString(m[key]))), "<nil>"), "<nil>")))
}

func delioNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(t)
		if string(b) == "null" || string(b) == "\"\"" {
			return ""
		}
		if len(b) > 0 && b[0] == '"' {
			var s string
			if err := json.Unmarshal(b, &s); err == nil {
				return s
			}
		}
		return strings.TrimSpace(string(b))
	}
}

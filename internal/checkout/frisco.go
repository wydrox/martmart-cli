package checkout

import (
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
)

const friscoProvider = session.ProviderFrisco

type requestFunc func(s *session.Session, method, pathOrURL string, opts httpclient.RequestOpts) (any, error)

// FriscoClient implements the Frisco-only express-checkout core flow.
type FriscoClient struct {
	request    requestFunc
	httpClient *http.Client
}

// NewFriscoClient returns a checkout client backed by internal/httpclient.
func NewFriscoClient() *FriscoClient {
	return &FriscoClient{request: httpclient.RequestJSON}
}

// NewFriscoClientForTests allows tests to inject a custom HTTP client.
func NewFriscoClientForTests(httpClient *http.Client) *FriscoClient {
	return &FriscoClient{request: httpclient.RequestJSON, httpClient: httpClient}
}

// Preview fetches and normalizes Frisco express-checkout cart data.
func (c *FriscoClient) Preview(s *session.Session, opts PreviewOptions) (*CheckoutPreview, error) {
	provider, uid, err := c.resolveFrisco(s, opts.Provider, opts.UserID)
	if err != nil {
		return nil, err
	}
	payload, err := c.requestMap(s, http.MethodGet, fmt.Sprintf("/app/commerce/api/v1/users/%s/express-checkout/cart", uid), httpclient.RequestOpts{})
	if err != nil {
		return nil, err
	}
	preview := &CheckoutPreview{
		Provider:        provider,
		UserID:          uid,
		CartID:          firstNonEmpty(findString(payload, "cartId"), findString(payload, "id")),
		ItemCount:       findItemCount(payload),
		Total:           findMoney(payload),
		Reservation:     findReservation(payload),
		Payment:         findPaymentSelection(payload),
		Issues:          findIssues(payload),
		ReadyToFinalize: false,
		Raw:             payload,
	}
	preview.ReadyToFinalize = len(preview.Issues) == 0 && preview.ItemCount > 0 && preview.Reservation != nil && preview.Payment != nil
	return preview, nil
}

// Finalize places the Frisco express-checkout order, reads the order back when possible,
// and blocks redirect/3DS flows behind a structured ActionRequiredError by default.
func (c *FriscoClient) Finalize(s *session.Session, opts FinalizeOptions) (*FinalizeResult, error) {
	provider, uid, err := c.resolveFrisco(s, opts.Provider, opts.UserID)
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
	payload, err := c.requestMap(s, http.MethodPost, fmt.Sprintf("/app/commerce/api/v1/users/%s/express-checkout/cart/order", uid), httpclient.RequestOpts{})
	if err != nil {
		return nil, err
	}
	result := &FinalizeResult{
		Provider:    provider,
		UserID:      uid,
		Status:      FinalizeStatusUnknown,
		OrderID:     firstNonEmpty(findString(payload, "orderId"), findString(payload, "id")),
		Preview:     preview,
		Action:      detectPaymentAction(payload),
		APIResponse: payload,
	}
	if result.Action != nil {
		result.Status = FinalizeStatusRequiresAction
		if !opts.AllowActionRequired {
			return result, &ActionRequiredError{Action: result.Action, Result: result}
		}
		return result, nil
	}

	if result.OrderID != "" {
		readback, err := c.readbackOrder(s, uid, result.OrderID)
		if err != nil {
			return result, err
		}
		result.Readback = readback
		result.Status = classifyFinalizeStatus(payload, readback)
		return result, nil
	}

	result.Status = classifyFinalizeStatus(payload, nil)
	return result, nil
}

func (c *FriscoClient) readbackOrder(s *session.Session, uid, orderID string) (*OrderReadback, error) {
	order, err := c.requestMap(s, http.MethodGet, fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s", uid, orderID), httpclient.RequestOpts{})
	if err != nil {
		return nil, err
	}
	paymentsPayload, err := c.requestAny(s, http.MethodGet, fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s/payments", uid, orderID), httpclient.RequestOpts{})
	if err != nil {
		return nil, err
	}
	return &OrderReadback{
		OrderID:  orderID,
		Order:    order,
		Payments: toMapSlice(paymentsPayload),
	}, nil
}

func (c *FriscoClient) resolveFrisco(s *session.Session, provider, userID string) (string, string, error) {
	provider = session.NormalizeProvider(strings.TrimSpace(provider))
	if provider == "" {
		provider = session.ProviderForSession(s, session.ProviderFrisco)
	}
	if provider != friscoProvider {
		return "", "", &UnsupportedProviderError{Provider: provider, Supported: []string{friscoProvider}}
	}
	uid, err := session.RequireUserID(s, userID)
	if err != nil {
		return "", "", err
	}
	return provider, uid, nil
}

func (c *FriscoClient) requestAny(s *session.Session, method, path string, opts httpclient.RequestOpts) (any, error) {
	if c == nil || c.request == nil {
		c = NewFriscoClient()
	}
	if opts.Client == nil {
		opts.Client = c.httpClient
	}
	return c.request(s, method, path, opts)
}

func (c *FriscoClient) requestMap(s *session.Session, method, path string, opts httpclient.RequestOpts) (map[string]any, error) {
	payload, err := c.requestAny(s, method, path, opts)
	if err != nil {
		return nil, err
	}
	m, ok := payload.(map[string]any)
	if !ok {
		return nil, &MalformedResponseError{Operation: path, Message: fmt.Sprintf("expected JSON object, got %T", payload)}
	}
	return m, nil
}

func validateGuard(preview *CheckoutPreview, guard *FinalizeGuard) error {
	if preview == nil || guard == nil {
		return nil
	}
	if guard.ExpectedCartID != "" && preview.CartID != guard.ExpectedCartID {
		return &GuardMismatchError{Field: "cart_id", Want: guard.ExpectedCartID, Got: preview.CartID}
	}
	if guard.ExpectedItemCount != nil && preview.ItemCount != *guard.ExpectedItemCount {
		return &GuardMismatchError{Field: "item_count", Want: fmt.Sprintf("%d", *guard.ExpectedItemCount), Got: fmt.Sprintf("%d", preview.ItemCount)}
	}
	if guard.ExpectedTotal != nil {
		got := 0.0
		if preview.Total != nil {
			got = preview.Total.Amount
		}
		if math.Abs(got-*guard.ExpectedTotal) > 0.0001 {
			return &GuardMismatchError{Field: "total", Want: formatAmount(*guard.ExpectedTotal), Got: formatAmount(got)}
		}
	}
	return nil
}

func classifyFinalizeStatus(payload map[string]any, readback *OrderReadback) FinalizeStatus {
	for _, candidate := range []string{
		findString(payload, "paymentStatus"),
		findString(payload, "status"),
	} {
		switch strings.ToLower(strings.TrimSpace(candidate)) {
		case "requiresaction", "redirectrequired", "actionrequired", "3dsrequired":
			return FinalizeStatusRequiresAction
		case "pending", "processing", "waitingforpayment", "waiting-for-payment", "waitingforpaymentstatus":
			return FinalizeStatusPending
		case "placed", "paid", "completed", "confirmed", "success":
			return FinalizeStatusPlaced
		}
	}
	if readback != nil {
		for _, candidate := range []string{
			findString(readback.Order, "status"),
			findString(readback.Order, "orderStatus"),
		} {
			switch strings.ToLower(strings.TrimSpace(candidate)) {
			case "pending", "processing", "awaitingpayment":
				return FinalizeStatusPending
			case "placed", "confirmed", "paid", "completed":
				return FinalizeStatusPlaced
			}
		}
		if readback.OrderID != "" {
			return FinalizeStatusPlaced
		}
	}
	if firstNonEmpty(findString(payload, "orderId"), findString(payload, "id")) != "" {
		return FinalizeStatusPlaced
	}
	return FinalizeStatusUnknown
}

func detectPaymentAction(payload map[string]any) *PaymentAction {
	if payload == nil {
		return nil
	}
	if threeDS := findMapByKey(payload, map[string]struct{}{"threeDS": {}, "threeDs": {}, "3ds": {}}); threeDS != nil {
		url := firstNonEmpty(findString(threeDS, "acsUrl"), findString(threeDS, "url"), findString(threeDS, "redirectUrl"))
		if url != "" || len(threeDS) > 0 {
			return &PaymentAction{
				Kind:    PaymentActionKind3DS,
				URL:     url,
				Method:  firstNonEmpty(findString(threeDS, "method"), http.MethodPost),
				Message: firstNonEmpty(findString(payload, "message"), findString(payload, "status")),
				Payload: threeDS,
			}
		}
	}
	redirectURL := firstNonEmpty(
		findString(payload, "redirectUrl"),
		findString(payload, "redirectUri"),
		findString(payload, "paymentUrl"),
		findString(payload, "url"),
	)
	if looksLikeAbsoluteURL(redirectURL) {
		return &PaymentAction{
			Kind:    PaymentActionKindRedirect,
			URL:     redirectURL,
			Method:  firstNonEmpty(findString(payload, "method"), http.MethodGet),
			Message: firstNonEmpty(findString(payload, "message"), findString(payload, "status")),
			Payload: payload,
		}
	}
	payment := findMapByKey(payload, map[string]struct{}{"payment": {}, "onlinePayment": {}})
	if payment != nil {
		redirectURL = firstNonEmpty(findString(payment, "redirectUrl"), findString(payment, "redirectUri"), findString(payment, "paymentUrl"), findString(payment, "url"))
		if looksLikeAbsoluteURL(redirectURL) {
			return &PaymentAction{
				Kind:    PaymentActionKindRedirect,
				URL:     redirectURL,
				Method:  firstNonEmpty(findString(payment, "method"), http.MethodGet),
				Message: firstNonEmpty(findString(payment, "message"), findString(payload, "status")),
				Payload: payment,
			}
		}
	}
	return nil
}

func findMoney(payload map[string]any) *Money {
	amount, ok := findFloat(payload, "total")
	if !ok {
		for _, key := range []string{"amount", "grossTotal", "finalTotal", "priceWithDeliveryCostAfterVoucherPayment"} {
			if amount, ok = findFloat(payload, key); ok {
				break
			}
		}
	}
	if !ok {
		return nil
	}
	return &Money{Amount: amount, Currency: firstNonEmpty(findString(payload, "currency"), "PLN")}
}

func findReservation(payload map[string]any) *ReservationWindow {
	reservation := findMapByKey(payload, map[string]struct{}{"reservation": {}, "deliveryWindow": {}})
	if reservation == nil {
		startsAt := findString(payload, "startsAt")
		endsAt := findString(payload, "endsAt")
		if startsAt == "" && endsAt == "" {
			return nil
		}
		return &ReservationWindow{StartsAt: startsAt, EndsAt: endsAt, DeliveryMethod: findString(payload, "deliveryMethod"), Warehouse: findString(payload, "warehouse")}
	}
	return &ReservationWindow{
		StartsAt:       firstNonEmpty(findString(reservation, "startsAt"), findString(reservation, "start")),
		EndsAt:         firstNonEmpty(findString(reservation, "endsAt"), findString(reservation, "end")),
		DeliveryMethod: findString(reservation, "deliveryMethod"),
		Warehouse:      findString(reservation, "warehouse"),
	}
}

func findPaymentSelection(payload map[string]any) *PaymentSelection {
	payment := findMapByKey(payload, map[string]struct{}{"payment": {}, "selectedPayment": {}, "paymentMethod": {}})
	if payment == nil {
		method := findString(payload, "paymentMethod")
		channel := findString(payload, "onlinePaymentChannel")
		status := findString(payload, "paymentStatus")
		if method == "" && channel == "" && status == "" {
			return nil
		}
		return &PaymentSelection{Method: method, Channel: channel, Status: status}
	}
	return &PaymentSelection{
		Method:  firstNonEmpty(findString(payment, "method"), findString(payment, "paymentMethod"), findString(payment, "type")),
		Channel: firstNonEmpty(findString(payment, "channel"), findString(payment, "channelName"), findString(payment, "onlinePaymentChannel")),
		Status:  firstNonEmpty(findString(payment, "status"), findString(payload, "paymentStatus")),
	}
}

func findIssues(payload map[string]any) []CheckoutIssue {
	var issues []CheckoutIssue
	appendIssues := func(code, message string) {
		code = strings.TrimSpace(code)
		message = strings.TrimSpace(message)
		if code == "" && message == "" {
			return
		}
		issues = append(issues, CheckoutIssue{Code: code, Message: message})
	}
	if rawErrors, ok := payload["errors"].([]any); ok {
		for _, item := range rawErrors {
			if m, ok := item.(map[string]any); ok {
				appendIssues(findString(m, "code"), firstNonEmpty(findString(m, "message"), findString(m, "reason")))
			}
		}
	}
	for _, key := range []string{"error", "message", "reason"} {
		if msg := findString(payload, key); msg != "" && len(issues) == 0 {
			appendIssues(findString(payload, "code"), msg)
		}
	}
	return issues
}

func findItemCount(payload map[string]any) int {
	for _, key := range []string{"items", "products", "cartProducts", "lineItems"} {
		if arr, ok := payload[key].([]any); ok {
			return len(arr)
		}
	}
	if n, ok := findInt(payload, "itemCount"); ok {
		return n
	}
	return 0
}

func findMapByKey(v any, candidates map[string]struct{}) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			if _, ok := candidates[k]; ok {
				if m, ok := child.(map[string]any); ok {
					return m
				}
			}
			if found := findMapByKey(child, candidates); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range t {
			if found := findMapByKey(child, candidates); found != nil {
				return found
			}
		}
	}
	return nil
}

func findString(v any, key string) string {
	switch t := v.(type) {
	case map[string]any:
		if raw, ok := t[key]; ok {
			return strings.TrimSpace(fmt.Sprint(raw))
		}
		for _, child := range t {
			if out := findString(child, key); out != "" {
				return out
			}
		}
	case []any:
		for _, child := range t {
			if out := findString(child, key); out != "" {
				return out
			}
		}
	}
	return ""
}

func findFloat(v any, key string) (float64, bool) {
	switch t := v.(type) {
	case map[string]any:
		if raw, ok := t[key]; ok {
			return asFloat(raw)
		}
		for _, child := range t {
			if out, ok := findFloat(child, key); ok {
				return out, true
			}
		}
	case []any:
		for _, child := range t {
			if out, ok := findFloat(child, key); ok {
				return out, true
			}
		}
	}
	return 0, false
}

func findInt(v any, key string) (int, bool) {
	f, ok := findFloat(v, key)
	if !ok {
		return 0, false
	}
	return int(f), true
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}

func toMapSlice(v any) []map[string]any {
	switch t := v.(type) {
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		if items, ok := t["items"].([]any); ok {
			return toMapSlice(items)
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func looksLikeAbsoluteURL(v string) bool {
	v = strings.TrimSpace(v)
	return strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")
}

func formatAmount(v float64) string {
	return fmt.Sprintf("%.2f", math.Round(v*100)/100)
}

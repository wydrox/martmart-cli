package checkout

import "net/url"

// Money captures a typed monetary amount extracted from checkout payloads.
type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency,omitempty"`
}

// ReservationWindow summarizes the reserved delivery window used for checkout.
type ReservationWindow struct {
	StartsAt       string `json:"starts_at,omitempty"`
	EndsAt         string `json:"ends_at,omitempty"`
	DeliveryMethod string `json:"delivery_method,omitempty"`
	Warehouse      string `json:"warehouse,omitempty"`
}

// PaymentSelection describes the currently selected payment option in checkout.
type PaymentSelection struct {
	Method  string `json:"method,omitempty"`
	Channel string `json:"channel,omitempty"`
	Status  string `json:"status,omitempty"`
}

// CheckoutIssue is a normalized validation or business issue returned by checkout.
type CheckoutIssue struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// CheckoutPreview is the normalized Frisco express-checkout cart snapshot.
type CheckoutPreview struct {
	Provider        string             `json:"provider"`
	UserID          string             `json:"user_id"`
	CartID          string             `json:"cart_id,omitempty"`
	ItemCount       int                `json:"item_count"`
	Total           *Money             `json:"total,omitempty"`
	Reservation     *ReservationWindow `json:"reservation,omitempty"`
	Payment         *PaymentSelection  `json:"payment,omitempty"`
	ReadyToFinalize bool               `json:"ready_to_finalize"`
	Issues          []CheckoutIssue    `json:"issues,omitempty"`
	Raw             map[string]any     `json:"raw,omitempty"`
}

// PreviewOptions controls checkout preview fetching.
type PreviewOptions struct {
	Provider string `json:"provider,omitempty"`
	UserID   string `json:"user_id,omitempty"`
}

// FinalizeGuard allows callers to verify the preview state before placing an order.
type FinalizeGuard struct {
	ExpectedCartID    string   `json:"expected_cart_id,omitempty"`
	ExpectedItemCount *int     `json:"expected_item_count,omitempty"`
	ExpectedTotal     *float64 `json:"expected_total,omitempty"`
}

// PaymentActionKind describes what kind of user/browser step is still required.
type PaymentActionKind string

const (
	PaymentActionKindRedirect PaymentActionKind = "redirect"
	PaymentActionKind3DS      PaymentActionKind = "3ds"
)

// PaymentAction captures required post-finalize interaction such as redirect/3DS.
type PaymentAction struct {
	Kind    PaymentActionKind `json:"kind"`
	URL     string            `json:"url,omitempty"`
	Method  string            `json:"method,omitempty"`
	Message string            `json:"message,omitempty"`
	Payload map[string]any    `json:"payload,omitempty"`
}

// IsRedirect reports whether the action URL is an absolute web URL.
func (a *PaymentAction) IsRedirect() bool {
	if a == nil || a.URL == "" {
		return false
	}
	u, err := url.Parse(a.URL)
	return err == nil && u.IsAbs()
}

// FinalizeStatus is the normalized high-level outcome of a finalize attempt.
type FinalizeStatus string

const (
	FinalizeStatusPlaced         FinalizeStatus = "placed"
	FinalizeStatusRequiresAction FinalizeStatus = "requires_action"
	FinalizeStatusPending        FinalizeStatus = "pending"
	FinalizeStatusUnknown        FinalizeStatus = "unknown"
)

// OrderReadback contains post-finalize GET responses when an order ID is known.
type OrderReadback struct {
	OrderID  string           `json:"order_id,omitempty"`
	Order    map[string]any   `json:"order,omitempty"`
	Payments []map[string]any `json:"payments,omitempty"`
}

// FinalizeOptions controls Frisco order finalization.
type FinalizeOptions struct {
	Provider            string         `json:"provider,omitempty"`
	UserID              string         `json:"user_id,omitempty"`
	Guard               *FinalizeGuard `json:"guard,omitempty"`
	AllowActionRequired bool           `json:"allow_action_required,omitempty"`
}

// FinalizeResult captures the POST /order response plus optional readback.
type FinalizeResult struct {
	Provider    string           `json:"provider"`
	UserID      string           `json:"user_id"`
	Status      FinalizeStatus   `json:"status"`
	OrderID     string           `json:"order_id,omitempty"`
	Preview     *CheckoutPreview `json:"preview,omitempty"`
	Action      *PaymentAction   `json:"action,omitempty"`
	Readback    *OrderReadback   `json:"readback,omitempty"`
	APIResponse map[string]any   `json:"api_response,omitempty"`
}

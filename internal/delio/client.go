package delio

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
)

// Coordinates represents a delivery/search point required by several Delio APIs.
type Coordinates struct {
	Lat  float64 `json:"lat"`
	Long float64 `json:"long"`
}

func (c *Coordinates) valid() bool {
	if c == nil {
		return false
	}
	return !(math.Abs(c.Lat) < 1e-9 && math.Abs(c.Long) < 1e-9)
}

func (c *Coordinates) toMap() map[string]any {
	return map[string]any{"lat": c.Lat, "long": c.Long}
}

func graphqlHeaders() map[string]string {
	return map[string]string{
		"Accept":           "*/*",
		"Content-Type":     "application/json",
		"X-Platform":       "web",
		"X-Api-Version":    "4.0",
		"X-App-Version":    "7.32.6",
		"X-Csrf-Protected": "",
	}
}

func graphqlRequest(s *session.Session, path string, payload map[string]any) (any, error) {
	return httpclient.RequestJSON(s, "POST", path, httpclient.RequestOpts{
		Data:         payload,
		DataFormat:   httpclient.FormatJSON,
		ExtraHeaders: graphqlHeaders(),
	})
}

type UpdateCurrentCartError struct {
	Message string
	Payload any
	Errors  []any
}

func (e *UpdateCurrentCartError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return "updateCurrentCart validation failed"
}

func IsUpdateCurrentCartBusinessError(err error) bool {
	var target *UpdateCurrentCartError
	return errors.As(err, &target)
}

func unwrapGraphQL(payload any) (map[string]any, error) {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil, errors.New("unexpected Delio response shape")
	}
	if rawErrors, ok := root["errors"].([]any); ok && len(rawErrors) > 0 {
		b, _ := json.Marshal(rawErrors)
		return nil, fmt.Errorf("graphql errors: %s", string(b))
	}
	data, ok := root["data"].(map[string]any)
	if !ok {
		return nil, errors.New("missing Delio response data")
	}
	return data, nil
}

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	default:
		return 0, false
	}
}

func mapField(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	out, _ := m[key].(map[string]any)
	return out
}

func listField(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	out, _ := m[key].([]any)
	return out
}

// CurrentCart fetches the current Delio cart for the active session.
func CurrentCart(s *session.Session) (any, error) {
	return graphqlRequest(s, "/api/proxy/delio", map[string]any{
		"operationName": "CurrentCart",
		"variables":     map[string]any{},
		"query":         currentCartQuery,
	})
}

// CustomerShippingAddress fetches the default shipping address from the onebrand API.
func CustomerShippingAddress(s *session.Session) (any, error) {
	return graphqlRequest(s, "/api/proxy/onebrand", map[string]any{
		"operationName": "CustomerShippingAddress",
		"variables":     map[string]any{},
		"query":         customerShippingAddressQuery,
	})
}

// ExtractCurrentCart returns the data.currentCart object.
func ExtractCurrentCart(payload any) (map[string]any, error) {
	data, err := unwrapGraphQL(payload)
	if err != nil {
		return nil, err
	}
	cart := mapField(data, "currentCart")
	if cart == nil {
		return nil, errors.New("missing currentCart in Delio response")
	}
	return cart, nil
}

// ExtractDefaultShippingAddress returns data.defaultShippingAddress.
func ExtractDefaultShippingAddress(payload any) (map[string]any, error) {
	data, err := unwrapGraphQL(payload)
	if err != nil {
		return nil, err
	}
	addr := mapField(data, "defaultShippingAddress")
	if addr == nil {
		return nil, errors.New("missing defaultShippingAddress in Delio response")
	}
	return addr, nil
}

func coordinatesFromMap(m map[string]any) *Coordinates {
	if m == nil {
		return nil
	}
	lat, okLat := asFloat(m["lat"])
	long, okLong := asFloat(m["long"])
	if !okLat || !okLong {
		return nil
	}
	coords := &Coordinates{Lat: lat, Long: long}
	if !coords.valid() {
		return nil
	}
	return coords
}

// ResolveCoordinates uses explicit coordinates when given, otherwise attempts
// to infer them from the current cart shippingAddress or default shipping address.
func ResolveCoordinates(s *session.Session, explicit *Coordinates) (*Coordinates, error) {
	if explicit != nil && explicit.valid() {
		return explicit, nil
	}
	if payload, err := CurrentCart(s); err == nil {
		if cart, err := ExtractCurrentCart(payload); err == nil {
			if coords := coordinatesFromMap(mapField(cart, "shippingAddress")); coords != nil {
				return coords, nil
			}
		}
	}
	if payload, err := CustomerShippingAddress(s); err == nil {
		if addr, err := ExtractDefaultShippingAddress(payload); err == nil {
			if coords := coordinatesFromMap(addr); coords != nil {
				return coords, nil
			}
		}
	}
	return nil, errors.New("missing Delio coordinates: pass --lat and --long or import a session with saved shipping address")
}

// SearchProducts performs Delio ProductSearch.
func SearchProducts(s *session.Session, query string, limit, offset int, coords *Coordinates) (any, error) {
	resolved, err := ResolveCoordinates(s, coords)
	if err != nil {
		return nil, err
	}
	return graphqlRequest(s, "/api/proxy/delio", map[string]any{
		"operationName": "ProductSearch",
		"variables": map[string]any{
			"query":       query,
			"limit":       limit,
			"offset":      offset,
			"coordinates": resolved.toMap(),
		},
		"query": productSearchQuery,
	})
}

// ExtractProductSearch returns data.productSearch.
func ExtractProductSearch(payload any) (map[string]any, error) {
	data, err := unwrapGraphQL(payload)
	if err != nil {
		return nil, err
	}
	res := mapField(data, "productSearch")
	if res == nil {
		return nil, errors.New("missing productSearch in Delio response")
	}
	return res, nil
}

// GetProduct loads a single Delio product by slug or SKU.
func GetProduct(s *session.Session, slug, sku string, coords *Coordinates) (any, error) {
	resolved, err := ResolveCoordinates(s, coords)
	if err != nil {
		return nil, err
	}
	variables := map[string]any{"coordinates": resolved.toMap()}
	if strings.TrimSpace(slug) != "" {
		variables["slug"] = strings.TrimSpace(slug)
	}
	if strings.TrimSpace(sku) != "" {
		variables["sku"] = strings.TrimSpace(sku)
	}
	return graphqlRequest(s, "/api/proxy/delio", map[string]any{
		"operationName": "Product",
		"variables":     variables,
		"query":         productQuery,
	})
}

// ExtractProduct returns data.product.
func ExtractProduct(payload any) (map[string]any, error) {
	data, err := unwrapGraphQL(payload)
	if err != nil {
		return nil, err
	}
	res := mapField(data, "product")
	if res == nil {
		return nil, errors.New("missing product in Delio response")
	}
	return res, nil
}

// UpdateCurrentCart applies cart actions and returns the raw GraphQL payload.
func UpdateCurrentCart(s *session.Session, cartID string, actions []map[string]any) (any, error) {
	if strings.TrimSpace(cartID) == "" {
		return nil, errors.New("missing cartId")
	}
	return graphqlRequest(s, "/api/proxy/delio", map[string]any{
		"operationName": "UpdateCurrentCart",
		"variables": map[string]any{
			"cartId":  cartID,
			"actions": actions,
		},
		"query": updateCurrentCartMutation,
	})
}

func ExtractUpdatedCart(payload any) (map[string]any, error) {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil, &UpdateCurrentCartError{Message: "unexpected Delio response shape", Payload: payload}
	}
	if rawErrors, ok := root["errors"].([]any); ok && len(rawErrors) > 0 {
		b, _ := json.Marshal(rawErrors)
		return nil, &UpdateCurrentCartError{Message: fmt.Sprintf("graphql errors: %s", string(b)), Payload: payload, Errors: rawErrors}
	}
	data, ok := root["data"].(map[string]any)
	if !ok {
		return nil, &UpdateCurrentCartError{Message: "missing Delio response data", Payload: payload}
	}
	updated, ok := data["updateCart"].(map[string]any)
	if !ok || updated == nil {
		return nil, &UpdateCurrentCartError{Message: "missing updateCart in Delio response", Payload: payload}
	}
	return updated, nil
}

// DeliveryScheduleSlots fetches delivery slots for the given or inferred coordinates.
func DeliveryScheduleSlots(s *session.Session, coords *Coordinates) (any, error) {
	resolved, err := ResolveCoordinates(s, coords)
	if err != nil {
		return nil, err
	}
	return graphqlRequest(s, "/api/proxy/delio", map[string]any{
		"operationName": "DeliveryScheduleSlots",
		"variables": map[string]any{
			"coordinates": resolved.toMap(),
		},
		"query": deliveryScheduleSlotsQuery,
	})
}

// ExtractDeliveryScheduleSlots returns data.deliveryScheduleSlots.
func ExtractDeliveryScheduleSlots(payload any) ([]any, error) {
	data, err := unwrapGraphQL(payload)
	if err != nil {
		return nil, err
	}
	slots := listField(data, "deliveryScheduleSlots")
	if slots == nil {
		return nil, errors.New("missing deliveryScheduleSlots in Delio response")
	}
	return slots, nil
}

package upmenu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

var ErrProductRequiresConfiguration = errors.New("upmenu product requires configuration")

var (
	reSiteID       = regexp.MustCompile(`com\.upmenu\.siteId\s*=\s*'([^']+)'`)
	reRestaurantID = regexp.MustCompile(`com\.upmenu\.restaurantId\s*=\s*'([^']+)'`)
)

type Client struct {
	cfg   Config
	http  *http.Client
	state State
}

func NewClient(cfg Config) (*Client, error) {
	cfg = withDefaults(cfg)
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.SiteID) == "" || strings.TrimSpace(cfg.RestaurantID) == "" {
		return nil, errors.New("missing upmenu base url, site id, or restaurant id")
	}
	if cfg.HTTPClient == nil {
		jar, _ := cookiejar.New(nil)
		cfg.HTTPClient = &http.Client{Jar: jar}
	} else if cfg.HTTPClient.Jar == nil {
		jar, _ := cookiejar.New(nil)
		cfg.HTTPClient.Jar = jar
	}
	return &Client{cfg: cfg, http: cfg.HTTPClient}, nil
}

func NewClientFromRestaurantURL(ctx context.Context, restaurantURL string, cfg Config) (*Client, error) {
	trimmed := strings.TrimSpace(restaurantURL)
	if trimmed == "" {
		return NewClient(cfg)
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("invalid restaurant url")
	}
	cfg.BaseURL = (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
	clientForFetch := cfg.HTTPClient
	if clientForFetch == nil {
		clientForFetch = http.DefaultClient
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trimmed, nil)
	if err != nil {
		return nil, err
	}
	resp, err := clientForFetch.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, httpError(resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	matchSite := reSiteID.FindStringSubmatch(string(body))
	matchRestaurant := reRestaurantID.FindStringSubmatch(string(body))
	if len(matchSite) < 2 || len(matchRestaurant) < 2 {
		return nil, errors.New("failed to resolve UpMenu site_id/restaurant_id from restaurant page")
	}
	cfg.SiteID = strings.TrimSpace(matchSite[1])
	cfg.RestaurantID = strings.TrimSpace(matchRestaurant[1])
	return NewClient(cfg)
}

func withDefaults(cfg Config) Config {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if strings.TrimSpace(cfg.SiteID) == "" {
		cfg.SiteID = DefaultSiteID
	}
	if strings.TrimSpace(cfg.RestaurantID) == "" {
		cfg.RestaurantID = DefaultRestaurantID
	}
	if strings.TrimSpace(cfg.Language) == "" {
		cfg.Language = DefaultLanguage
	}
	if strings.TrimSpace(cfg.DeliveryType) == "" {
		cfg.DeliveryType = DefaultDeliveryType
	}
	if strings.TrimSpace(cfg.CartLocation) == "" {
		cfg.CartLocation = DefaultCartLocation
	}
	if strings.TrimSpace(cfg.PaymentMethod) == "" {
		cfg.PaymentMethod = DefaultPaymentMethod
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		cfg.UserAgent = "martmart-cli/upmenu"
	}
	return cfg
}

func (c *Client) State() State { return c.state }

func (c *Client) SetState(state State) { c.state = state }

func (c *Client) RestaurantInfo(ctx context.Context) (*RestaurantInfo, error) {
	var raw map[string]any
	if err := c.getJSON(ctx, "/restapi/restaurant/"+c.cfg.RestaurantID, &raw); err != nil {
		return nil, err
	}
	info := &RestaurantInfo{
		ID:             stringValue(raw["id"]),
		Name:           stringValue(raw["name"]),
		URL:            stringValue(raw["url"]),
		Street:         stringValue(raw["street"]),
		PostalCode:     stringValue(raw["postalCode"]),
		City:           stringValue(raw["city"]),
		Phone:          firstNonEmpty(stringValue(raw["phone"]), stringValue(raw["mobilePhone"])),
		Email:          stringValue(raw["email"]),
		Currency:       stringValue(raw["currency"]),
		Delivery:       boolValue(raw["delivery"]),
		Takeaway:       boolValue(raw["takeaway"]),
		OnSite:         boolValue(raw["onsite"]),
		OnlineOrdering: boolValue(raw["onlineOrderingEnabled"]),
		OpenNow:        boolValue(raw["openNow"]),
	}
	info.MinimumOrderPrice = firstFloatPtr(raw["minOrderPriceInRestaurant"], raw["minMinOrderPrice"])
	info.MinimumDeliveryCost = floatPtr(raw["minDeliveryCost"])
	info.MaximumDeliveryCost = floatPtr(raw["maxDeliveryCost"])
	return info, nil
}

func (c *Client) MenuHTML(ctx context.Context) (string, error) {
	return c.getText(ctx, "/api/v1/menu/"+c.cfg.SiteID+"/"+c.cfg.RestaurantID)
}

func (c *Client) Menu(ctx context.Context) (*Menu, error) {
	html, err := c.MenuHTML(ctx)
	if err != nil {
		return nil, err
	}
	return ParseMenuHTML(html)
}

func (c *Client) MenuJSON(ctx context.Context) (*Menu, error) {
	var raw []map[string]any
	if err := c.getJSON(ctx, "/restapi/menu/"+c.cfg.SiteID+"/"+c.cfg.RestaurantID, &raw); err != nil {
		return nil, err
	}
	menu := &Menu{}
	for _, categoryMap := range raw {
		category := MenuCategory{
			ID:          stringValue(categoryMap["id"]),
			Name:        stringValue(categoryMap["name"]),
			Description: stringValue(categoryMap["description"]),
		}
		for _, productMap := range mapSlice(categoryMap["products"]) {
			product := MenuProduct{
				ID:           stringValue(productMap["id"]),
				CategoryID:   category.ID,
				CategoryName: category.Name,
				Name:         stringValue(productMap["name"]),
				Description:  stringValue(productMap["description"]),
				ImageURL:     imageURL(productMap["image"]),
				BasePrice:    floatPtr(productMap["price"]),
			}
			if product.ProductPriceID == "" {
				product.ProductPriceID = stringValue(productMap["productPriceId"])
			}
			for _, priceMap := range mapSlice(productMap["prices"]) {
				variant := Variant{ID: stringValue(priceMap["id"]), Name: stringValue(priceMap["type"]), Price: floatPtr(priceMap["price"])}
				product.AvailableOptions = append(product.AvailableOptions, variant)
				if product.ProductPriceID == "" {
					product.ProductPriceID = variant.ID
				}
				if product.BasePrice == nil && variant.Price != nil {
					product.BasePrice = variant.Price
				}
			}
			category.Products = append(category.Products, product)
			menu.Products = append(menu.Products, product)
		}
		menu.Categories = append(menu.Categories, category)
	}
	return menu, nil
}

func (c *Client) ShowCart(ctx context.Context) (*Cart, error) {
	body := CartRequest{
		CartID:        c.state.CartID,
		CustomerID:    nil,
		DeliveryType:  c.cfg.DeliveryType,
		CartLocation:  c.cfg.CartLocation,
		PaymentMethod: c.cfg.PaymentMethod,
	}
	var raw struct {
		Cart map[string]any `json:"cart"`
	}
	if err := c.postJSON(ctx, "/restapi/cart/"+c.cfg.SiteID+"/"+c.cfg.RestaurantID, body, &raw); err != nil {
		return nil, err
	}
	cart := normalizeCart(raw.Cart)
	if cart.ID != "" {
		c.state.CartID = cart.ID
	}
	return cart, nil
}

func (c *Client) RequiresConfiguration(ctx context.Context, productPriceID string) (bool, error) {
	var res RequiredResult
	if err := c.getJSON(ctx, "/restapi/buyingFlow/required/"+c.cfg.RestaurantID+"/"+productPriceID, &res); err != nil {
		return false, err
	}
	return res.Required, nil
}

func (c *Client) StartBuyingFlow(ctx context.Context, productPriceID string) (*BuyingFlow, error) {
	path := fmt.Sprintf("/restapi/buyingFlow/startByProductPrice/%s/%s/%s?cartId=%s&buyingFlowId=undefined", c.cfg.SiteID, c.cfg.RestaurantID, productPriceID, url.QueryEscape(c.state.CartID))
	var raw map[string]any
	if err := c.postJSON(ctx, path, nil, &raw); err != nil {
		return nil, err
	}
	return normalizeBuyingFlow(raw), nil
}

func (c *Client) FinishBuyingFlow(ctx context.Context, flow *BuyingFlow) (*BuyingFlow, error) {
	if flow == nil || flow.Raw == nil {
		return nil, errors.New("missing buying flow payload")
	}
	var raw map[string]any
	if err := c.postJSON(ctx, "/restapi/buyingFlow/finish/"+c.cfg.SiteID+"/"+c.cfg.RestaurantID, flow.Raw, &raw); err != nil {
		return nil, err
	}
	result := normalizeBuyingFlow(raw)
	if result.CartID != "" {
		c.state.CartID = result.CartID
	}
	return result, nil
}

func (c *Client) AddSimple(ctx context.Context, productPriceID string, quantity int) (*Cart, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be > 0")
	}
	required, err := c.RequiresConfiguration(ctx, productPriceID)
	if err != nil {
		return nil, err
	}
	if required {
		return nil, ErrProductRequiresConfiguration
	}
	for i := 0; i < quantity; i++ {
		flow, err := c.StartBuyingFlow(ctx, productPriceID)
		if err != nil {
			return nil, err
		}
		if len(flow.Steps) > 0 {
			return nil, ErrProductRequiresConfiguration
		}
		if _, err := c.FinishBuyingFlow(ctx, flow); err != nil {
			return nil, err
		}
	}
	return c.ShowCart(ctx)
}

func (c *Client) getJSON(ctx context.Context, path string, dest any) error {
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return httpError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func (c *Client) getText(ctx context.Context, path string) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", httpError(resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) postJSON(ctx context.Context, path string, body any, dest any) error {
	resp, err := c.do(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return httpError(resp)
	}
	if dest == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	rawURL := strings.TrimRight(c.cfg.BaseURL, "/") + path
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set(HeaderAccept, "application/json, text/html, */*")
	req.Header.Set(HeaderUserAgent, c.cfg.UserAgent)
	req.Header.Set(HeaderXRequestedWith, "XMLHttpRequest")
	if body != nil {
		req.Header.Set(HeaderContentType, ContentTypeJSON)
	}
	return c.http.Do(req)
}

func httpError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("upmenu http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func normalizeCart(raw map[string]any) *Cart {
	cart := &Cart{
		ID:             stringValue(raw["id"]),
		DeliveryType:   stringValue(raw["deliveryType"]),
		DeliveryStatus: stringValue(raw["deliveryStatus"]),
		TotalCost:      floatPtr(raw["totalCost"]),
		ProductsCost:   floatPtr(raw["productsCost"]),
		DeliveryCost:   floatPtr(raw["deliveryCost"]),
		ItemsSize:      intValue(raw["itemsSize"]),
		Messages:       stringSlice(raw["messages"]),
		Errors:         stringSlice(raw["errors"]),
	}
	for _, item := range mapSlice(raw["items"]) {
		cart.Items = append(cart.Items, CartItem{
			ID:             stringValue(item["id"]),
			Name:           stringValue(item["name"]),
			ProductID:      stringValue(item["productId"]),
			ProductPriceID: stringValue(item["productPriceId"]),
			Quantity:       floatValue(item["quantity"]),
			Price:          floatPtr(item["price"]),
		})
	}
	return cart
}

func normalizeBuyingFlow(raw map[string]any) *BuyingFlow {
	flow := &BuyingFlow{
		BuyingFlowID:   stringValue(raw["buyingFlowId"]),
		RestaurantID:   stringValue(raw["restaurantId"]),
		CartID:         stringValue(raw["cartId"]),
		ProductPriceID: stringValue(raw["productPriceId"]),
		ProductName:    stringValue(raw["productName"]),
		ProductPrice:   floatPtr(raw["productPrice"]),
		Quantity:       intValue(raw["quantity"]),
		Errors:         stringSlice(raw["errors"]),
		TotalPrice:     floatPtr(raw["totalPrice"]),
		Raw:            raw,
	}
	for _, step := range mapSlice(raw["steps"]) {
		flow.Steps = append(flow.Steps, BuyingFlowStep{
			ID:   stringValue(step["id"]),
			Name: stringValue(step["name"]),
			Done: boolValue(step["done"]),
		})
	}
	return flow
}

func mapSlice(v any) []map[string]any {
	items, _ := v.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func stringSlice(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := stringValue(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func imageURL(v any) string {
	if m, ok := v.(map[string]any); ok {
		return firstNonEmpty(stringValue(m["themeUrl"]), stringValue(m["url"]), stringValue(m["medium_url"]))
	}
	return ""
}

func stringValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func intValue(v any) int {
	return int(floatValue(v))
}

func boolValue(v any) bool {
	b, _ := v.(bool)
	return b
}

func floatValue(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

func floatPtr(v any) *float64 {
	f := floatValue(v)
	switch v.(type) {
	case float64, float32, int, int64:
		return &f
	default:
		return nil
	}
}

func firstFloatPtr(values ...any) *float64 {
	for _, v := range values {
		if f := floatPtr(v); f != nil {
			return f
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

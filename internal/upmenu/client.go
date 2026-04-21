package upmenu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	// DefaultBaseURL points at the public UpMenu site used for the Dobra Buła MVP.
	DefaultBaseURL = "https://dobrabula.orderwebsite.com"
	// DefaultRestaurantPath points at the default Dobra Buła restaurant page.
	DefaultRestaurantPath = "/dobra-bula-solidarnosci-warszawa"
	defaultLanguage       = "pl"
	defaultDeviceType     = "DESKTOP"
	defaultOrderSource    = "WWW"
)

var (
	reSiteID       = regexp.MustCompile(`com\.upmenu\.siteId\s*=\s*'([^']+)'`)
	reRestaurantID = regexp.MustCompile(`com\.upmenu\.restaurantId\s*=\s*'([^']+)'`)
)

// Config configures a public UpMenu restaurant client.
type Config struct {
	RestaurantURL  string
	BaseURL        string
	RestaurantPath string
	SiteID         string
	RestaurantID   string
	Language       string
	DeviceType     string
	OrderSource    string
	HTTPClient     *http.Client
}

// Metadata is the resolved UpMenu site/restaurant identity.
type Metadata struct {
	SiteID        string `json:"site_id"`
	RestaurantID  string `json:"restaurant_id"`
	RestaurantURL string `json:"restaurant_url"`
}

// Client talks to the public UpMenu storefront APIs used by the Dobra Buła MVP.
type Client struct {
	cfg      Config
	metadata *Metadata
}

// DefaultConfig returns the Dobra Buła MVP defaults.
func DefaultConfig() Config {
	return Config{
		RestaurantURL:  DefaultBaseURL + DefaultRestaurantPath,
		BaseURL:        DefaultBaseURL,
		RestaurantPath: DefaultRestaurantPath,
		Language:       defaultLanguage,
		DeviceType:     defaultDeviceType,
		OrderSource:    defaultOrderSource,
	}
}

// NewClient creates a new UpMenu storefront client.
func NewClient(cfg Config) *Client {
	cfg = normalizeConfig(cfg)
	return &Client{cfg: cfg}
}

func normalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	if strings.TrimSpace(cfg.RestaurantURL) == "" {
		cfg.RestaurantURL = defaults.RestaurantURL
	}
	if baseURL, path, ok := splitRestaurantURL(cfg.RestaurantURL); ok {
		cfg.BaseURL = baseURL
		cfg.RestaurantPath = path
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaults.BaseURL
	}
	if strings.TrimSpace(cfg.RestaurantPath) == "" {
		cfg.RestaurantPath = defaults.RestaurantPath
	}
	if strings.TrimSpace(cfg.Language) == "" {
		cfg.Language = defaults.Language
	}
	if strings.TrimSpace(cfg.DeviceType) == "" {
		cfg.DeviceType = defaults.DeviceType
	}
	if strings.TrimSpace(cfg.OrderSource) == "" {
		cfg.OrderSource = defaults.OrderSource
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return cfg
}

func splitRestaurantURL(raw string) (string, string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", "", false
	}
	path := strings.TrimSpace(u.EscapedPath())
	if path == "" {
		path = "/"
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host}).String(), path, true
}

// Metadata resolves or returns cached UpMenu site/restaurant identifiers.
func (c *Client) Metadata(ctx context.Context) (*Metadata, error) {
	if c.metadata != nil {
		copy := *c.metadata
		return &copy, nil
	}
	if strings.TrimSpace(c.cfg.SiteID) != "" && strings.TrimSpace(c.cfg.RestaurantID) != "" {
		c.metadata = &Metadata{
			SiteID:        strings.TrimSpace(c.cfg.SiteID),
			RestaurantID:  strings.TrimSpace(c.cfg.RestaurantID),
			RestaurantURL: c.restaurantURL(),
		}
		copy := *c.metadata
		return &copy, nil
	}
	page, err := c.fetchRestaurantPage(ctx)
	if err != nil {
		return nil, err
	}
	matchSite := reSiteID.FindStringSubmatch(page)
	matchRestaurant := reRestaurantID.FindStringSubmatch(page)
	if len(matchSite) < 2 || len(matchRestaurant) < 2 {
		return nil, errors.New("failed to resolve UpMenu site_id/restaurant_id from restaurant page")
	}
	c.metadata = &Metadata{
		SiteID:        strings.TrimSpace(matchSite[1]),
		RestaurantID:  strings.TrimSpace(matchRestaurant[1]),
		RestaurantURL: c.restaurantURL(),
	}
	copy := *c.metadata
	return &copy, nil
}

func (c *Client) restaurantURL() string {
	return strings.TrimRight(c.cfg.BaseURL, "/") + ensureLeadingSlash(c.cfg.RestaurantPath)
}

func ensureLeadingSlash(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func (c *Client) fetchRestaurantPage(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.restaurantURL(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("UpMenu restaurant page returned HTTP %d", resp.StatusCode)
	}
	return string(body), nil
}

// RestaurantInfo fetches the public restaurant details payload.
func (c *Client) RestaurantInfo(ctx context.Context) (any, error) {
	meta, err := c.Metadata(ctx)
	if err != nil {
		return nil, err
	}
	return c.doJSON(ctx, http.MethodGet, "/restapi/restaurant/"+meta.RestaurantID, nil, false)
}

// Menu fetches the public CMS v2 menu payload.
func (c *Client) Menu(ctx context.Context) (any, error) {
	meta, err := c.Metadata(ctx)
	if err != nil {
		return nil, err
	}
	return c.doJSON(ctx, http.MethodGet, "/api/v2/menu/"+meta.SiteID+"/"+meta.RestaurantID, nil, false)
}

// CartShow fetches the current UpMenu cart snapshot.
func (c *Client) CartShow(ctx context.Context, cartID, customerID string) (any, error) {
	meta, err := c.Metadata(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"cartId":              nullableTrimmed(cartID),
		"latitude":            nil,
		"longitude":           nil,
		"customerId":          nullableTrimmed(customerID),
		"deliveryType":        nil,
		"email":               nil,
		"phone":               nil,
		"deliveryDate":        nil,
		"deliveryTime":        nil,
		"cartLocation":        "MENU",
		"paymentMethod":       nil,
		"invoiceTaxId":        nil,
		"city":                nil,
		"street":              nil,
		"streetNumber":        nil,
		"createAccount":       nil,
		"paymentProviderType": nil,
	}
	return c.doJSON(ctx, http.MethodPost, "/restapi/cart/"+meta.SiteID+"/"+meta.RestaurantID, body, true)
}

// CartAdd adds a product price to the cart.
func (c *Client) CartAdd(ctx context.Context, cartID, productID, customerID string) (any, error) {
	meta, err := c.Metadata(ctx)
	if err != nil {
		return nil, err
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, errors.New("product_id is required")
	}
	body := map[string]any{
		"cartId":     nullableTrimmed(cartID),
		"productId":  productID,
		"customerId": nullableTrimmed(customerID),
	}
	return c.doJSON(ctx, http.MethodPost, "/restapi/cart/items/add/"+meta.RestaurantID, body, true)
}

func nullableTrimmed(v string) any {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return v
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, includeOrderHeaders bool) (any, error) {
	fullURL := strings.TrimRight(c.cfg.BaseURL, "/") + ensureLeadingSlash(path)
	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(raw)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("language", c.cfg.Language)
	req.Header.Set("deviceType", c.cfg.DeviceType)
	if includeOrderHeaders {
		req.Header.Set("orderSource", c.cfg.OrderSource)
	}
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("UpMenu request failed: %s %s returned HTTP %d", method, ensureLeadingSlash(path), resp.StatusCode)
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"status": resp.StatusCode, "body": string(raw)}, nil
	}
	return out, nil
}

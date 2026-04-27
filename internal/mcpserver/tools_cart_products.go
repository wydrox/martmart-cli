package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wydrox/martmart-cli/internal/delio"
	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
	"github.com/wydrox/martmart-cli/internal/shared"
)

// mcpCPFriscoToolOut is the structured tool result envelope for MCP tools.
type mcpCPFriscoToolOut struct {
	OK   bool           `json:"ok" jsonschema:"true when the provider API call completed without a transport error"`
	Data map[string]any `json:"data,omitempty" jsonschema:"Normalized payload envelope with api_response containing provider JSON"`
}

func mcpResolveProvider(provider string) (string, error) {
	provider = session.NormalizeProvider(provider)
	if provider == "" {
		return "", errors.New("provider is required; ask the user whether to use frisco or delio")
	}
	if isUpMenuProviderAlias(provider) {
		return "", errUpMenuLegacyUnsupported
	}
	if err := session.ValidateProvider(provider); err != nil {
		return "", err
	}
	return provider, nil
}

var errUpMenuLegacyUnsupported = errors.New(
	`provider "upmenu" is not supported by the legacy Frisco/Delio MCP tools; ` +
		`use upmenu_restaurant_info, upmenu_menu_show, upmenu_cart_show, or upmenu_cart_add instead`,
)

func isUpMenuProviderAlias(provider string) bool {
	switch strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(provider))) {
	case "upmenu", "dobrabula", "dobrabuła":
		return true
	default:
		return false
	}
}

func mcpAvailableProviders() []string {
	return session.SupportedProviders()
}

var (
	mcpDelioCurrentCartFn        = delio.CurrentCart
	mcpDelioExtractCurrentCartFn = delio.ExtractCurrentCart
	mcpDelioUpdateCurrentCartFn  = delio.UpdateCurrentCart
	mcpDelioExtractUpdatedCartFn = delio.ExtractUpdatedCart
	mcpDelioSearchProductsFn     = delio.SearchProducts
	mcpDelioGetProductFn         = delio.GetProduct
)

// registerCartAndProductsTools registers all cart and product MCP tools.
func registerCartAndProductsTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cart_show",
		Description: "Fetch the current shopping cart for Frisco or Delio. Frisco uses /app/commerce/api/v1/users/{id}/cart; Delio uses the currentCart GraphQL query.",
	}, mcpCPCartShow)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cart_add",
		Description: "Add or set product quantity in the cart. Frisco uses PUT /cart with products[]; Delio uses UpdateCurrentCart and SKU quantities.",
	}, mcpCPCartAdd)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cart_remove",
		Description: "Remove a product from the cart by setting quantity to 0. Frisco uses PUT /cart; Delio uses UpdateCurrentCart with a negative SKU delta.",
	}, mcpCPCartRemove)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "products_search",
		Description: "Search the product catalog for Frisco or Delio. Frisco uses /offer/products/query; Delio uses the ProductSearch GraphQL query. category_id and delivery_method apply to Frisco only.",
	}, mcpCPProductsSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "products_by_ids",
		Description: "Fetch products by provider product IDs. Frisco uses repeated productIds query params; Delio resolves each SKU via the Product GraphQL query.",
	}, mcpCPProductsByIDs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "products_nutrition",
		Description: "Fetch product nutrition/content when the provider exposes it. Full content API support currently applies to Frisco; Delio returns a clear unsupported error.",
	}, mcpCPProductsNutrition)
}

// mcpCPCartShowIn is the input type for the cart_show tool.
type mcpCPCartShowIn struct {
	Provider string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID   string `json:"user_id,omitempty" jsonschema:"provider user id; falls back to session user_id"`
}

func mcpCPCartShow(_ context.Context, _ *mcp.CallToolRequest, in mcpCPCartShowIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	provider, s, err := loadSessionOnlyAuth(in.Provider)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	if provider == session.ProviderDelio {
		result, err := mcpDelioCurrentCartFn(s)
		if err != nil {
			return nil, mcpCPFriscoToolOut{}, err
		}
		return mcpCPWrapFriscoResult(result)
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// mcpCPCartAddIn is the input type for the cart_add tool.
type mcpCPCartAddIn struct {
	Provider  string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	ProductID string `json:"product_id" jsonschema:"provider product id"`
	Quantity  *int   `json:"quantity,omitempty" jsonschema:"defaults to 1 when omitted"`
	UserID    string `json:"user_id,omitempty" jsonschema:"optional override of session user_id"`
}

func mcpCPCartAdd(_ context.Context, _ *mcp.CallToolRequest, in mcpCPCartAddIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.ProductID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("product_id is required")
	}
	provider, s, err := loadSessionOnlyAuth(in.Provider)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	qty := 1
	if in.Quantity != nil {
		qty = *in.Quantity
	}
	if qty < 0 {
		return nil, mcpCPFriscoToolOut{}, errors.New("quantity must be >= 0")
	}
	if provider == session.ProviderDelio {
		result, err := mcpDelioSetCartQuantity(s, in.ProductID, qty)
		if err != nil {
			return nil, mcpCPFriscoToolOut{}, err
		}
		return mcpCPWrapFriscoResult(result)
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
	body := map[string]any{
		"products": []any{
			map[string]any{"productId": in.ProductID, "quantity": qty},
		},
	}
	result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// mcpCPCartRemoveIn is the input type for the cart_remove tool.
type mcpCPCartRemoveIn struct {
	Provider  string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	ProductID string `json:"product_id" jsonschema:"provider product id"`
	UserID    string `json:"user_id,omitempty" jsonschema:"optional override of session user_id"`
}

func mcpCPCartRemove(_ context.Context, _ *mcp.CallToolRequest, in mcpCPCartRemoveIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.ProductID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("product_id is required")
	}
	provider, s, err := loadSessionOnlyAuth(in.Provider)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	if provider == session.ProviderDelio {
		result, err := mcpDelioSetCartQuantity(s, in.ProductID, 0)
		if err != nil {
			return nil, mcpCPFriscoToolOut{}, err
		}
		return mcpCPWrapFriscoResult(result)
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
	body := map[string]any{
		"products": []any{
			map[string]any{"productId": in.ProductID, "quantity": 0},
		},
	}
	result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// mcpCPProductsSearchIn is the input type for the products_search tool.
type mcpCPProductsSearchIn struct {
	Provider       string  `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	Search         string  `json:"search" jsonschema:"search phrase (purpose=Listing)"`
	CategoryID     string  `json:"category_id,omitempty" jsonschema:"optional categoryId to narrow results"`
	PageIndex      *int    `json:"page_index,omitempty" jsonschema:"1-based page index; default 1"`
	PageSize       *int    `json:"page_size,omitempty" jsonschema:"default 84"`
	DeliveryMethod *string `json:"delivery_method,omitempty" jsonschema:"default Van"`
	UserID         string  `json:"user_id,omitempty" jsonschema:"optional override of session user_id"`
}

func mcpCPProductsSearch(_ context.Context, _ *mcp.CallToolRequest, in mcpCPProductsSearchIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.Search) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("search is required")
	}
	provider, s, err := loadSessionOnlyAuth(in.Provider)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	pageIndex := 1
	if in.PageIndex != nil {
		pageIndex = *in.PageIndex
	}
	if pageIndex <= 0 {
		pageIndex = 1
	}
	pageSize := 84
	if in.PageSize != nil {
		pageSize = *in.PageSize
	}
	if pageSize <= 0 {
		pageSize = 84
	}
	if provider == session.ProviderDelio {
		result, err := mcpDelioSearchProductsFn(s, in.Search, pageSize, (pageIndex-1)*pageSize, nil)
		if err != nil {
			return nil, mcpCPFriscoToolOut{}, err
		}
		return mcpCPWrapFriscoResult(result)
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	deliveryMethod := "Van"
	if in.DeliveryMethod != nil && *in.DeliveryMethod != "" {
		deliveryMethod = *in.DeliveryMethod
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products/query", uid)
	q := []string{
		"purpose=Listing",
		fmt.Sprintf("pageIndex=%d", pageIndex),
		fmt.Sprintf("search=%s", url.QueryEscape(in.Search)),
		"includeFacets=true",
		fmt.Sprintf("deliveryMethod=%s", deliveryMethod),
		fmt.Sprintf("pageSize=%d", pageSize),
		"language=pl",
		"disableAutocorrect=false",
	}
	if strings.TrimSpace(in.CategoryID) != "" {
		q = append(q, fmt.Sprintf("categoryId=%s", strings.TrimSpace(in.CategoryID)))
	}
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// mcpCPProductsByIDsIn is the input type for the products_by_ids tool.
type mcpCPProductsByIDsIn struct {
	Provider   string   `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	ProductIDs []string `json:"product_ids" jsonschema:"list of provider product ids"`
	UserID     string   `json:"user_id,omitempty" jsonschema:"optional override of session user_id"`
}

func mcpCPProductsByIDs(_ context.Context, _ *mcp.CallToolRequest, in mcpCPProductsByIDsIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if len(in.ProductIDs) == 0 {
		return nil, mcpCPFriscoToolOut{}, errors.New("product_ids must contain at least one id")
	}
	provider, s, err := loadSessionOnlyAuth(in.Provider)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	if provider == session.ProviderDelio {
		products := make([]map[string]any, 0, len(in.ProductIDs))
		for _, pid := range in.ProductIDs {
			pid = strings.TrimSpace(pid)
			if pid == "" {
				continue
			}
			payload, err := mcpDelioGetProductFn(s, "", pid, nil)
			if err != nil {
				return nil, mcpCPFriscoToolOut{}, err
			}
			product, err := delio.ExtractProduct(payload)
			if err != nil {
				return nil, mcpCPFriscoToolOut{}, err
			}
			products = append(products, product)
		}
		return mcpCPWrapFriscoResult(map[string]any{
			"provider":    session.ProviderDelio,
			"product_ids": in.ProductIDs,
			"products":    products,
		})
	}
	uid, err := session.RequireUserID(s, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products", uid)
	var q []string
	for _, pid := range in.ProductIDs {
		q = append(q, fmt.Sprintf("productIds=%s", url.QueryEscape(pid)))
	}
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// mcpCPProductsNutritionIn is the input type for the products_nutrition tool.
type mcpCPProductsNutritionIn struct {
	Provider  string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	ProductID string `json:"product_id" jsonschema:"provider product id"`
	Raw       bool   `json:"raw,omitempty" jsonschema:"if true, return full API JSON; default false"`
}

func mcpCPProductsNutrition(_ context.Context, _ *mcp.CallToolRequest, in mcpCPProductsNutritionIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.ProductID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("product_id is required")
	}
	provider, s, err := loadSessionOnlyAuth(in.Provider)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	if provider == session.ProviderDelio {
		return nil, mcpCPFriscoToolOut{}, errors.New("products_nutrition is currently supported for Frisco only; Delio does not expose a compatible content API in MartMart MCP yet")
	}
	path := fmt.Sprintf("/app/content/api/v1/products/get/%s", in.ProductID)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	if in.Raw {
		return mcpCPWrapFriscoResult(result)
	}
	nutrition := shared.ExtractNutritionBlock(result)
	if nutrition == nil {
		out := map[string]any{
			"productId": in.ProductID,
			"message":   "No explicit nutrition block in this response. Use raw=true for the full API JSON.",
		}
		return mcpCPWrapFriscoResult(out)
	}
	out := map[string]any{
		"productId":  in.ProductID,
		"nutrition":  nutrition,
		"sourcePath": "/app/content/api/v1/products/get/{id}",
	}
	return mcpCPWrapFriscoResult(out)
}

var errNotAuthenticated = errors.New(
	"not authenticated — no session found; " +
		"call session_status to inspect saved sessions, then session_login to open a browser and log in interactively, " +
		"or use session_from_curl with a cURL copied from DevTools",
)

func loadSessionOnlyAuth(provider string) (string, *session.Session, error) {
	provider, err := mcpResolveProvider(provider)
	if err != nil {
		return "", nil, err
	}
	s, err := session.LoadProvider(provider)
	if err != nil {
		return "", nil, err
	}
	if !session.IsAuthenticated(s) {
		return "", nil, errNotAuthenticated
	}
	return provider, s, nil
}

// loadSessionAuth loads the provider session and verifies it has authentication credentials.
func loadSessionAuth(provider string, explicitUserID string) (*session.Session, string, error) {
	_, s, err := loadSessionOnlyAuth(provider)
	if err != nil {
		return nil, "", err
	}
	uid, err := session.RequireUserID(s, explicitUserID)
	if err != nil {
		return nil, "", err
	}
	return s, uid, nil
}

func mcpDelioSetCartQuantity(s *session.Session, sku string, targetQty int) (any, error) {
	sku = strings.TrimSpace(sku)
	if sku == "" {
		return nil, errors.New("product_id is required")
	}
	if targetQty < 0 {
		return nil, errors.New("quantity must be >= 0")
	}
	current, err := mcpDelioCurrentCartFn(s)
	if err != nil {
		return nil, err
	}
	cart, err := mcpDelioExtractCurrentCartFn(current)
	if err != nil {
		return nil, err
	}
	currentQty := mcpDelioCartItemQuantity(cart, sku)
	delta := targetQty - currentQty
	if delta == 0 {
		return current, nil
	}
	result, err := mcpDelioUpdateCurrentCartFn(s, mcpCPString(cart["id"]), []map[string]any{{
		"AddLineItem": map[string]any{"quantity": delta, "sku": sku},
	}})
	if err != nil {
		return nil, err
	}
	if _, err := mcpDelioExtractUpdatedCartFn(result); err != nil {
		return nil, err
	}
	return result, nil
}

func mcpDelioCartItemQuantity(cart map[string]any, sku string) int {
	sku = strings.TrimSpace(sku)
	if cart == nil || sku == "" {
		return 0
	}
	items, _ := cart["lineItems"].([]any)
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		product, _ := row["product"].(map[string]any)
		if !strings.EqualFold(mcpCPString(product["sku"]), sku) {
			continue
		}
		switch q := row["quantity"].(type) {
		case int:
			return q
		case int64:
			return int(q)
		case float64:
			return int(q)
		}
	}
	return 0
}

func mcpCPString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

// mcpCPWrapFriscoResult marshals v into a CallToolResult and the structured
// mcpCPFriscoToolOut envelope, truncating the text content at 8000 runes.
func mcpCPWrapFriscoResult(v any) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	text := string(raw)
	const mcpCPMaxSummaryRunes = 8000
	if utf8.RuneCountInString(text) > mcpCPMaxSummaryRunes {
		text = string([]rune(text)[:mcpCPMaxSummaryRunes]) + "...[truncated]"
	}
	res := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
	return res, mcpCPFriscoToolOut{
		OK: true,
		Data: map[string]any{
			"api_response": v,
		},
	}, nil
}

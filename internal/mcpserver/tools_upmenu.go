package mcpserver

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wydrox/martmart-cli/internal/upmenu"
)

func registerUpMenuTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "upmenu_restaurant_info",
		Description: "Fetch public restaurant information from the UpMenu/Dobra Buła storefront. Defaults to the Dobra Buła Solidarności restaurant page when restaurant_url is omitted.",
	}, toolUpMenuRestaurantInfo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "upmenu_menu_show",
		Description: "Fetch the public UpMenu menu for the Dobra Buła MVP storefront. Defaults to the Dobra Buła Solidarności restaurant page when restaurant_url is omitted.",
	}, toolUpMenuMenuShow)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "upmenu_cart_show",
		Description: "Fetch the current public UpMenu cart snapshot. Pass cart_id to resume an existing cart; otherwise UpMenu may create or return an empty cart.",
	}, toolUpMenuCartShow)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "upmenu_cart_add",
		Description: "Add an UpMenu product price id to the public cart for the Dobra Buła MVP storefront. product_id must be an UpMenu product price id from the menu payload.",
	}, toolUpMenuCartAdd)
}

type upmenuBaseIn struct {
	RestaurantURL string `json:"restaurant_url,omitempty" jsonschema:"optional absolute UpMenu restaurant page URL; defaults to the Dobra Buła Solidarności storefront"`
	Language      string `json:"language,omitempty" jsonschema:"optional storefront language header; default pl"`
}

type upmenuRestaurantInfoIn struct{ upmenuBaseIn }

type upmenuMenuShowIn struct{ upmenuBaseIn }

type upmenuCartShowIn struct {
	upmenuBaseIn
	CartID     string `json:"cart_id,omitempty" jsonschema:"optional existing UpMenu cart id"`
	CustomerID string `json:"customer_id,omitempty" jsonschema:"optional public/customer id when resuming a known cart"`
}

type upmenuCartAddIn struct {
	upmenuBaseIn
	CartID     string `json:"cart_id,omitempty" jsonschema:"optional existing UpMenu cart id"`
	ProductID  string `json:"product_id" jsonschema:"required UpMenu product price id from the menu payload"`
	CustomerID string `json:"customer_id,omitempty" jsonschema:"optional public/customer id when resuming a known cart"`
}

func toolUpMenuRestaurantInfo(ctx context.Context, _ *mcp.CallToolRequest, in upmenuRestaurantInfoIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	client, err := newUpMenuClient(in.RestaurantURL, in.Language)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	result, err := client.RestaurantInfo(ctx)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

func toolUpMenuMenuShow(ctx context.Context, _ *mcp.CallToolRequest, in upmenuMenuShowIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	client, err := newUpMenuClient(in.RestaurantURL, in.Language)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	result, err := client.Menu(ctx)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

func toolUpMenuCartShow(ctx context.Context, _ *mcp.CallToolRequest, in upmenuCartShowIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	client, err := newUpMenuClient(in.RestaurantURL, in.Language)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	state := client.State()
	state.CartID = strings.TrimSpace(in.CartID)
	client.SetState(state)
	result, err := client.ShowCart(ctx)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

func toolUpMenuCartAdd(ctx context.Context, _ *mcp.CallToolRequest, in upmenuCartAddIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.ProductID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("product_id is required")
	}
	client, err := newUpMenuClient(in.RestaurantURL, in.Language)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	state := client.State()
	state.CartID = strings.TrimSpace(in.CartID)
	client.SetState(state)
	result, err := client.AddSimple(ctx, in.ProductID, 1)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

func newUpMenuClient(restaurantURL, language string) (*upmenu.Client, error) {
	cfg := upmenu.Config{
		BaseURL:      upmenu.DefaultBaseURL,
		SiteID:       upmenu.DefaultSiteID,
		RestaurantID: upmenu.DefaultRestaurantID,
		Language:     upmenu.DefaultLanguage,
	}
	trimmedRestaurantURL := strings.TrimSpace(restaurantURL)
	if trimmedRestaurantURL != "" {
		if u, err := url.Parse(trimmedRestaurantURL); err == nil && u.Scheme != "" && u.Host != "" {
			cfg.BaseURL = (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
		}
	}
	if strings.TrimSpace(language) != "" {
		cfg.Language = strings.TrimSpace(language)
	}
	if trimmedRestaurantURL != "" {
		return upmenu.NewClientFromRestaurantURL(context.Background(), trimmedRestaurantURL, cfg)
	}
	return upmenu.NewClient(cfg)
}

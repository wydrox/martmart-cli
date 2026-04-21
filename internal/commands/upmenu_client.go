package commands

import (
	"context"
	"net/url"
	"strings"

	"github.com/wydrox/martmart-cli/internal/session"
	"github.com/wydrox/martmart-cli/internal/upmenu"
)

type upmenuCLIClient interface {
	RestaurantInfo(ctx context.Context) (any, error)
	Menu(ctx context.Context) (any, error)
	CartShow(ctx context.Context, cartID, customerID string) (any, error)
	CartAdd(ctx context.Context, cartID, productID, customerID string, quantity int) (any, error)
}

type upmenuCLIWrapper struct {
	client *upmenu.Client
}

func (w *upmenuCLIWrapper) RestaurantInfo(ctx context.Context) (any, error) {
	return w.client.RestaurantInfo(ctx)
}

func (w *upmenuCLIWrapper) Menu(ctx context.Context) (any, error) {
	return w.client.Menu(ctx)
}

func (w *upmenuCLIWrapper) CartShow(ctx context.Context, cartID, _ string) (any, error) {
	state := w.client.State()
	state.CartID = strings.TrimSpace(cartID)
	w.client.SetState(state)
	return w.client.ShowCart(ctx)
}

func (w *upmenuCLIWrapper) CartAdd(ctx context.Context, cartID, productID, _ string, quantity int) (any, error) {
	state := w.client.State()
	state.CartID = strings.TrimSpace(cartID)
	w.client.SetState(state)
	return w.client.AddSimple(ctx, strings.TrimSpace(productID), quantity)
}

var newUpMenuCLIClient = func(s *session.Session, restaurantURL, language string) (upmenuCLIClient, error) {
	cfg := upmenu.Config{
		BaseURL:      upmenu.DefaultBaseURL,
		SiteID:       upmenu.DefaultSiteID,
		RestaurantID: upmenu.DefaultRestaurantID,
		Language:     upmenu.DefaultLanguage,
	}
	if s != nil && strings.TrimSpace(s.BaseURL) != "" {
		cfg.BaseURL = strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
	}
	if trimmed := strings.TrimSpace(restaurantURL); trimmed != "" {
		if u, err := url.Parse(trimmed); err == nil && u.Scheme != "" && u.Host != "" {
			cfg.BaseURL = (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
		}
	}
	if strings.TrimSpace(language) != "" {
		cfg.Language = strings.TrimSpace(language)
	}
	var client *upmenu.Client
	var err error
	if strings.TrimSpace(restaurantURL) != "" {
		client, err = upmenu.NewClientFromRestaurantURL(context.Background(), restaurantURL, cfg)
	} else {
		client, err = upmenu.NewClient(cfg)
	}
	if err != nil {
		return nil, err
	}
	return &upmenuCLIWrapper{client: client}, nil
}

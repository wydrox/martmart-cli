package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPResolveProvider_RejectsUpMenuAlias(t *testing.T) {
	for _, provider := range []string{"upmenu", "Dobra_Bula", "dobra-bula"} {
		_, err := mcpResolveProvider(provider)
		if err == nil {
			t.Fatalf("expected error for provider %q", provider)
		}
		if !strings.Contains(err.Error(), "upmenu_restaurant_info") {
			t.Fatalf("unexpected error for %q: %v", provider, err)
		}
	}
}

func TestToolUpMenuRestaurantInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/restaurant":
			_, _ = w.Write([]byte(`<script>com.upmenu.siteId = 'site-a'; com.upmenu.restaurantId = 'rest-a';</script>`))
		case "/restapi/restaurant/rest-a":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"rest-a","name":"Dobra Buła"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	res, out, err := toolUpMenuRestaurantInfo(context.Background(), nil, upmenuRestaurantInfoIn{upmenuBaseIn: upmenuBaseIn{RestaurantURL: srv.URL + "/restaurant"}})
	if err != nil {
		t.Fatalf("toolUpMenuRestaurantInfo: %v", err)
	}
	if !out.OK || res == nil {
		t.Fatalf("unexpected result envelope: %+v %v", out, res)
	}
	payload := out.Data["api_response"].(map[string]any)
	if payload["name"] != "Dobra Buła" {
		t.Fatalf("unexpected payload: %v", payload)
	}
}

func TestToolUpMenuCartShow(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/restaurant":
			_, _ = w.Write([]byte(`<script>com.upmenu.siteId = 'site-b'; com.upmenu.restaurantId = 'rest-b';</script>`))
		case "/restapi/cart/site-b/rest-b":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"cartId":"cart-b","items":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	_, out, err := toolUpMenuCartShow(context.Background(), nil, upmenuCartShowIn{
		upmenuBaseIn: upmenuBaseIn{RestaurantURL: srv.URL + "/restaurant"},
		CartID:       "cart-b",
		CustomerID:   "cust-b",
	})
	if err != nil {
		t.Fatalf("toolUpMenuCartShow: %v", err)
	}
	if payload["cartId"] != "cart-b" || payload["customerId"] != "cust-b" {
		t.Fatalf("unexpected cart payload: %v", payload)
	}
	api := out.Data["api_response"].(map[string]any)
	if api["cartId"] != "cart-b" {
		t.Fatalf("unexpected api response: %v", api)
	}
}

func TestToolUpMenuCartAddRequiresProductID(t *testing.T) {
	_, _, err := toolUpMenuCartAdd(context.Background(), nil, upmenuCartAddIn{})
	if err == nil || !strings.Contains(err.Error(), "product_id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

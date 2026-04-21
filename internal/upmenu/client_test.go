package upmenu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRestaurantInfoResolvesMetadataFromPage(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest":
			_, _ = w.Write([]byte(`<script>com.upmenu.siteId = 'site-1'; com.upmenu.restaurantId = 'rest-1';</script>`))
		case "/restapi/restaurant/rest-1":
			gotPath = r.URL.Path
			if r.Header.Get("language") != "pl" {
				t.Fatalf("language header = %q", r.Header.Get("language"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"rest-1","name":"Dobra Buła"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(Config{RestaurantURL: srv.URL + "/rest", HTTPClient: srv.Client()})
	result, err := client.RestaurantInfo(context.Background())
	if err != nil {
		t.Fatalf("RestaurantInfo: %v", err)
	}
	if gotPath != "/restapi/restaurant/rest-1" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	m := result.(map[string]any)
	if m["name"] != "Dobra Buła" {
		t.Fatalf("unexpected payload: %v", m)
	}
}

func TestMenuUsesCms2Endpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest":
			_, _ = w.Write([]byte(`<script>com.upmenu.siteId = 'site-9'; com.upmenu.restaurantId = 'rest-9';</script>`))
		case "/api/v2/menu/site-9/rest-9":
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"categories":[{"name":"Burgery"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(Config{RestaurantURL: srv.URL + "/rest", HTTPClient: srv.Client()})
	result, err := client.Menu(context.Background())
	if err != nil {
		t.Fatalf("Menu: %v", err)
	}
	if gotPath != "/api/v2/menu/site-9/rest-9" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	m := result.(map[string]any)
	cats := m["categories"].([]any)
	if len(cats) != 1 {
		t.Fatalf("unexpected payload: %v", m)
	}
}

func TestCartShowPostsExpectedPayload(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest":
			_, _ = w.Write([]byte(`<script>com.upmenu.siteId = 'site-2'; com.upmenu.restaurantId = 'rest-2';</script>`))
		case "/restapi/cart/site-2/rest-2":
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s", r.Method)
			}
			if r.Header.Get("orderSource") != "WWW" {
				t.Fatalf("orderSource = %q", r.Header.Get("orderSource"))
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"cartId":"cart-1","items":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(Config{RestaurantURL: srv.URL + "/rest", HTTPClient: srv.Client()})
	result, err := client.CartShow(context.Background(), "cart-1", "cust-1")
	if err != nil {
		t.Fatalf("CartShow: %v", err)
	}
	if payload["cartId"] != "cart-1" || payload["customerId"] != "cust-1" || payload["cartLocation"] != "MENU" {
		t.Fatalf("unexpected payload: %v", payload)
	}
	m := result.(map[string]any)
	if m["cartId"] != "cart-1" {
		t.Fatalf("unexpected result: %v", m)
	}
}

func TestCartAddPostsExpectedPayload(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest":
			_, _ = w.Write([]byte(`<script>com.upmenu.siteId = 'site-3'; com.upmenu.restaurantId = 'rest-3';</script>`))
		case "/restapi/cart/items/add/rest-3":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"cartId":"cart-3","items":[{"productId":"pp-1"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(Config{RestaurantURL: srv.URL + "/rest", HTTPClient: srv.Client()})
	result, err := client.CartAdd(context.Background(), "cart-3", "pp-1", "cust-3")
	if err != nil {
		t.Fatalf("CartAdd: %v", err)
	}
	if payload["cartId"] != "cart-3" || payload["productId"] != "pp-1" || payload["customerId"] != "cust-3" {
		t.Fatalf("unexpected payload: %v", payload)
	}
	m := result.(map[string]any)
	if m["cartId"] != "cart-3" {
		t.Fatalf("unexpected result: %v", m)
	}
}

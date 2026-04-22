package upmenu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestaurantInfo(t *testing.T) {
	ts := newFixtureServer(t)
	defer ts.Close()

	client := newTestClient(t, ts)
	info, err := client.RestaurantInfo(context.Background())
	if err != nil {
		t.Fatalf("RestaurantInfo: %v", err)
	}
	if info.Name != "Dobra Buła Wola" {
		t.Fatalf("name=%q", info.Name)
	}
	if info.MinimumOrderPrice == nil || *info.MinimumOrderPrice != 38 {
		t.Fatalf("minimum_order_price=%v", info.MinimumOrderPrice)
	}
}

func TestMenuJSON(t *testing.T) {
	ts := newFixtureServer(t)
	defer ts.Close()

	client := newTestClient(t, ts)
	menu, err := client.MenuJSON(context.Background())
	if err != nil {
		t.Fatalf("MenuJSON: %v", err)
	}
	if len(menu.Categories) == 0 || len(menu.Products) == 0 {
		t.Fatalf("unexpected empty menu: %+v", menu)
	}
	if menu.Products[0].ProductPriceID == "" {
		t.Fatal("expected product price id")
	}
}

func TestMenuParsesHTML(t *testing.T) {
	ts := newFixtureServer(t)
	defer ts.Close()

	client := newTestClient(t, ts)
	menu, err := client.Menu(context.Background())
	if err != nil {
		t.Fatalf("Menu: %v", err)
	}
	if len(menu.Products) == 0 {
		t.Fatal("expected parsed products")
	}
	if menu.Products[0].Name != "Truflowe Love Burger" {
		t.Fatalf("first product=%q", menu.Products[0].Name)
	}
}

func TestShowCartMutatesState(t *testing.T) {
	ts := newFixtureServer(t)
	defer ts.Close()

	client := newTestClient(t, ts)
	cart, err := client.ShowCart(context.Background())
	if err != nil {
		t.Fatalf("ShowCart: %v", err)
	}
	if cart.ID == "" {
		t.Fatal("expected cart id")
	}
	if client.State().CartID != cart.ID {
		t.Fatalf("state cart id=%q want=%q", client.State().CartID, cart.ID)
	}
}

func TestAddSimpleUsesBuyingFlowAndUpdatesCart(t *testing.T) {
	ts := newFixtureServer(t)
	defer ts.Close()

	client := newTestClient(t, ts)
	cart, err := client.AddSimple(context.Background(), "16848b98-94d0-11f0-9141-525400080621", 1)
	if err != nil {
		t.Fatalf("AddSimple: %v", err)
	}
	if cart.ItemsSize != 1 {
		t.Fatalf("items_size=%d", cart.ItemsSize)
	}
	if len(cart.Items) != 1 || cart.Items[0].ProductPriceID != "16848b98-94d0-11f0-9141-525400080621" {
		t.Fatalf("unexpected cart items: %+v", cart.Items)
	}
	if client.State().CartID != "2c62e3d7-3da9-11f1-9141-525400080621" {
		t.Fatalf("state cart id=%q", client.State().CartID)
	}
}

func TestAddSimpleRejectsRequiredProducts(t *testing.T) {
	ts := newFixtureServer(t)
	defer ts.Close()

	client := newTestClient(t, ts)
	_, err := client.AddSimple(context.Background(), "6cc16155-1669-11f1-9141-525400080621", 1)
	if err != ErrProductRequiresConfiguration {
		t.Fatalf("err=%v want=%v", err, ErrProductRequiresConfiguration)
	}
}

func TestAddSimpleQuantityValidation(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "https://example.com", SiteID: "s", RestaurantID: "r"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.AddSimple(context.Background(), "p1", 0); err == nil {
		t.Fatal("expected quantity validation error")
	}
}

func TestStartBuyingFlowIncludesCartIDQuery(t *testing.T) {
	var rawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"productPriceId":"p1","quantity":1,"steps":[],"errors":[]}`)
	}))
	defer ts.Close()

	client := newTestClient(t, ts)
	client.SetState(State{CartID: "cart 123"})
	flow, err := client.StartBuyingFlow(context.Background(), "p1")
	if err != nil {
		t.Fatalf("StartBuyingFlow: %v", err)
	}
	if flow.ProductPriceID != "p1" {
		t.Fatalf("flow=%+v", flow)
	}
	if !strings.Contains(rawQuery, "cartId=cart+123") {
		t.Fatalf("rawQuery=%q", rawQuery)
	}
}

func TestFinishBuyingFlowRequiresPayload(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "https://example.com", SiteID: "s", RestaurantID: "r"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.FinishBuyingFlow(context.Background(), nil); err == nil {
		t.Fatal("expected missing payload error")
	}
}

func TestFinishBuyingFlowUpdatesState(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"cartId":"cart-1","productPriceId":"p1","quantity":1,"steps":[],"errors":[]}`)
	}))
	defer ts.Close()

	client := newTestClient(t, ts)
	flow, err := client.FinishBuyingFlow(context.Background(), &BuyingFlow{Raw: map[string]any{"productPriceId": "p1"}})
	if err != nil {
		t.Fatalf("FinishBuyingFlow: %v", err)
	}
	if flow.CartID != "cart-1" || client.State().CartID != "cart-1" {
		t.Fatalf("flow=%+v state=%+v", flow, client.State())
	}
}

func TestShowCartRequestDefaults(t *testing.T) {
	var got CartRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = io.WriteString(w, `{"cart":{"id":"c1"}}`)
	}))
	defer ts.Close()

	client := newTestClient(t, ts)
	if _, err := client.ShowCart(context.Background()); err != nil {
		t.Fatalf("ShowCart: %v", err)
	}
	if got.DeliveryType != DefaultDeliveryType || got.CartLocation != DefaultCartLocation || got.PaymentMethod != DefaultPaymentMethod {
		t.Fatalf("unexpected request: %+v", got)
	}
}

func TestNewClientFromRestaurantURLExtractsIDs(t *testing.T) {
	html := `
		<html><body>
		<script>
		com.upmenu.siteId = 'site-123';
		com.upmenu.restaurantId = 'rest-456';
		</script>
		</body></html>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, html)
	}))
	defer ts.Close()

	client, err := NewClientFromRestaurantURL(context.Background(), ts.URL, Config{})
	if err != nil {
		t.Fatalf("NewClientFromRestaurantURL: %v", err)
	}
	if client.cfg.BaseURL != ts.URL || client.cfg.SiteID != "site-123" || client.cfg.RestaurantID != "rest-456" {
		t.Fatalf("cfg=%+v", client.cfg)
	}
}

func TestNormalizeHelpers(t *testing.T) {
	if got := firstNonEmpty("", " a "); got != "a" {
		t.Fatalf("firstNonEmpty=%q", got)
	}
	if got := stringValue(12); got != "12" {
		t.Fatalf("stringValue=%q", got)
	}
	if got := intValue(3.9); got != 3 {
		t.Fatalf("intValue=%d", got)
	}
	if got := normalizeWhitespace(" a\n b \t c "); got != "a b c" {
		t.Fatalf("normalizeWhitespace=%q", got)
	}
	if got := extractBackgroundURL("background-image: url(https://example.com/x.webp);"); got != "https://example.com/x.webp" {
		t.Fatalf("extractBackgroundURL=%q", got)
	}
}

func TestFixtureFilesExist(t *testing.T) {
	for _, name := range []string{
		"restaurant_wola.json",
		"menu_wola.json",
		"menu_wola.html",
		"cart_empty_wola.json",
		"buying_flow_simple_start.json",
		"buying_flow_simple_finish.json",
	} {
		if _, err := os.Stat(filepath.Join("testdata", name)); err != nil {
			t.Fatalf("missing fixture %s: %v", name, err)
		}
	}
}

func newTestClient(t *testing.T, ts *httptest.Server) *Client {
	t.Helper()
	client, err := NewClient(Config{BaseURL: ts.URL, SiteID: DefaultSiteID, RestaurantID: DefaultRestaurantID})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func newFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()

	readFixture := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		return data
	}

	restaurant := readFixture("restaurant_wola.json")
	menuJSON := readFixture("menu_wola.json")
	menuHTML := readFixture("menu_wola.html")
	cartEmpty := readFixture("cart_empty_wola.json")
	startSimple := readFixture("buying_flow_simple_start.json")
	finishSimple := readFixture("buying_flow_simple_finish.json")

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/restapi/restaurant/"+DefaultRestaurantID:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(restaurant)
		case r.Method == http.MethodGet && r.URL.Path == "/restapi/menu/"+DefaultSiteID+"/"+DefaultRestaurantID:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(menuJSON)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/menu/"+DefaultSiteID+"/"+DefaultRestaurantID:
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(menuHTML)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/restapi/buyingFlow/required/"):
			w.Header().Set("Content-Type", "application/json")
			if strings.HasSuffix(r.URL.Path, "/16848b98-94d0-11f0-9141-525400080621") {
				_, _ = io.WriteString(w, `{"required":false}`)
				return
			}
			_, _ = io.WriteString(w, `{"required":true}`)
		case r.Method == http.MethodPost && r.URL.Path == "/restapi/buyingFlow/startByProductPrice/"+DefaultSiteID+"/"+DefaultRestaurantID+"/16848b98-94d0-11f0-9141-525400080621":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(startSimple)
		case r.Method == http.MethodPost && r.URL.Path == "/restapi/buyingFlow/startByProductPrice/"+DefaultSiteID+"/"+DefaultRestaurantID+"/6cc16155-1669-11f1-9141-525400080621":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"productPriceId":"6cc16155-1669-11f1-9141-525400080621","quantity":1,"steps":[{"id":"step-1","name":"Choose option","done":false}],"errors":[]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/restapi/buyingFlow/finish/"+DefaultSiteID+"/"+DefaultRestaurantID:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(finishSimple)
		case r.Method == http.MethodPost && r.URL.Path == "/restapi/cart/"+DefaultSiteID+"/"+DefaultRestaurantID:
			var req CartRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode cart request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			if req.CartID == "2c62e3d7-3da9-11f1-9141-525400080621" {
				_, _ = io.WriteString(w, `{"cart":{"id":"2c62e3d7-3da9-11f1-9141-525400080621","totalCost":9,"productsCost":4,"deliveryCost":5,"deliveryType":"DELIVERY","deliveryStatus":"NO_ADDRESS","itemsSize":1,"messages":["ok"],"errors":[],"items":[{"id":"1d55f35c-3da9-11f1-9141-525400080621","name":"Sos Aioli 30 ml ( z opakowaniem)","productPriceId":"16848b98-94d0-11f0-9141-525400080621","quantity":1,"price":4}]}}`)
				return
			}
			_, _ = w.Write(cartEmpty)
		default:
			http.NotFound(w, r)
		}
	}))
}

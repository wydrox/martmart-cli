package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/session"
)

type fakeUpMenuCLIClient struct {
	restaurantInfoFn func(context.Context) (any, error)
	menuFn           func(context.Context) (any, error)
	cartShowFn       func(context.Context, string, string) (any, error)
	cartAddFn        func(context.Context, string, string, string, int) (any, error)
}

func (f *fakeUpMenuCLIClient) RestaurantInfo(ctx context.Context) (any, error) {
	return f.restaurantInfoFn(ctx)
}

func (f *fakeUpMenuCLIClient) Menu(ctx context.Context) (any, error) {
	return f.menuFn(ctx)
}

func (f *fakeUpMenuCLIClient) CartShow(ctx context.Context, cartID, customerID string) (any, error) {
	return f.cartShowFn(ctx, cartID, customerID)
}

func (f *fakeUpMenuCLIClient) CartAdd(ctx context.Context, cartID, productID, customerID string, quantity int) (any, error) {
	return f.cartAddFn(ctx, cartID, productID, customerID, quantity)
}

func TestNewRestaurantCmdSubcommands(t *testing.T) {
	cmd := newRestaurantCmd()
	got := commandNames(cmd)
	want := []string{"info", "menu"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("subcommands = %v, want %v", got, want)
	}
}

func TestRootIncludesRestaurantCommand(t *testing.T) {
	root := NewRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "restaurant" {
			return
		}
	}
	t.Fatal("expected root to register restaurant command")
}

func TestRestaurantInfoUsesUpMenuClientWithoutProviderFlag(t *testing.T) {
	orig := newUpMenuCLIClient
	defer func() { newUpMenuCLIClient = orig }()

	newUpMenuCLIClient = func(s *session.Session, restaurantURL, language string) (upmenuCLIClient, error) {
		if s != nil {
			t.Fatal("restaurant info should not require a session")
		}
		if restaurantURL != "https://example.com/rest" {
			t.Fatalf("restaurantURL = %q", restaurantURL)
		}
		if language != "pl" {
			t.Fatalf("language = %q", language)
		}
		return &fakeUpMenuCLIClient{
			restaurantInfoFn: func(context.Context) (any, error) {
				return map[string]any{"name": "Dobra Buła", "id": "rest-1"}, nil
			},
			menuFn: func(context.Context) (any, error) { t.Fatal("menu should not be called"); return nil, nil },
			cartShowFn: func(context.Context, string, string) (any, error) {
				t.Fatal("cart show should not be called")
				return nil, nil
			},
			cartAddFn: func(context.Context, string, string, string, int) (any, error) {
				t.Fatal("cart add should not be called")
				return nil, nil
			},
		}, nil
	}

	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--format", "json", "restaurant", "info", "--restaurant-url", "https://example.com/rest", "--language", "pl"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	if !strings.Contains(out, "Dobra Buła") || !strings.Contains(out, "rest-1") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestCartShowUpMenuUsesClient(t *testing.T) {
	orig := newUpMenuCLIClient
	defer func() { newUpMenuCLIClient = orig }()

	newUpMenuCLIClient = func(s *session.Session, restaurantURL, language string) (upmenuCLIClient, error) {
		if s == nil {
			t.Fatal("cart show should receive a session")
		}
		if restaurantURL != "https://example.com/rest" {
			t.Fatalf("restaurantURL = %q", restaurantURL)
		}
		if language != "pl" {
			t.Fatalf("language = %q", language)
		}
		return &fakeUpMenuCLIClient{
			restaurantInfoFn: func(context.Context) (any, error) { t.Fatal("restaurant info should not be called"); return nil, nil },
			menuFn:           func(context.Context) (any, error) { t.Fatal("menu should not be called"); return nil, nil },
			cartShowFn: func(_ context.Context, cartID, customerID string) (any, error) {
				if cartID != "cart-1" || customerID != "cust-1" {
					t.Fatalf("cart show args = %q, %q", cartID, customerID)
				}
				return map[string]any{"cartId": cartID, "customerId": customerID}, nil
			},
			cartAddFn: func(context.Context, string, string, string, int) (any, error) {
				t.Fatal("cart add should not be called")
				return nil, nil
			},
		}, nil
	}

	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderUpMenu, "--format", "json", "cart", "show", "--restaurant-url", "https://example.com/rest", "--language", "pl", "--cart-id", "cart-1", "--customer-id", "cust-1"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	if !strings.Contains(out, "cart-1") || !strings.Contains(out, "cust-1") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestCartAddUpMenuUsesClient(t *testing.T) {
	orig := newUpMenuCLIClient
	defer func() { newUpMenuCLIClient = orig }()

	newUpMenuCLIClient = func(s *session.Session, restaurantURL, language string) (upmenuCLIClient, error) {
		return &fakeUpMenuCLIClient{
			restaurantInfoFn: func(context.Context) (any, error) { t.Fatal("restaurant info should not be called"); return nil, nil },
			menuFn:           func(context.Context) (any, error) { t.Fatal("menu should not be called"); return nil, nil },
			cartShowFn: func(context.Context, string, string) (any, error) {
				t.Fatal("cart show should not be called")
				return nil, nil
			},
			cartAddFn: func(_ context.Context, cartID, productID, customerID string, quantity int) (any, error) {
				if cartID != "cart-2" || productID != "pp-1" || customerID != "cust-2" || quantity != 1 {
					t.Fatalf("cart add args = %q, %q, %q, %d", cartID, productID, customerID, quantity)
				}
				return map[string]any{"cartId": cartID, "productId": productID}, nil
			},
		}, nil
	}

	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderUpMenu, "--format", "json", "cart", "add", "--product-id", "pp-1", "--cart-id", "cart-2", "--customer-id", "cust-2"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	if !strings.Contains(out, "pp-1") || !strings.Contains(out, "cart-2") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestUpMenuGuardsUnsupportedCommands(t *testing.T) {
	cases := [][]string{
		{"--provider", session.ProviderUpMenu, "checkout", "preview"},
		{"--provider", session.ProviderUpMenu, "products", "get", "--product-id", "x"},
		{"--provider", session.ProviderUpMenu, "reservation", "slots"},
		{"--provider", session.ProviderUpMenu, "cart", "remove", "--product-id", "x"},
	}
	for _, args := range cases {
		root := NewRootCmd()
		root.SetArgs(args)
		err := root.Execute()
		if err == nil {
			t.Fatalf("%v: expected error", args)
		}
		if !strings.Contains(err.Error(), "does not support provider \"upmenu\"") {
			t.Fatalf("%v: error = %v", args, err)
		}
	}
}

func TestRestaurantInfoRejectsNonUpMenuProvider(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"--provider", session.ProviderFrisco, "restaurant", "info"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "does not support provider \"frisco\"") {
		t.Fatalf("Execute error = %v", err)
	}
}

func commandNames(cmd *cobra.Command) []string {
	commands := cmd.Commands()
	out := make([]string, 0, len(commands))
	for _, c := range commands {
		out = append(out, c.Name())
	}
	return out
}

package commands

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	checkoutcore "github.com/wydrox/martmart-cli/internal/checkout"
	"github.com/wydrox/martmart-cli/internal/session"
)

type fakeCheckoutClient struct {
	previewFn  func(*session.Session, checkoutcore.PreviewOptions) (*checkoutcore.CheckoutPreview, error)
	finalizeFn func(*session.Session, checkoutcore.FinalizeOptions) (*checkoutcore.FinalizeResult, error)
}

func (f *fakeCheckoutClient) Preview(s *session.Session, opts checkoutcore.PreviewOptions) (*checkoutcore.CheckoutPreview, error) {
	return f.previewFn(s, opts)
}

func (f *fakeCheckoutClient) Finalize(s *session.Session, opts checkoutcore.FinalizeOptions) (*checkoutcore.FinalizeResult, error) {
	return f.finalizeFn(s, opts)
}

func TestNewCheckoutCmdSubcommands(t *testing.T) {
	cmd := newCheckoutCmd()
	got := checkoutSubcommandNames(cmd)
	want := []string{"finalize", "preview"}
	if len(got) != len(want) {
		t.Fatalf("subcommand count = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("subcommands = %v, want %v", got, want)
		}
	}
}

func TestRootIncludesCheckoutCommand(t *testing.T) {
	root := NewRootCmd()
	var found bool
	for _, cmd := range root.Commands() {
		if cmd.Name() == "checkout" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected root to register checkout command")
	}
}

func TestCheckoutFinalizeRequiresConfirm(t *testing.T) {
	origLoad := checkoutLoadSession
	origClient := newCheckoutClient
	defer func() {
		checkoutLoadSession = origLoad
		newCheckoutClient = origClient
	}()

	checkoutLoadSession = func(_ *cobra.Command) (string, *session.Session, error) {
		return session.ProviderFrisco, &session.Session{BaseURL: session.DefaultBaseURL, UserID: "u-1"}, nil
	}
	preview := &checkoutcore.CheckoutPreview{
		Provider:        session.ProviderFrisco,
		UserID:          "u-1",
		CartID:          "cart-1",
		ItemCount:       2,
		ReadyToFinalize: true,
	}
	newCheckoutClient = func(provider string) (checkoutCLIClient, error) {
		if provider != session.ProviderFrisco {
			t.Fatalf("factory provider = %q, want %q", provider, session.ProviderFrisco)
		}
		return &fakeCheckoutClient{
			previewFn: func(_ *session.Session, opts checkoutcore.PreviewOptions) (*checkoutcore.CheckoutPreview, error) {
				if opts.Provider != session.ProviderFrisco {
					t.Fatalf("preview opts.Provider = %q, want %q", opts.Provider, session.ProviderFrisco)
				}
				if opts.UserID != "" {
					t.Fatalf("preview opts.UserID = %q, want empty", opts.UserID)
				}
				return preview, nil
			},
			finalizeFn: func(_ *session.Session, _ checkoutcore.FinalizeOptions) (*checkoutcore.FinalizeResult, error) {
				t.Fatal("finalize must not be called without --confirm")
				return nil, nil
			},
		}, nil
	}

	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderFrisco, "--format", "json", "checkout", "finalize"})
		err := root.Execute()
		if err == nil {
			t.Fatal("expected finalize without --confirm to abort")
		}
		if !strings.Contains(err.Error(), "--confirm") {
			t.Fatalf("error = %v, want --confirm guard", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if parsed["dryRun"] != true || parsed["aborted"] != true {
		t.Fatalf("unexpected guard output: %v", parsed)
	}
	guard, ok := parsed["guard"].(map[string]any)
	if !ok || guard["requiresConfirm"] != true {
		t.Fatalf("guard = %v, want requiresConfirm=true", parsed["guard"])
	}
	previewOut, ok := parsed["preview"].(map[string]any)
	if !ok || previewOut["cart_id"] != "cart-1" {
		t.Fatalf("preview = %v, want cart_id=cart-1", parsed["preview"])
	}
}

func TestCheckoutPreviewJSONOutputShape(t *testing.T) {
	origLoad := checkoutLoadSession
	origClient := newCheckoutClient
	defer func() {
		checkoutLoadSession = origLoad
		newCheckoutClient = origClient
	}()

	checkoutLoadSession = func(_ *cobra.Command) (string, *session.Session, error) {
		return session.ProviderFrisco, &session.Session{BaseURL: session.DefaultBaseURL, UserID: "u-1"}, nil
	}
	newCheckoutClient = func(provider string) (checkoutCLIClient, error) {
		if provider != session.ProviderFrisco {
			t.Fatalf("factory provider = %q, want %q", provider, session.ProviderFrisco)
		}
		return &fakeCheckoutClient{
			previewFn: func(_ *session.Session, opts checkoutcore.PreviewOptions) (*checkoutcore.CheckoutPreview, error) {
				if opts.Provider != session.ProviderFrisco {
					t.Fatalf("preview opts.Provider = %q, want %q", opts.Provider, session.ProviderFrisco)
				}
				return &checkoutcore.CheckoutPreview{
					Provider:        session.ProviderFrisco,
					UserID:          "u-1",
					CartID:          "cart-1",
					ItemCount:       3,
					ReadyToFinalize: true,
					Reservation:     &checkoutcore.ReservationWindow{StartsAt: "2026-04-21T10:00:00Z", EndsAt: "2026-04-21T12:00:00Z"},
					Payment:         &checkoutcore.PaymentSelection{Method: "CARD", Channel: "Dotpay"},
				}, nil
			},
			finalizeFn: func(_ *session.Session, _ checkoutcore.FinalizeOptions) (*checkoutcore.FinalizeResult, error) {
				return nil, errors.New("unexpected finalize")
			},
		}, nil
	}

	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderFrisco, "--format", "json", "checkout", "preview"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	for _, key := range []string{"provider", "user_id", "cart_id", "item_count", "ready_to_finalize", "reservation", "payment"} {
		if _, ok := parsed[key]; !ok {
			t.Fatalf("missing key %q in output: %v", key, parsed)
		}
	}
}

func TestCheckoutPreviewRoutesDelioProvider(t *testing.T) {
	origLoad := checkoutLoadSession
	origClient := newCheckoutClient
	defer func() {
		checkoutLoadSession = origLoad
		newCheckoutClient = origClient
	}()

	checkoutLoadSession = func(_ *cobra.Command) (string, *session.Session, error) {
		return session.ProviderDelio, &session.Session{BaseURL: session.DefaultDelioBaseURL, UserID: "d-1"}, nil
	}
	newCheckoutClient = func(provider string) (checkoutCLIClient, error) {
		if provider != session.ProviderDelio {
			t.Fatalf("factory provider = %q, want %q", provider, session.ProviderDelio)
		}
		return &fakeCheckoutClient{
			previewFn: func(_ *session.Session, opts checkoutcore.PreviewOptions) (*checkoutcore.CheckoutPreview, error) {
				if opts.Provider != session.ProviderDelio {
					t.Fatalf("preview opts.Provider = %q, want %q", opts.Provider, session.ProviderDelio)
				}
				return &checkoutcore.CheckoutPreview{
					Provider:        session.ProviderDelio,
					UserID:          "d-1",
					CartID:          "delio-cart-1",
					ItemCount:       1,
					ReadyToFinalize: true,
				}, nil
			},
			finalizeFn: func(_ *session.Session, _ checkoutcore.FinalizeOptions) (*checkoutcore.FinalizeResult, error) {
				return nil, errors.New("unexpected finalize")
			},
		}, nil
	}

	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderDelio, "--format", "json", "checkout", "preview"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if parsed["provider"] != session.ProviderDelio {
		t.Fatalf("provider = %v, want %s", parsed["provider"], session.ProviderDelio)
	}
	if parsed["cart_id"] != "delio-cart-1" {
		t.Fatalf("cart_id = %v, want delio-cart-1", parsed["cart_id"])
	}
}

func TestCheckoutFinalizeConfirmReadback(t *testing.T) {
	origLoad := checkoutLoadSession
	origClient := newCheckoutClient
	defer func() {
		checkoutLoadSession = origLoad
		newCheckoutClient = origClient
	}()

	checkoutLoadSession = func(_ *cobra.Command) (string, *session.Session, error) {
		return session.ProviderFrisco, &session.Session{BaseURL: session.DefaultBaseURL, UserID: "u-1"}, nil
	}
	preview := &checkoutcore.CheckoutPreview{
		Provider:        session.ProviderFrisco,
		UserID:          "u-1",
		CartID:          "cart-1",
		ItemCount:       2,
		ReadyToFinalize: true,
		Total:           &checkoutcore.Money{Amount: 42.5, Currency: "PLN"},
	}
	newCheckoutClient = func(provider string) (checkoutCLIClient, error) {
		if provider != session.ProviderFrisco {
			t.Fatalf("factory provider = %q, want %q", provider, session.ProviderFrisco)
		}
		return &fakeCheckoutClient{
			previewFn: func(_ *session.Session, opts checkoutcore.PreviewOptions) (*checkoutcore.CheckoutPreview, error) {
				if opts.Provider != session.ProviderFrisco {
					t.Fatalf("preview opts.Provider = %q, want %q", opts.Provider, session.ProviderFrisco)
				}
				return preview, nil
			},
			finalizeFn: func(_ *session.Session, opts checkoutcore.FinalizeOptions) (*checkoutcore.FinalizeResult, error) {
				if opts.Provider != session.ProviderFrisco {
					t.Fatalf("finalize opts.Provider = %q, want %q", opts.Provider, session.ProviderFrisco)
				}
				if opts.Guard == nil || opts.Guard.ExpectedCartID != "cart-1" {
					t.Fatalf("guard = %+v, want cart-1", opts.Guard)
				}
				if opts.Guard.ExpectedItemCount == nil || *opts.Guard.ExpectedItemCount != 2 {
					t.Fatalf("guard item count = %+v, want 2", opts.Guard)
				}
				if opts.Guard.ExpectedTotal == nil || *opts.Guard.ExpectedTotal != 42.5 {
					t.Fatalf("guard total = %+v, want 42.5", opts.Guard)
				}
				return &checkoutcore.FinalizeResult{
					Provider: session.ProviderFrisco,
					UserID:   "u-1",
					Status:   checkoutcore.FinalizeStatusPlaced,
					OrderID:  "ord-123",
					Preview:  preview,
					Readback: &checkoutcore.OrderReadback{
						OrderID:  "ord-123",
						Order:    map[string]any{"id": "ord-123", "status": "Placed"},
						Payments: []map[string]any{{"status": "Paid"}},
					},
				}, nil
			},
		}, nil
	}

	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderFrisco, "--format", "json", "checkout", "finalize", "--confirm"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if parsed["status"] != string(checkoutcore.FinalizeStatusPlaced) {
		t.Fatalf("status = %v, want %s", parsed["status"], checkoutcore.FinalizeStatusPlaced)
	}
	if parsed["order_id"] != "ord-123" {
		t.Fatalf("order_id = %v, want ord-123", parsed["order_id"])
	}
	readback, ok := parsed["readback"].(map[string]any)
	if !ok {
		t.Fatalf("readback = %T, want object", parsed["readback"])
	}
	if _, ok := readback["order"]; !ok {
		t.Fatalf("readback.order missing: %v", readback)
	}
	if _, ok := readback["payments"]; !ok {
		t.Fatalf("readback.payments missing: %v", readback)
	}
}

func TestCheckoutFinalizeRoutesDelioProvider(t *testing.T) {
	origLoad := checkoutLoadSession
	origClient := newCheckoutClient
	defer func() {
		checkoutLoadSession = origLoad
		newCheckoutClient = origClient
	}()

	checkoutLoadSession = func(_ *cobra.Command) (string, *session.Session, error) {
		return session.ProviderDelio, &session.Session{BaseURL: session.DefaultDelioBaseURL, UserID: "d-1"}, nil
	}
	preview := &checkoutcore.CheckoutPreview{
		Provider:        session.ProviderDelio,
		UserID:          "d-1",
		CartID:          "delio-cart-1",
		ItemCount:       1,
		ReadyToFinalize: true,
		Total:           &checkoutcore.Money{Amount: 18.25, Currency: "PLN"},
	}
	newCheckoutClient = func(provider string) (checkoutCLIClient, error) {
		if provider != session.ProviderDelio {
			t.Fatalf("factory provider = %q, want %q", provider, session.ProviderDelio)
		}
		return &fakeCheckoutClient{
			previewFn: func(_ *session.Session, opts checkoutcore.PreviewOptions) (*checkoutcore.CheckoutPreview, error) {
				if opts.Provider != session.ProviderDelio {
					t.Fatalf("preview opts.Provider = %q, want %q", opts.Provider, session.ProviderDelio)
				}
				if opts.UserID != "" {
					t.Fatalf("preview opts.UserID = %q, want empty", opts.UserID)
				}
				return preview, nil
			},
			finalizeFn: func(_ *session.Session, opts checkoutcore.FinalizeOptions) (*checkoutcore.FinalizeResult, error) {
				if opts.Provider != session.ProviderDelio {
					t.Fatalf("finalize opts.Provider = %q, want %q", opts.Provider, session.ProviderDelio)
				}
				if opts.Guard == nil || opts.Guard.ExpectedCartID != "delio-cart-1" {
					t.Fatalf("guard = %+v, want delio-cart-1", opts.Guard)
				}
				if opts.Guard.ExpectedItemCount == nil || *opts.Guard.ExpectedItemCount != 1 {
					t.Fatalf("guard item count = %+v, want 1", opts.Guard)
				}
				if opts.Guard.ExpectedTotal == nil || *opts.Guard.ExpectedTotal != 18.25 {
					t.Fatalf("guard total = %+v, want 18.25", opts.Guard)
				}
				return &checkoutcore.FinalizeResult{
					Provider: session.ProviderDelio,
					UserID:   "d-1",
					Status:   checkoutcore.FinalizeStatusPending,
					OrderID:  "delio-ord-123",
					Preview:  preview,
				}, nil
			},
		}, nil
	}

	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderDelio, "--format", "json", "checkout", "finalize", "--confirm"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if parsed["provider"] != session.ProviderDelio {
		t.Fatalf("provider = %v, want %s", parsed["provider"], session.ProviderDelio)
	}
	if parsed["status"] != string(checkoutcore.FinalizeStatusPending) {
		t.Fatalf("status = %v, want %s", parsed["status"], checkoutcore.FinalizeStatusPending)
	}
	if parsed["order_id"] != "delio-ord-123" {
		t.Fatalf("order_id = %v, want delio-ord-123", parsed["order_id"])
	}
}

func TestCheckoutFinalizeActionRequiredPrintsStructuredResult(t *testing.T) {
	origLoad := checkoutLoadSession
	origClient := newCheckoutClient
	defer func() {
		checkoutLoadSession = origLoad
		newCheckoutClient = origClient
	}()

	checkoutLoadSession = func(_ *cobra.Command) (string, *session.Session, error) {
		return session.ProviderFrisco, &session.Session{BaseURL: session.DefaultBaseURL, UserID: "u-1"}, nil
	}
	preview := &checkoutcore.CheckoutPreview{Provider: session.ProviderFrisco, UserID: "u-1", CartID: "cart-1", ItemCount: 1}
	result := &checkoutcore.FinalizeResult{
		Provider: session.ProviderFrisco,
		UserID:   "u-1",
		Status:   checkoutcore.FinalizeStatusRequiresAction,
		OrderID:  "ord-3ds",
		Preview:  preview,
		Action: &checkoutcore.PaymentAction{
			Kind:   checkoutcore.PaymentActionKindRedirect,
			URL:    "https://bank.example/3ds",
			Method: "GET",
		},
	}
	newCheckoutClient = func(provider string) (checkoutCLIClient, error) {
		if provider != session.ProviderFrisco {
			t.Fatalf("factory provider = %q, want %q", provider, session.ProviderFrisco)
		}
		return &fakeCheckoutClient{
			previewFn: func(_ *session.Session, opts checkoutcore.PreviewOptions) (*checkoutcore.CheckoutPreview, error) {
				if opts.Provider != session.ProviderFrisco {
					t.Fatalf("preview opts.Provider = %q, want %q", opts.Provider, session.ProviderFrisco)
				}
				return preview, nil
			},
			finalizeFn: func(_ *session.Session, opts checkoutcore.FinalizeOptions) (*checkoutcore.FinalizeResult, error) {
				if opts.Provider != session.ProviderFrisco {
					t.Fatalf("finalize opts.Provider = %q, want %q", opts.Provider, session.ProviderFrisco)
				}
				return result, &checkoutcore.ActionRequiredError{Action: result.Action, Result: result}
			},
		}, nil
	}

	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderFrisco, "--format", "json", "checkout", "finalize", "--confirm"})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if parsed["status"] != string(checkoutcore.FinalizeStatusRequiresAction) {
		t.Fatalf("status = %v, want requires_action", parsed["status"])
	}
	action, ok := parsed["action"].(map[string]any)
	if !ok || action["url"] != "https://bank.example/3ds" {
		t.Fatalf("action = %v, want redirect url", parsed["action"])
	}
}

func TestFinalizeGuardFromPreview(t *testing.T) {
	preview := &checkoutcore.CheckoutPreview{
		CartID:    "cart-1",
		ItemCount: 4,
		Total:     &checkoutcore.Money{Amount: 99.9},
	}
	guard := finalizeGuardFromPreview(preview)
	if guard == nil || guard.ExpectedCartID != "cart-1" {
		t.Fatalf("guard = %+v", guard)
	}
	if guard.ExpectedItemCount == nil || *guard.ExpectedItemCount != 4 {
		t.Fatalf("item count guard = %+v", guard)
	}
	if guard.ExpectedTotal == nil || *guard.ExpectedTotal != 99.9 {
		t.Fatalf("total guard = %+v", guard)
	}
}

func checkoutSubcommandNames(cmd *cobra.Command) []string {
	names := make([]string, 0, len(cmd.Commands()))
	for _, subcmd := range cmd.Commands() {
		names = append(names, subcmd.Name())
	}
	sort.Strings(names)
	return names
}

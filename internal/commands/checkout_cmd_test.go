package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
)

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
	origReq := checkoutRequestJSON
	origAddr := checkoutGetShippingAddr
	defer func() {
		checkoutLoadSession = origLoad
		checkoutRequestJSON = origReq
		checkoutGetShippingAddr = origAddr
	}()

	checkoutLoadSession = func(_ *cobra.Command, supported ...string) (string, *session.Session, error) {
		if len(supported) != 1 || supported[0] != session.ProviderFrisco {
			t.Fatalf("supported = %v", supported)
		}
		return session.ProviderFrisco, &session.Session{BaseURL: session.DefaultBaseURL, UserID: "u-1"}, nil
	}
	checkoutGetShippingAddr = func(_ *session.Session, userID string) (map[string]any, error) {
		return map[string]any{"city": "Warsaw", "recipient": "Test"}, nil
	}
	checkoutRequestJSON = func(_ *session.Session, method, path string, opts httpclient.RequestOpts) (any, error) {
		switch {
		case method == "GET" && strings.HasSuffix(path, "/cart"):
			return map[string]any{"products": []any{map[string]any{"productId": "P1", "quantity": float64(2)}}, "total": 12.34}, nil
		case method == "GET" && strings.HasSuffix(path, "/cart/reservation"):
			return map[string]any{"startsAt": "2026-04-21T10:00:00Z", "endsAt": "2026-04-21T12:00:00Z"}, nil
		case method == "GET" && strings.HasSuffix(path, "/payments"):
			return map[string]any{"items": []any{map[string]any{"channel": "card"}}}, nil
		case method == "POST" && strings.Contains(path, "/cart/checkout/preview"):
			return map[string]any{"ready": true, "warnings": []any{}}, nil
		default:
			t.Fatalf("unexpected request %s %s", method, path)
			return nil, nil
		}
	}

	payloadFile := writeTempJSON(t, map[string]any{"paymentMethodId": "pm-1"})
	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderFrisco, "--format", "json", "checkout", "finalize", "--payload-file", payloadFile})
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
	if parsed["dryRun"] != true {
		t.Fatalf("dryRun = %v, want true", parsed["dryRun"])
	}
	if parsed["aborted"] != true {
		t.Fatalf("aborted = %v, want true", parsed["aborted"])
	}
	guard, ok := parsed["guard"].(map[string]any)
	if !ok || guard["requiresConfirm"] != true {
		t.Fatalf("guard = %v, want requiresConfirm=true", parsed["guard"])
	}
	preview, ok := parsed["preview"].(map[string]any)
	if !ok {
		t.Fatalf("preview = %T, want object", parsed["preview"])
	}
	if preview["mode"] != "preview" {
		t.Fatalf("preview.mode = %v, want preview", preview["mode"])
	}
}

func TestCheckoutPreviewJSONOutputShape(t *testing.T) {
	origLoad := checkoutLoadSession
	origReq := checkoutRequestJSON
	origAddr := checkoutGetShippingAddr
	defer func() {
		checkoutLoadSession = origLoad
		checkoutRequestJSON = origReq
		checkoutGetShippingAddr = origAddr
	}()

	checkoutLoadSession = func(_ *cobra.Command, _ ...string) (string, *session.Session, error) {
		return session.ProviderFrisco, &session.Session{BaseURL: session.DefaultBaseURL, UserID: "u-1"}, nil
	}
	checkoutGetShippingAddr = func(_ *session.Session, userID string) (map[string]any, error) {
		return map[string]any{"city": "Warsaw", "recipient": "Ada"}, nil
	}
	checkoutRequestJSON = func(_ *session.Session, method, path string, _ httpclient.RequestOpts) (any, error) {
		switch {
		case method == "GET" && strings.HasSuffix(path, "/cart"):
			return map[string]any{"products": []any{map[string]any{"productId": "P1", "quantity": float64(3)}}}, nil
		case method == "GET" && strings.HasSuffix(path, "/cart/reservation"):
			return map[string]any{"warehouse": "WA1"}, nil
		case method == "GET" && strings.HasSuffix(path, "/payments"):
			return map[string]any{"items": []any{map[string]any{"channel": "visa"}}}, nil
		case method == "POST" && strings.Contains(path, "/cart/checkout/preview"):
			return map[string]any{"status": "ok", "totals": map[string]any{"grandTotal": 49.99}}, nil
		default:
			return nil, errors.New("unexpected request")
		}
	}

	payloadFile := writeTempJSON(t, map[string]any{"paymentMethodId": "pm-1"})
	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderFrisco, "--format", "json", "checkout", "preview", "--payload-file", payloadFile})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	for _, key := range []string{"mode", "provider", "userId", "cart", "cartSummary", "checkoutPreview", "shippingAddress", "accountPayments"} {
		if _, ok := parsed[key]; !ok {
			t.Fatalf("missing key %q in output: %v", key, parsed)
		}
	}
	if parsed["mode"] != "preview" {
		t.Fatalf("mode = %v, want preview", parsed["mode"])
	}
}

func TestCheckoutFinalizeConfirmReadback(t *testing.T) {
	origLoad := checkoutLoadSession
	origReq := checkoutRequestJSON
	origAddr := checkoutGetShippingAddr
	defer func() {
		checkoutLoadSession = origLoad
		checkoutRequestJSON = origReq
		checkoutGetShippingAddr = origAddr
	}()

	checkoutLoadSession = func(_ *cobra.Command, _ ...string) (string, *session.Session, error) {
		return session.ProviderFrisco, &session.Session{BaseURL: session.DefaultBaseURL, UserID: "u-1"}, nil
	}
	checkoutGetShippingAddr = func(_ *session.Session, userID string) (map[string]any, error) {
		return map[string]any{"city": "Warsaw"}, nil
	}
	ordersCalls := 0
	checkoutRequestJSON = func(_ *session.Session, method, path string, opts httpclient.RequestOpts) (any, error) {
		switch {
		case method == "GET" && strings.HasSuffix(path, "/orders"):
			ordersCalls++
			if ordersCalls == 1 {
				return map[string]any{"orders": []any{map[string]any{"orderId": "OLD-1"}}}, nil
			}
			return map[string]any{"orders": []any{map[string]any{"orderId": "NEW-1"}, map[string]any{"orderId": "OLD-1"}}}, nil
		case method == "POST" && strings.Contains(path, "/cart/checkout/finalize"):
			if opts.Data == nil {
				t.Fatal("expected finalize payload to be forwarded")
			}
			return map[string]any{"status": "placed", "orderId": "NEW-1"}, nil
		case method == "GET" && strings.HasSuffix(path, "/orders/NEW-1"):
			return map[string]any{"orderId": "NEW-1", "status": "Placed"}, nil
		case method == "GET" && strings.HasSuffix(path, "/orders/NEW-1/delivery"):
			return map[string]any{"slot": "10:00-12:00"}, nil
		case method == "GET" && strings.HasSuffix(path, "/orders/NEW-1/payments"):
			return map[string]any{"status": "authorized"}, nil
		case method == "GET" && strings.HasSuffix(path, "/cart/reservation"):
			return nil, &httpclient.ErrorDetails{Status: 404, Reason: "Not Found"}
		default:
			return nil, fmt.Errorf("unexpected request %s %s", method, path)
		}
	}

	payloadFile := writeTempJSON(t, map[string]any{"paymentMethodId": "pm-1"})
	out := captureStdout(func() {
		root := NewRootCmd()
		root.SetArgs([]string{"--provider", session.ProviderFrisco, "--format", "json", "checkout", "finalize", "--confirm", "--payload-file", payloadFile})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if parsed["finalized"] != true {
		t.Fatalf("finalized = %v, want true", parsed["finalized"])
	}
	if parsed["orderId"] != "NEW-1" {
		t.Fatalf("orderId = %v, want NEW-1", parsed["orderId"])
	}
	readback, ok := parsed["readback"].(map[string]any)
	if !ok {
		t.Fatalf("readback = %T, want object", parsed["readback"])
	}
	if _, ok := readback["order"]; !ok {
		t.Fatalf("readback.order missing: %v", readback)
	}
	if _, ok := readback["delivery"]; !ok {
		t.Fatalf("readback.delivery missing: %v", readback)
	}
	if _, ok := readback["payments"]; !ok {
		t.Fatalf("readback.payments missing: %v", readback)
	}
}

func TestExtractCheckoutHelpers(t *testing.T) {
	if got := extractCheckoutOrderID(map[string]any{"order": map[string]any{"orderId": "ORD-1"}}); got != "ORD-1" {
		t.Fatalf("extractCheckoutOrderID = %q, want ORD-1", got)
	}
	redirect := extractCheckoutRedirect(map[string]any{"redirectUrl": "https://bank.example/3ds", "method": "POST"})
	if redirect == nil || redirect["url"] != "https://bank.example/3ds" {
		t.Fatalf("redirect = %v", redirect)
	}
	before := []map[string]any{{"orderId": "OLD-1"}}
	after := []map[string]any{{"orderId": "NEW-1"}, {"orderId": "OLD-1"}}
	if got := detectNewOrderID(before, after); got != "NEW-1" {
		t.Fatalf("detectNewOrderID = %q, want NEW-1", got)
	}
}

func writeTempJSON(t *testing.T, v any) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.json")
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

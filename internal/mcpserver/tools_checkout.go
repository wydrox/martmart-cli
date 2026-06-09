package mcpserver

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	checkoutcore "github.com/wydrox/martmart-cli/internal/checkout"
	"github.com/wydrox/martmart-cli/internal/session"
)

type mcpCheckoutClient interface {
	Preview(s *session.Session, opts checkoutcore.PreviewOptions) (*checkoutcore.CheckoutPreview, error)
	Finalize(s *session.Session, opts checkoutcore.FinalizeOptions) (*checkoutcore.FinalizeResult, error)
}

var newMCPCheckoutClient = func(provider string) (mcpCheckoutClient, error) {
	provider = session.NormalizeProvider(provider)
	if provider == session.ProviderUpMenu {
		return nil, &checkoutcore.UnsupportedProviderError{Provider: provider, Supported: []string{session.ProviderFrisco, session.ProviderDelio}}
	}
	return checkoutcore.NewClient(provider), nil
}

func registerCheckoutTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "checkout_preview",
		Description: "Preview checkout for Frisco or Delio without finalizing. Returns normalized totals, reservation, payment state, readiness, issues, and raw provider data.",
	}, toolCheckoutPreview)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "checkout_finalize",
		Description: "Guarded headless checkout finalization for Frisco or Delio. Requires confirm=true; otherwise returns a dry-run preview and suggested guard. May return requires_action for redirect/3DS handoff.",
	}, toolCheckoutFinalize)
}

type checkoutPreviewIn struct {
	Provider string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID   string `json:"user_id,omitempty" jsonschema:"optional provider user id override; defaults to session user_id"`
}

type checkoutPreviewOut struct {
	Preview *checkoutcore.CheckoutPreview `json:"preview" jsonschema:"normalized checkout preview"`
}

type checkoutFinalizeIn struct {
	Provider            string   `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID              string   `json:"user_id,omitempty" jsonschema:"optional provider user id override; defaults to session user_id"`
	Confirm             bool     `json:"confirm,omitempty" jsonschema:"must be true to send the finalization request; false returns dry-run preview only"`
	ExpectedCartID      string   `json:"expected_cart_id,omitempty" jsonschema:"guard from a previously approved preview"`
	ExpectedItemCount   *int     `json:"expected_item_count,omitempty" jsonschema:"guard from a previously approved preview"`
	ExpectedTotal       *float64 `json:"expected_total,omitempty" jsonschema:"guard from a previously approved preview"`
	AllowActionRequired bool     `json:"allow_action_required,omitempty" jsonschema:"when true, return requires_action results instead of surfacing an action-required error"`
}

type checkoutFinalizeOut struct {
	DryRun  bool                          `json:"dry_run,omitempty"`
	Guard   *checkoutcore.FinalizeGuard   `json:"guard,omitempty"`
	Preview *checkoutcore.CheckoutPreview `json:"preview,omitempty"`
	Result  *checkoutcore.FinalizeResult  `json:"result,omitempty"`
}

func toolCheckoutPreview(_ context.Context, _ *mcp.CallToolRequest, in checkoutPreviewIn) (*mcp.CallToolResult, checkoutPreviewOut, error) {
	provider, s, err := loadSessionOnlyAuth(in.Provider)
	if err != nil {
		return nil, checkoutPreviewOut{}, err
	}
	client, err := newMCPCheckoutClient(provider)
	if err != nil {
		return nil, checkoutPreviewOut{}, err
	}
	preview, err := client.Preview(s, checkoutcore.PreviewOptions{Provider: provider, UserID: in.UserID})
	if err != nil {
		return nil, checkoutPreviewOut{}, err
	}
	if preview != nil {
		mcpIngestCartBestEffort(provider, preview.Raw)
	}
	return nil, checkoutPreviewOut{Preview: preview}, nil
}

func toolCheckoutFinalize(_ context.Context, _ *mcp.CallToolRequest, in checkoutFinalizeIn) (*mcp.CallToolResult, checkoutFinalizeOut, error) {
	provider, s, err := loadSessionOnlyAuth(in.Provider)
	if err != nil {
		return nil, checkoutFinalizeOut{}, err
	}
	client, err := newMCPCheckoutClient(provider)
	if err != nil {
		return nil, checkoutFinalizeOut{}, err
	}
	preview, err := client.Preview(s, checkoutcore.PreviewOptions{Provider: provider, UserID: in.UserID})
	if err != nil {
		return nil, checkoutFinalizeOut{}, err
	}
	if preview != nil {
		mcpIngestCartBestEffort(provider, preview.Raw)
	}
	guard := checkoutGuardFromFinalizeInput(in)
	if guard == nil {
		guard = checkoutGuardFromPreview(preview)
	}
	if !in.Confirm {
		return nil, checkoutFinalizeOut{DryRun: true, Guard: guard, Preview: preview}, nil
	}
	result, err := client.Finalize(s, checkoutcore.FinalizeOptions{
		Provider:            provider,
		UserID:              in.UserID,
		Guard:               guard,
		AllowActionRequired: in.AllowActionRequired,
	})
	if err != nil {
		var actionErr *checkoutcore.ActionRequiredError
		if errors.As(err, &actionErr) && actionErr.Result != nil {
			return nil, checkoutFinalizeOut{Result: actionErr.Result}, nil
		}
		return nil, checkoutFinalizeOut{}, err
	}
	return nil, checkoutFinalizeOut{Result: result}, nil
}

func checkoutGuardFromFinalizeInput(in checkoutFinalizeIn) *checkoutcore.FinalizeGuard {
	if in.ExpectedCartID == "" && in.ExpectedItemCount == nil && in.ExpectedTotal == nil {
		return nil
	}
	return &checkoutcore.FinalizeGuard{
		ExpectedCartID:    in.ExpectedCartID,
		ExpectedItemCount: in.ExpectedItemCount,
		ExpectedTotal:     in.ExpectedTotal,
	}
}

func checkoutGuardFromPreview(preview *checkoutcore.CheckoutPreview) *checkoutcore.FinalizeGuard {
	if preview == nil {
		return nil
	}
	guard := &checkoutcore.FinalizeGuard{
		ExpectedCartID:    preview.CartID,
		ExpectedItemCount: &preview.ItemCount,
	}
	if preview.Total != nil {
		total := preview.Total.Amount
		guard.ExpectedTotal = &total
	}
	return guard
}

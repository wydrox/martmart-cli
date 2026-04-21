package commands

import (
	"errors"

	"github.com/spf13/cobra"

	checkoutcore "github.com/wydrox/martmart-cli/internal/checkout"
	"github.com/wydrox/martmart-cli/internal/session"
)

type checkoutCLIClient interface {
	Preview(s *session.Session, opts checkoutcore.PreviewOptions) (*checkoutcore.CheckoutPreview, error)
	Finalize(s *session.Session, opts checkoutcore.FinalizeOptions) (*checkoutcore.FinalizeResult, error)
}

var checkoutLoadSession = loadSessionForRequest
var newCheckoutClient = func(provider string) (checkoutCLIClient, error) {
	switch session.NormalizeProvider(provider) {
	case session.ProviderFrisco:
		return checkoutcore.NewFriscoClient(), nil
	default:
		return nil, &checkoutcore.UnsupportedProviderError{Provider: provider, Supported: []string{session.ProviderFrisco}}
	}
}

func newCheckoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkout",
		Short: "Checkout preview and finalization.",
	}
	cmd.AddCommand(
		newCheckoutPreviewCmd(),
		newCheckoutFinalizeCmd(),
	)
	return cmd
}

func newCheckoutPreviewCmd() *cobra.Command {
	var userID string
	c := &cobra.Command{
		Use:   "preview",
		Short: "Preview provider checkout.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, s, err := checkoutLoadSession(cmd)
			if err != nil {
				return err
			}
			client, err := newCheckoutClient(provider)
			if err != nil {
				return err
			}
			preview, err := client.Preview(s, checkoutcore.PreviewOptions{Provider: provider, UserID: userID})
			if err != nil {
				return err
			}
			return printJSON(preview)
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func newCheckoutFinalizeCmd() *cobra.Command {
	var userID string
	var confirm bool
	c := &cobra.Command{
		Use:   "finalize",
		Short: "Finalize provider checkout. Requires explicit --confirm.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, s, err := checkoutLoadSession(cmd)
			if err != nil {
				return err
			}
			client, err := newCheckoutClient(provider)
			if err != nil {
				return err
			}
			preview, err := client.Preview(s, checkoutcore.PreviewOptions{Provider: provider, UserID: userID})
			if err != nil {
				return err
			}
			if !confirm {
				if err := printJSON(map[string]any{
					"aborted": true,
					"dryRun":  true,
					"guard": map[string]any{
						"requiresConfirm": true,
						"message":         "checkout finalize requires explicit --confirm; preview shown and no finalization request was sent",
					},
					"preview": preview,
				}); err != nil {
					return err
				}
				return errors.New("checkout finalize aborted: rerun with --confirm to submit the finalization request")
			}

			result, err := client.Finalize(s, checkoutcore.FinalizeOptions{
				Provider: provider,
				UserID:   userID,
				Guard:    finalizeGuardFromPreview(preview),
			})
			if err != nil {
				var actionErr *checkoutcore.ActionRequiredError
				if errors.As(err, &actionErr) && actionErr.Result != nil {
					return printJSON(actionErr.Result)
				}
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&userID, "user-id", "", "")
	c.Flags().BoolVar(&confirm, "confirm", false, "Actually send the finalization request. Without this flag the command prints a dry-run preview and aborts.")
	return c
}

func finalizeGuardFromPreview(preview *checkoutcore.CheckoutPreview) *checkoutcore.FinalizeGuard {
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

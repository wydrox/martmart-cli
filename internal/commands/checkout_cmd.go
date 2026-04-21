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

var checkoutLoadSession = loadSessionForSupportedProviders
var newCheckoutClient = func() checkoutCLIClient { return checkoutcore.NewFriscoClient() }

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
		Short: "Preview Frisco express checkout.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, s, err := checkoutLoadSession(cmd, session.ProviderFrisco)
			if err != nil {
				return err
			}
			preview, err := newCheckoutClient().Preview(s, checkoutcore.PreviewOptions{UserID: userID})
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
		Short: "Finalize Frisco express checkout. Requires explicit --confirm.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, s, err := checkoutLoadSession(cmd, session.ProviderFrisco)
			if err != nil {
				return err
			}
			client := newCheckoutClient()
			preview, err := client.Preview(s, checkoutcore.PreviewOptions{UserID: userID})
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
				UserID: userID,
				Guard:  finalizeGuardFromPreview(preview),
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

// Package commands implements the MartMart CLI command tree.
package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/config"
	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
)

// Execute runs the root command (for main).
func Execute() error {
	return NewRootCmd().Execute()
}

// NewRootCmd builds the full CLI command tree.
func NewRootCmd() *cobra.Command {
	format := outputFormat
	rateLimitRPS := 0.0
	rateLimitBurst := 1
	root := &cobra.Command{
		Use:   "martmart",
		Short: "MartMart CLI — shared grocery CLI for Frisco.pl and Delio.",
		Long:  "Session management, product search, cart, reservations and account operations across multiple grocery providers.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := withResolvedProvider(cmd); err != nil {
				return err
			}

			format = strings.ToLower(strings.TrimSpace(format))
			if format == "" {
				format = "table"
			}
			if format != "table" && format != "json" {
				return fmt.Errorf(
					"unsupported --format: %s (use table or json)",
					format,
				)
			}
			outputFormat = format

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			effectiveRPS := cfg.RateLimitRPS
			if cmd.Flags().Changed("rate-limit-rps") {
				effectiveRPS = rateLimitRPS
			}
			effectiveBurst := cfg.RateLimitBurst
			if cmd.Flags().Changed("rate-limit-burst") {
				effectiveBurst = rateLimitBurst
			}
			if effectiveBurst < 1 {
				effectiveBurst = 1
			}
			httpclient.SetRateLimit(effectiveRPS, effectiveBurst)
			return nil
		},
	}
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().StringVar(
		&format,
		"format",
		"table",
		"Output format: table or json.",
	)
	root.PersistentFlags().String(
		"provider",
		"",
		fmt.Sprintf("Provider for this command invocation: %s.", session.SupportedProvidersFlagHelp()),
	)
	root.PersistentFlags().Float64Var(
		&rateLimitRPS,
		"rate-limit-rps",
		0,
		"Request rate limit in requests/second (0 = disabled, default comes from config).",
	)
	root.PersistentFlags().IntVar(
		&rateLimitBurst,
		"rate-limit-burst",
		1,
		"Request burst size (default comes from config).",
	)

	root.AddCommand(
		newSessionCmd(),
		newProductsCmd(),
		newCartCmd(),
		newCheckoutCmd(),
		newReservationCmd(),
		newAccountCmd(),
		newMCPCmd(),
		newSetupCmd(),
		newConfigCmd(),
		newVoiceCmd(),
	)
	return root
}

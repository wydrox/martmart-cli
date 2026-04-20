package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/config"
	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or persist shared CLI settings.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runConfigShow()
		},
	}
	cmd.AddCommand(newConfigShowCmd(), newConfigSetCmd())
	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show saved config and currently active provider/rate limit.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runConfigShow()
		},
	}
}

func runConfigShow() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	rps, burst := httpclient.CurrentRateLimit()
	return printJSON(map[string]any{
		"config_file":         config.Path(),
		"default_provider":    cfg.DefaultProvider,
		"saved_rate_limit":    map[string]any{"rps": cfg.RateLimitRPS, "burst": cfg.RateLimitBurst},
		"active_provider":     session.CurrentProvider(),
		"active_rate_limit":   map[string]any{"rps": rps, "burst": burst},
		"supported_providers": session.SupportedProviders(),
	})
}

func newConfigSetCmd() *cobra.Command {
	var (
		defaultProvider string
		rateLimitRPS    float64
		rateLimitBurst  int
	)
	c := &cobra.Command{
		Use:   "set",
		Short: "Persist default provider and/or rate limiter settings.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			changed := false
			if cmd.Flags().Changed("default-provider") {
				provider := session.NormalizeProvider(defaultProvider)
				if err := session.ValidateProvider(provider); err != nil {
					return err
				}
				cfg.DefaultProvider = provider
				changed = true
			}
			if cmd.Flags().Changed("rate-limit-rps") {
				if rateLimitRPS < 0 {
					return fmt.Errorf("--rate-limit-rps must be >= 0")
				}
				cfg.RateLimitRPS = rateLimitRPS
				changed = true
			}
			if cmd.Flags().Changed("rate-limit-burst") {
				if rateLimitBurst < 1 {
					return fmt.Errorf("--rate-limit-burst must be >= 1")
				}
				cfg.RateLimitBurst = rateLimitBurst
				changed = true
			}
			if !changed {
				return errors.New("nothing to save: set at least one flag")
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			return runConfigShow()
		},
	}
	c.Flags().StringVar(&defaultProvider, "default-provider", "", "Default provider: frisco or delio.")
	c.Flags().Float64Var(&rateLimitRPS, "rate-limit-rps", 0, "Saved request rate in requests/second (0 = disabled).")
	c.Flags().IntVar(&rateLimitBurst, "rate-limit-burst", 1, "Saved request burst size.")
	return c
}

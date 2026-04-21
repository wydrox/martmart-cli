package commands

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/config"
	"github.com/wydrox/martmart-cli/internal/session"
)

// selectedProvider resolves the provider for the current command invocation.
// It prefers an explicit --provider flag and falls back to the saved config
// default provider, then Frisco.
func selectedProvider(cmd *cobra.Command) (string, error) {
	provider := ""
	if cmd != nil {
		if v, err := cmd.Flags().GetString("provider"); err == nil {
			provider = strings.TrimSpace(v)
		}
	}
	if provider == "" {
		cfg, err := config.Load()
		if err != nil {
			return "", err
		}
		provider = strings.TrimSpace(cfg.DefaultProvider)
	}
	provider = session.NormalizeProvider(provider)
	if provider == "" {
		provider = session.ProviderFrisco
	}
	if err := session.ValidateProvider(provider); err != nil {
		return "", err
	}
	return provider, nil
}

// loadSessionForRequest resolves the provider for the current command and loads
// the matching provider-specific session.
func loadSessionForRequest(cmd *cobra.Command) (string, *session.Session, error) {
	provider, err := selectedProvider(cmd)
	if err != nil {
		return "", nil, err
	}
	s, err := session.LoadProvider(provider)
	if err != nil {
		return "", nil, err
	}
	return provider, s, nil
}

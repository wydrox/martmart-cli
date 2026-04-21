package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/config"
	"github.com/wydrox/martmart-cli/internal/session"
)

type resolvedProviderContextKey struct{}

// selectedProvider resolves the provider for the current command invocation.
// It prefers a provider already attached to the command context, then an
// explicit --provider flag, then the saved config fallback, then Frisco.
func selectedProvider(cmd *cobra.Command) (string, error) {
	if cmd != nil {
		if v, ok := cmd.Context().Value(resolvedProviderContextKey{}).(string); ok {
			provider := session.NormalizeProvider(strings.TrimSpace(v))
			if err := session.ValidateProvider(provider); err != nil {
				return "", err
			}
			return provider, nil
		}
	}

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

func withResolvedProvider(cmd *cobra.Command) error {
	if cmd == nil {
		return nil
	}
	provider, err := selectedProvider(cmd)
	if err != nil {
		return err
	}
	cmd.SetContext(context.WithValue(cmd.Context(), resolvedProviderContextKey{}, provider))
	return nil
}

func unsupportedProviderError(cmd *cobra.Command, provider string, supported ...string) error {
	if len(supported) == 0 {
		return fmt.Errorf("%s does not support provider %q", cmd.CommandPath(), provider)
	}
	return fmt.Errorf(
		"%s does not support provider %q; use --provider %s",
		cmd.CommandPath(),
		provider,
		strings.Join(supported, " or --provider "),
	)
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

func loadSessionForSupportedProviders(cmd *cobra.Command, supported ...string) (string, *session.Session, error) {
	provider, s, err := loadSessionForRequest(cmd)
	if err != nil {
		return "", nil, err
	}
	if len(supported) == 0 {
		return provider, s, nil
	}
	for _, candidate := range supported {
		if provider == session.NormalizeProvider(candidate) {
			return provider, s, nil
		}
	}
	return "", nil, unsupportedProviderError(cmd, provider, supported...)
}

package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/session"
)

type resolvedProviderContextKey struct{}

func providerRequiredError() error {
	return fmt.Errorf("provider is required; pass --provider %s or --provider %s", session.ProviderFrisco, session.ProviderDelio)
}

func explicitProvider(cmd *cobra.Command) (string, bool, error) {
	if cmd != nil {
		if v, ok := cmd.Context().Value(resolvedProviderContextKey{}).(string); ok {
			provider := session.NormalizeProvider(strings.TrimSpace(v))
			if provider == "" {
				return "", false, nil
			}
			if err := session.ValidateProvider(provider); err != nil {
				return "", false, err
			}
			return provider, true, nil
		}
	}

	if cmd != nil {
		if v, err := cmd.Flags().GetString("provider"); err == nil {
			provider := session.NormalizeProvider(strings.TrimSpace(v))
			if provider == "" {
				return "", false, nil
			}
			if err := session.ValidateProvider(provider); err != nil {
				return "", false, err
			}
			return provider, true, nil
		}
	}

	return "", false, nil
}

// selectedProvider resolves the provider for the current command invocation.
// The provider must be passed explicitly; MartMart does not infer it automatically.
func selectedProvider(cmd *cobra.Command) (string, error) {
	provider, ok, err := explicitProvider(cmd)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", providerRequiredError()
	}
	return provider, nil
}

func withResolvedProvider(cmd *cobra.Command) error {
	if cmd == nil {
		return nil
	}
	provider, ok, err := explicitProvider(cmd)
	if err != nil {
		return err
	}
	if !ok {
		return nil
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

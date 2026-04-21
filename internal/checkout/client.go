package checkout

import "github.com/wydrox/martmart-cli/internal/session"

// Client is the provider-aware checkout interface shared by Frisco and Delio.
type Client interface {
	Preview(s *session.Session, opts PreviewOptions) (*CheckoutPreview, error)
	Finalize(s *session.Session, opts FinalizeOptions) (*FinalizeResult, error)
}

// newDelioClient is wired by Delio checkout support when available.
var newDelioClient func() Client

// NewClient returns a provider-aware checkout client.
//
// Unknown or empty providers fall back to the existing Frisco implementation so
// the returned client is always usable; provider validation still happens in the
// concrete client methods.
func NewClient(provider string) Client {
	switch session.NormalizeProvider(provider) {
	case session.ProviderDelio:
		if newDelioClient != nil {
			return newDelioClient()
		}
		return NewFriscoClient()
	default:
		return NewFriscoClient()
	}
}

// NewClientForSession selects a checkout client from the session when possible.
func NewClientForSession(s *session.Session, fallbackProvider string) Client {
	return NewClient(session.ProviderForSession(s, fallbackProvider))
}

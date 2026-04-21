package checkout

import (
	"fmt"
	"strings"
)

// UnsupportedProviderError reports that the selected provider is not supported by the requested checkout flow.
type UnsupportedProviderError struct {
	Provider  string
	Supported []string
}

func (e *UnsupportedProviderError) Error() string {
	if e == nil {
		return ""
	}
	supported := strings.Join(e.Supported, ", ")
	if supported == "" {
		supported = "frisco"
	}
	if strings.TrimSpace(e.Provider) == "" {
		return fmt.Sprintf("checkout is supported only for: %s", supported)
	}
	return fmt.Sprintf("checkout does not support provider %q; supported providers: %s", e.Provider, supported)
}

// GuardMismatchError reports that finalize guards did not match the fresh preview.
type GuardMismatchError struct {
	Field string
	Want  string
	Got   string
}

func (e *GuardMismatchError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("checkout finalize guard mismatch for %s: want %s, got %s", e.Field, e.Want, e.Got)
}

// MalformedResponseError reports that the provider returned an unexpected shape.
type MalformedResponseError struct {
	Operation string
	Message   string
}

func (e *MalformedResponseError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Operation) == "" {
		return e.Message
	}
	return fmt.Sprintf("checkout %s response malformed: %s", e.Operation, e.Message)
}

// ActionRequiredError reports a guarded stop when payment needs redirect/3DS.
type ActionRequiredError struct {
	Action *PaymentAction
	Result *FinalizeResult
}

func (e *ActionRequiredError) Error() string {
	if e == nil || e.Action == nil {
		return "checkout requires additional user action"
	}
	parts := []string{"checkout requires additional user action", string(e.Action.Kind)}
	if e.Action.URL != "" {
		parts = append(parts, e.Action.URL)
	}
	return strings.Join(parts, ": ")
}

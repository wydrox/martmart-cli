package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/login"
	"github.com/wydrox/martmart-cli/internal/session"
)

const sessionStatusLoginWindow = 5 * time.Minute

var (
	sessionStatusCheckMu       sync.Mutex
	sessionStatusCheckedByProv = map[string]time.Time{}
)

// registerAccountSessionAuthTools registers all account, session, and auth MCP tools.
func registerAccountSessionAuthTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_profile",
		Description: "Fetch account profile (GET /users/{id}).",
	}, toolAccountProfile)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_addresses_list",
		Description: "List shipping addresses (GET /users/{id}/addresses/shipping-addresses).",
	}, toolAccountAddressesList)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_addresses_add",
		Description: "Add shipping address JSON (object or {shippingAddress:{...}}).",
	}, toolAccountAddressesAdd)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_addresses_update",
		Description: "Update shipping address by UUID (PUT). Body object or {shippingAddress:{...}}.",
	}, toolAccountAddressesUpdate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_addresses_delete",
		Description: "Delete shipping address by UUID.",
	}, toolAccountAddressesDelete)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_consents_update",
		Description: "Update account consents (PUT /users/{id}/consents).",
	}, toolAccountConsentsUpdate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_vouchers",
		Description: "Fetch account vouchers (GET /users/{id}/vouchers).",
	}, toolAccountVouchers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_payments",
		Description: "Fetch account payment methods (GET /users/{id}/payments).",
	}, toolAccountPayments)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_membership_cards",
		Description: "Fetch membership cards (GET /users/{id}/membership-cards).",
	}, toolAccountMembershipCards)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "account_membership_points",
		Description: "Fetch membership points history (GET /users/{id}/membership/points).",
	}, toolAccountMembershipPoints)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_status",
		Description: "Inspect saved session/auth status for one provider or all providers. Use this before deciding whether interactive login is needed.",
	}, toolSessionStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_login",
		Description: "Opens the provider page in the user's default browser app with temporary remote debugging, captures auth session data automatically, and saves the session. Requires a recent session_status check for the same provider so agents do not open login unnecessarily.",
	}, toolSessionLogin)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "providers_list",
		Description: "List supported providers and whether a saved authenticated session exists for each provider.",
	}, toolProvidersList)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_from_curl",
		Description: "Parse curl, ApplyFromCurl, Save (mirrors CLI session from-curl).",
	}, toolSessionFromCurl)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_refresh_token",
		Description: "POST /app/commerce/connect/token with refresh_token grant.",
	}, toolAuthRefreshToken)
}

// accountAddressesListIn is the input type for the account_addresses_list tool.
type accountAddressesListIn struct {
	Provider string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID   string `json:"user_id,omitempty" jsonschema:"optional; defaults to session user_id"`
}

// accountProfileIn is the input type for the account_profile tool.
type accountProfileIn struct {
	Provider string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID   string `json:"user_id,omitempty" jsonschema:"optional; defaults to session user_id"`
}

func toolAccountProfile(_ context.Context, _ *mcp.CallToolRequest, in accountProfileIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, uid, err := loadSessionAuth(in.Provider, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

func toolAccountAddressesList(_ context.Context, _ *mcp.CallToolRequest, in accountAddressesListIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, uid, err := loadSessionAuth(in.Provider, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// accountAddressesAddIn is the input type for the account_addresses_add tool.
type accountAddressesAddIn struct {
	Provider string         `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID   string         `json:"user_id,omitempty"`
	Payload  map[string]any `json:"payload"`
}

func toolAccountAddressesAdd(_ context.Context, _ *mcp.CallToolRequest, in accountAddressesAddIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if len(in.Payload) == 0 {
		return nil, mcpCPFriscoToolOut{}, errors.New("payload is required")
	}
	s, uid, err := loadSessionAuth(in.Provider, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	body := wrapShippingAddressPayload(in.Payload)
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses", uid)
	result, err := httpclient.RequestJSON(s, "POST", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// accountAddressesUpdateIn is the input type for the account_addresses_update tool.
type accountAddressesUpdateIn struct {
	Provider  string         `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID    string         `json:"user_id,omitempty"`
	AddressID string         `json:"address_id"`
	Payload   map[string]any `json:"payload"`
}

func toolAccountAddressesUpdate(_ context.Context, _ *mcp.CallToolRequest, in accountAddressesUpdateIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.AddressID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("address_id is required")
	}
	if len(in.Payload) == 0 {
		return nil, mcpCPFriscoToolOut{}, errors.New("payload is required")
	}
	s, uid, err := loadSessionAuth(in.Provider, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	body := wrapShippingAddressPayload(in.Payload)
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses/%s", uid, in.AddressID)
	result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// accountAddressesDeleteIn is the input type for the account_addresses_delete tool.
type accountAddressesDeleteIn struct {
	Provider  string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID    string `json:"user_id,omitempty"`
	AddressID string `json:"address_id"`
}

func toolAccountAddressesDelete(_ context.Context, _ *mcp.CallToolRequest, in accountAddressesDeleteIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.AddressID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("address_id is required")
	}
	s, uid, err := loadSessionAuth(in.Provider, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses/%s", uid, in.AddressID)
	result, err := httpclient.RequestJSON(s, "DELETE", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// wrapShippingAddressPayload ensures the address payload is wrapped under
// a "shippingAddress" key if it is not already.
func wrapShippingAddressPayload(data map[string]any) map[string]any {
	if _, has := data["shippingAddress"]; has {
		return data
	}
	return map[string]any{"shippingAddress": data}
}

// accountConsentsUpdateIn is the input type for the account_consents_update tool.
type accountConsentsUpdateIn struct {
	Provider string         `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID   string         `json:"user_id,omitempty"`
	Payload  map[string]any `json:"payload"`
}

func toolAccountConsentsUpdate(_ context.Context, _ *mcp.CallToolRequest, in accountConsentsUpdateIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if len(in.Payload) == 0 {
		return nil, mcpCPFriscoToolOut{}, errors.New("payload is required")
	}
	s, uid, err := loadSessionAuth(in.Provider, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	body, err := mcpASANormalizeConsentsPayload(s, uid, in.Payload)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/consents", uid)
	result, err := httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// mcpASANormalizeConsentsPayload accepts either raw Frisco payload shape
// (consentDecisions + consentChannel) or a simplified map (consents/email/phone/etc)
// and always converts to the Frisco API contract.
func mcpASANormalizeConsentsPayload(s *session.Session, userID string, payload map[string]any) (map[string]any, error) {
	channel, hasChannel, err := mcpASAConsentChannelFromPayload(payload)
	if err != nil {
		return nil, err
	}
	decisions, hasExplicitDecisions, err := mcpASAExtractExplicitConsentDecisions(payload)
	if err != nil {
		return nil, err
	}
	if hasExplicitDecisions {
		if !hasChannel {
			channel = 0
		}
		return map[string]any{
			"consentChannel":   channel,
			"consentDecisions": decisions,
		}, nil
	}

	requestedConsents := mcpASAExtractSimpleConsentMap(payload)
	if len(requestedConsents) == 0 {
		return nil, errors.New("payload must contain either consentDecisions or simplified consent booleans (email, phone, third_party, membership_rewards, meal_concierge)")
	}

	catalogByType, err := mcpASAFetchConsentCatalogByType(s, userID)
	if err != nil {
		return nil, err
	}
	decisions = make([]map[string]any, 0, len(requestedConsents))
	var unknown []string
	for k, v := range requestedConsents {
		consentType := mcpASANormalizeConsentType(k)
		consentID, ok := catalogByType[consentType]
		if !ok {
			unknown = append(unknown, k)
			continue
		}
		decisions = append(decisions, map[string]any{
			"consentId":   consentID,
			"isConsented": v,
		})
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		available := make([]string, 0, len(catalogByType))
		for k := range catalogByType {
			available = append(available, k)
		}
		sort.Strings(available)
		return nil, fmt.Errorf("unknown consent keys: %s; available consent types: %s", strings.Join(unknown, ", "), strings.Join(available, ", "))
	}
	if len(decisions) == 0 {
		return nil, errors.New("no consent decisions produced from payload")
	}
	if !hasChannel {
		channel = 0
	}
	return map[string]any{
		"consentChannel":   channel,
		"consentDecisions": decisions,
	}, nil
}

// mcpASAConsentChannelFromPayload returns (channel, hasChannel, err).
func mcpASAConsentChannelFromPayload(payload map[string]any) (int, bool, error) {
	for _, key := range []string{"consentChannel", "ConsentChannel", "consent_channel"} {
		v, ok := payload[key]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case int:
			return t, true, nil
		case int64:
			return int(t), true, nil
		case float64:
			return int(t), true, nil
		case string:
			n, err := strconv.Atoi(strings.TrimSpace(t))
			if err != nil {
				return 0, false, fmt.Errorf("consent_channel must be numeric, got %q", t)
			}
			return n, true, nil
		default:
			return 0, false, fmt.Errorf("consent_channel has unsupported type %T", v)
		}
	}
	return 0, false, nil
}

// mcpASAExtractExplicitConsentDecisions normalizes an explicit consentDecisions array.
func mcpASAExtractExplicitConsentDecisions(payload map[string]any) ([]map[string]any, bool, error) {
	var raw any
	var ok bool
	for _, key := range []string{"consentDecisions", "ConsentDecisions", "consent_decisions"} {
		raw, ok = payload[key]
		if ok {
			break
		}
	}
	if !ok {
		return nil, false, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, true, errors.New("consentDecisions must be an array")
	}
	out := make([]map[string]any, 0, len(arr))
	for i, item := range arr {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, true, fmt.Errorf("consentDecisions[%d] must be an object", i)
		}
		consentID := mcpASAGetStringAny(row, "consentId", "ConsentId", "consent_id")
		if strings.TrimSpace(consentID) == "" {
			return nil, true, fmt.Errorf("consentDecisions[%d].consentId is required", i)
		}
		val, ok := mcpASAGetBoolAny(row, "isConsented", "IsConsented", "is_consented", "isAccepted", "IsAccepted", "is_accepted")
		if !ok {
			return nil, true, fmt.Errorf("consentDecisions[%d].isConsented (or isAccepted) is required", i)
		}
		out = append(out, map[string]any{
			"consentId":   consentID,
			"isConsented": val,
		})
	}
	return out, true, nil
}

// mcpASAExtractSimpleConsentMap extracts consent booleans from payload.
func mcpASAExtractSimpleConsentMap(payload map[string]any) map[string]bool {
	out := map[string]bool{}
	if v, ok := payload["consents"]; ok {
		if m, ok := v.(map[string]any); ok {
			for k, raw := range m {
				if b, ok := raw.(bool); ok {
					out[k] = b
				}
			}
		}
	}
	for k, raw := range payload {
		switch k {
		case "consentChannel", "ConsentChannel", "consent_channel", "consentDecisions", "ConsentDecisions", "consent_decisions", "consents":
			continue
		}
		if b, ok := raw.(bool); ok {
			out[k] = b
		}
	}
	return out
}

// mcpASAFetchConsentCatalogByType maps consent type -> consent id.
func mcpASAFetchConsentCatalogByType(s *session.Session, userID string) (map[string]string, error) {
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/consents", userID)
	data, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, err
	}
	root, ok := data.(map[string]any)
	if !ok {
		return nil, errors.New("unexpected /consents response shape")
	}
	rawList, ok := root["consents"].([]any)
	if !ok {
		return nil, errors.New("unexpected /consents response: missing consents list")
	}
	out := map[string]string{}
	for _, item := range rawList {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		consentID := mcpASAGetStringAny(row, "consentId", "ConsentId", "consent_id")
		consentObj, _ := row["consent"].(map[string]any)
		consentType := mcpASAGetStringAny(consentObj, "consentType", "ConsentType", "consent_type")
		if strings.TrimSpace(consentID) == "" || strings.TrimSpace(consentType) == "" {
			continue
		}
		out[mcpASANormalizeConsentType(consentType)] = consentID
	}
	if len(out) == 0 {
		return nil, errors.New("no consent definitions available for this user")
	}
	return out, nil
}

// mcpASANormalizeConsentType normalizes consent keys for matching.
func mcpASANormalizeConsentType(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	switch s {
	case "sms", "text", "telephone":
		return "phone"
	case "thirdpartyconsent", "partners":
		return "thirdparty"
	case "membership", "membershipreward":
		return "membershiprewards"
	case "mealconciergeconsent":
		return "mealconcierge"
	default:
		return s
	}
}

// mcpASAGetStringAny returns the first non-empty string field by keys.
func mcpASAGetStringAny(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := mcpASAStringField(v); ok {
				return s
			}
		}
	}
	return ""
}

// mcpASAGetBoolAny returns first bool-like field by keys.
func mcpASAGetBoolAny(m map[string]any, keys ...string) (bool, bool) {
	if m == nil {
		return false, false
	}
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case bool:
			return t, true
		case string:
			switch strings.ToLower(strings.TrimSpace(t)) {
			case "true", "1", "yes":
				return true, true
			case "false", "0", "no":
				return false, true
			}
		}
	}
	return false, false
}

// accountVouchersIn is the input type for the account_vouchers tool.
type accountVouchersIn struct {
	Provider string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID   string `json:"user_id,omitempty"`
}

func toolAccountVouchers(_ context.Context, _ *mcp.CallToolRequest, in accountVouchersIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, uid, err := loadSessionAuth(in.Provider, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/vouchers", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// accountPaymentsIn is the input type for the account_payments tool.
type accountPaymentsIn struct {
	Provider string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID   string `json:"user_id,omitempty"`
}

func toolAccountPayments(_ context.Context, _ *mcp.CallToolRequest, in accountPaymentsIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, uid, err := loadSessionAuth(in.Provider, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/payments", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// accountMembershipCardsIn is the input type for the account_membership_cards tool.
type accountMembershipCardsIn struct {
	Provider string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID   string `json:"user_id,omitempty"`
}

func toolAccountMembershipCards(_ context.Context, _ *mcp.CallToolRequest, in accountMembershipCardsIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, uid, err := loadSessionAuth(in.Provider, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/membership-cards", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

// accountMembershipPointsIn is the input type for the account_membership_points tool.
type accountMembershipPointsIn struct {
	Provider  string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco"`
	UserID    string `json:"user_id,omitempty"`
	PageIndex int    `json:"page_index,omitempty"`
	PageSize  int    `json:"page_size,omitempty"`
}

func toolAccountMembershipPoints(_ context.Context, _ *mcp.CallToolRequest, in accountMembershipPointsIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, uid, err := loadSessionAuth(in.Provider, in.UserID)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	pageIndex := in.PageIndex
	if pageIndex <= 0 {
		pageIndex = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 25
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/membership/points", uid)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{
		Query: []string{
			fmt.Sprintf("pageIndex=%d", pageIndex),
			fmt.Sprintf("pageSize=%d", pageSize),
		},
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(result)
}

type sessionStatusIn struct {
	Provider string `json:"provider,omitempty" jsonschema:"optional provider id; when omitted returns all providers; one of delio, frisco, upmenu"`
}

func toolSessionStatus(_ context.Context, _ *mcp.CallToolRequest, in sessionStatusIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	targetProviders, err := sessionStatusProviders(strings.TrimSpace(in.Provider))
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	payload, err := sessionStatusPayload(targetProviders)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	markSessionStatusChecked(targetProviders...)
	return mcpCPWrapFriscoResult(payload)
}

func toolProvidersList(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	targetProviders, err := sessionStatusProviders("")
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	payload, err := sessionStatusPayload(targetProviders)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(payload)
}

func sessionStatusProviders(requestedProvider string) ([]string, error) {
	if strings.TrimSpace(requestedProvider) == "" {
		return mcpAvailableProviders(), nil
	}
	provider, err := mcpResolveSessionProvider(requestedProvider)
	if err != nil {
		return nil, err
	}
	return []string{provider}, nil
}

func mcpResolveSessionProvider(provider string) (string, error) {
	provider = session.NormalizeProvider(provider)
	if provider == "" {
		return "", errors.New("provider is required; ask the user whether to use frisco, delio, or upmenu")
	}
	if err := session.ValidateProvider(provider); err != nil {
		return "", err
	}
	return provider, nil
}

func sessionStatusPayload(targetProviders []string) (map[string]any, error) {
	providers := mcpAvailableProviders()
	items := make([]map[string]any, 0, len(targetProviders))
	savedProviders := make([]string, 0, len(targetProviders))
	authenticatedProviders := make([]string, 0, len(targetProviders))
	for _, provider := range targetProviders {
		s, sourcePath, err := session.LoadProviderWithPath(provider)
		if err != nil {
			return nil, err
		}
		item := sessionStatusEntry(provider, s, sourcePath)
		items = append(items, item)
		if item["session_saved"] == true {
			savedProviders = append(savedProviders, provider)
		}
		if item["authenticated"] == true {
			authenticatedProviders = append(authenticatedProviders, provider)
		}
	}

	payload := map[string]any{
		"available_providers":     providers,
		"providers":               items,
		"saved_providers":         savedProviders,
		"authenticated_providers": authenticatedProviders,
	}
	if len(targetProviders) == 1 {
		payload["requested_provider"] = targetProviders[0]
	}
	return payload, nil
}

func markSessionStatusChecked(providers ...string) {
	now := time.Now()
	sessionStatusCheckMu.Lock()
	defer sessionStatusCheckMu.Unlock()
	for _, provider := range providers {
		provider = session.NormalizeProvider(provider)
		if provider == "" {
			continue
		}
		sessionStatusCheckedByProv[provider] = now
	}
}

func resetSessionStatusChecks() {
	sessionStatusCheckMu.Lock()
	defer sessionStatusCheckMu.Unlock()
	sessionStatusCheckedByProv = map[string]time.Time{}
}

func recentSessionStatusCheck(provider string, now time.Time) bool {
	provider = session.NormalizeProvider(provider)
	sessionStatusCheckMu.Lock()
	defer sessionStatusCheckMu.Unlock()
	checkedAt, ok := sessionStatusCheckedByProv[provider]
	if !ok {
		return false
	}
	return now.Sub(checkedAt) <= sessionStatusLoginWindow
}

func requireRecentSessionStatus(provider string, now time.Time) error {
	if recentSessionStatusCheck(provider, now) {
		return nil
	}
	return fmt.Errorf("call session_status for provider %q immediately before session_login", session.NormalizeProvider(provider))
}

func sessionStatusEntry(provider string, s *session.Session, sourcePath string) map[string]any {
	authorizationSaved := session.HeaderValue(s, "Authorization") != ""
	cookieSaved := session.HeaderValue(s, "Cookie") != ""
	tokenSaved := mcpASATokenSaved(s)
	refreshTokenSaved := session.RefreshTokenString(s) != ""
	authMechanisms := make([]string, 0, 3)
	if tokenSaved {
		authMechanisms = append(authMechanisms, "token")
	}
	if authorizationSaved {
		authMechanisms = append(authMechanisms, "authorization_header")
	}
	if cookieSaved {
		authMechanisms = append(authMechanisms, "cookie")
	}
	return map[string]any{
		"provider":               provider,
		"base_url":               s.BaseURL,
		"default_base_url":       session.DefaultBaseURLForProvider(provider),
		"session_file":           sourcePath,
		"session_saved":          sourcePath != "",
		"authenticated":          session.IsAuthenticated(s),
		"user_id":                session.UserIDString(s),
		"token_saved":            tokenSaved,
		"authorization_saved":    authorizationSaved,
		"refresh_token_saved":    refreshTokenSaved,
		"cookie_saved":           cookieSaved,
		"header_keys":            mcpASAHeaderKeysSorted(s.Headers),
		"auth_mechanisms":        authMechanisms,
		"interactive_login_hint": provider != session.ProviderUpMenu && !session.IsAuthenticated(s),
	}
}

// sessionFromCurlIn is the input type for the session_from_curl tool.
type sessionFromCurlIn struct {
	Provider string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco, upmenu"`
	Curl     string `json:"curl"`
}

func toolSessionFromCurl(_ context.Context, _ *mcp.CallToolRequest, in sessionFromCurlIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.Curl) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("curl is required")
	}
	provider, err := mcpResolveSessionProvider(in.Provider)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	cd, err := session.ParseCurl(in.Curl)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	s, err := session.LoadProvider(provider)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	session.ApplyFromCurlForProvider(s, cd, provider)
	if err := session.SaveProvider(provider, s); err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"provider":      provider,
		"saved":         true,
		"base_url":      s.BaseURL,
		"user_id":       s.UserID,
		"token_saved":   mcpASATokenSaved(s),
		"headers_saved": mcpASAHeaderKeysSorted(s.Headers),
	})
}

// authRefreshTokenIn is the input type for the session_refresh_token tool.
type authRefreshTokenIn struct {
	Provider     string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco, upmenu"`
	RefreshToken string `json:"refresh_token,omitempty" jsonschema:"optional; else session refresh_token"`
}

func toolAuthRefreshToken(_ context.Context, _ *mcp.CallToolRequest, in authRefreshTokenIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	provider, err := mcpResolveSessionProvider(in.Provider)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	if provider == session.ProviderUpMenu {
		return nil, mcpCPFriscoToolOut{}, errors.New("session_refresh_token is not supported for provider \"upmenu\"; import a fresh request with session_from_curl or use the public upmenu_* tools")
	}
	s, err := session.LoadProvider(provider)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	rt := strings.TrimSpace(in.RefreshToken)
	if rt == "" {
		rt = session.RefreshTokenString(s)
	}
	if rt == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("missing refresh_token (argument or session)")
	}
	payload := map[string]any{
		"grant_type":    "refresh_token",
		"refresh_token": rt,
	}
	result, err := httpclient.RequestJSON(s, "POST", "/app/commerce/connect/token", httpclient.RequestOpts{
		Data:       payload,
		DataFormat: httpclient.FormatForm,
	})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	if m, ok := result.(map[string]any); ok {
		expiresIn := m["expires_in"]
		if at, ok := mcpASAStringField(m["access_token"]); ok && at != "" {
			s.Token = at
			if s.Headers == nil {
				s.Headers = map[string]string{}
			}
			s.Headers["Authorization"] = "Bearer " + at
		}
		if nr, ok := mcpASAStringField(m["refresh_token"]); ok && nr != "" {
			s.RefreshToken = nr
		}
		if err := session.SaveProvider(provider, s); err != nil {
			return nil, mcpCPFriscoToolOut{}, err
		}
		return mcpCPWrapFriscoResult(map[string]any{
			"provider":            provider,
			"saved":               true,
			"token_saved":         mcpASATokenSaved(s),
			"refresh_token_saved": session.RefreshTokenString(s) != "",
			"expires_in":          expiresIn,
		})
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"provider":            provider,
		"saved":               false,
		"token_saved":         mcpASATokenSaved(s),
		"refresh_token_saved": session.RefreshTokenString(s) != "",
		"message":             "Unexpected token endpoint payload shape; session not updated.",
	})
}

const defaultSessionLoginTimeoutSec = 180

// sessionLoginIn is the input type for the session_login tool.
type sessionLoginIn struct {
	Provider   string `json:"provider,omitempty" jsonschema:"provider id; required; one of delio, frisco, upmenu"`
	TimeoutSec *int   `json:"timeout_sec,omitempty" jsonschema:"Login timeout in seconds; default 180"`
}

func sessionLoginTimeoutSec(in sessionLoginIn) int {
	timeout := defaultSessionLoginTimeoutSec
	if in.TimeoutSec != nil && *in.TimeoutSec > 0 {
		timeout = *in.TimeoutSec
	}
	return timeout
}

func toolSessionLogin(ctx context.Context, _ *mcp.CallToolRequest, in sessionLoginIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	provider, err := mcpResolveSessionProvider(in.Provider)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	if provider == session.ProviderUpMenu {
		return nil, mcpCPFriscoToolOut{}, errors.New("session_login is not supported for provider \"upmenu\"; use session_status or the public upmenu_* tools instead")
	}
	if err := requireRecentSessionStatus(provider, time.Now()); err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	timeout := sessionLoginTimeoutSec(in)
	result, err := login.Run(ctx, login.Options{Provider: provider, TimeoutSec: timeout})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"provider":              provider,
		"saved":                 result.Saved,
		"browser_app":           result.BrowserApp,
		"browser_user_data_dir": result.BrowserUserDataDir,
		"profile_directory":     result.ProfileDirectory,
		"base_url":              result.BaseURL,
		"user_id":               result.UserID,
		"token_saved":           result.TokenSaved,
		"refresh_token_saved":   result.RefreshTokenSaved,
		"cookie_saved":          result.CookieSaved,
	})
}

// mcpASAStringField converts v to a trimmed string and reports whether it is non-empty.
func mcpASAStringField(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t), strings.TrimSpace(t) != ""
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		return s, s != ""
	}
}

// mcpASATokenSaved reports whether the session contains a non-empty access token.
func mcpASATokenSaved(s *session.Session) bool {
	if s == nil || s.Token == nil {
		return false
	}
	if str, ok := s.Token.(string); ok {
		return str != ""
	}
	return true
}

// mcpASAHeaderKeysSorted returns the header map keys in sorted order.
func mcpASAHeaderKeysSorted(h map[string]string) []string {
	if len(h) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/login"
	"github.com/wydrox/martmart-cli/internal/session"
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
		Name:        "session_login",
		Description: "Opens the provider page in the user's default Chromium-based browser app with temporary remote debugging, captures auth session data automatically, and saves the session.",
	}, toolSessionLogin)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_show",
		Description: "Current session with secrets redacted (same as CLI session show).",
	}, toolSessionShow)

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
	UserID string `json:"user_id,omitempty" jsonschema:"optional; defaults to session user_id"`
}

// accountProfileIn is the input type for the account_profile tool.
type accountProfileIn struct {
	UserID string `json:"user_id,omitempty" jsonschema:"optional; defaults to session user_id"`
}

func toolAccountProfile(_ context.Context, _ *mcp.CallToolRequest, in accountProfileIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, uid, err := loadSessionAuth(in.UserID)
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
	s, uid, err := loadSessionAuth(in.UserID)
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
	UserID  string         `json:"user_id,omitempty"`
	Payload map[string]any `json:"payload"`
}

func toolAccountAddressesAdd(_ context.Context, _ *mcp.CallToolRequest, in accountAddressesAddIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if len(in.Payload) == 0 {
		return nil, mcpCPFriscoToolOut{}, errors.New("payload is required")
	}
	s, uid, err := loadSessionAuth(in.UserID)
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
	s, uid, err := loadSessionAuth(in.UserID)
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
	UserID    string `json:"user_id,omitempty"`
	AddressID string `json:"address_id"`
}

func toolAccountAddressesDelete(_ context.Context, _ *mcp.CallToolRequest, in accountAddressesDeleteIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.AddressID) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("address_id is required")
	}
	s, uid, err := loadSessionAuth(in.UserID)
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
	UserID  string         `json:"user_id,omitempty"`
	Payload map[string]any `json:"payload"`
}

func toolAccountConsentsUpdate(_ context.Context, _ *mcp.CallToolRequest, in accountConsentsUpdateIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if len(in.Payload) == 0 {
		return nil, mcpCPFriscoToolOut{}, errors.New("payload is required")
	}
	s, uid, err := loadSessionAuth(in.UserID)
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
	UserID string `json:"user_id,omitempty"`
}

func toolAccountVouchers(_ context.Context, _ *mcp.CallToolRequest, in accountVouchersIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, uid, err := loadSessionAuth(in.UserID)
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
	UserID string `json:"user_id,omitempty"`
}

func toolAccountPayments(_ context.Context, _ *mcp.CallToolRequest, in accountPaymentsIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, uid, err := loadSessionAuth(in.UserID)
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
	UserID string `json:"user_id,omitempty"`
}

func toolAccountMembershipCards(_ context.Context, _ *mcp.CallToolRequest, in accountMembershipCardsIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, uid, err := loadSessionAuth(in.UserID)
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
	UserID    string `json:"user_id,omitempty"`
	PageIndex int    `json:"page_index,omitempty"`
	PageSize  int    `json:"page_size,omitempty"`
}

func toolAccountMembershipPoints(_ context.Context, _ *mcp.CallToolRequest, in accountMembershipPointsIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, uid, err := loadSessionAuth(in.UserID)
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

// sessionShowIn is the (empty) input type for the session_show tool.
type sessionShowIn struct{}

func toolSessionShow(_ context.Context, _ *mcp.CallToolRequest, _ sessionShowIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(session.RedactedCopy(s))
}

// sessionFromCurlIn is the input type for the session_from_curl tool.
type sessionFromCurlIn struct {
	Curl string `json:"curl"`
}

func toolSessionFromCurl(_ context.Context, _ *mcp.CallToolRequest, in sessionFromCurlIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	if strings.TrimSpace(in.Curl) == "" {
		return nil, mcpCPFriscoToolOut{}, errors.New("curl is required")
	}
	cd, err := session.ParseCurl(in.Curl)
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	s, err := session.Load()
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	session.ApplyFromCurl(s, cd)
	if err := session.Save(s); err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"saved":         true,
		"base_url":      s.BaseURL,
		"user_id":       s.UserID,
		"token_saved":   mcpASATokenSaved(s),
		"headers_saved": mcpASAHeaderKeysSorted(s.Headers),
	})
}

// authRefreshTokenIn is the input type for the session_refresh_token tool.
type authRefreshTokenIn struct {
	RefreshToken string `json:"refresh_token,omitempty" jsonschema:"optional; else session refresh_token"`
}

func toolAuthRefreshToken(_ context.Context, _ *mcp.CallToolRequest, in authRefreshTokenIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	s, err := session.Load()
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
		if err := session.Save(s); err != nil {
			return nil, mcpCPFriscoToolOut{}, err
		}
		return mcpCPWrapFriscoResult(map[string]any{
			"saved":               true,
			"token_saved":         mcpASATokenSaved(s),
			"refresh_token_saved": session.RefreshTokenString(s) != "",
			"expires_in":          expiresIn,
		})
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"saved":               false,
		"token_saved":         mcpASATokenSaved(s),
		"refresh_token_saved": session.RefreshTokenString(s) != "",
		"message":             "Unexpected token endpoint payload shape; session not updated.",
	})
}

// sessionLoginIn is the input type for the session_login tool.
type sessionLoginIn struct {
	TimeoutSec *int `json:"timeout_sec,omitempty" jsonschema:"Login timeout in seconds; default 180"`
}

func toolSessionLogin(ctx context.Context, _ *mcp.CallToolRequest, in sessionLoginIn) (*mcp.CallToolResult, mcpCPFriscoToolOut, error) {
	timeout := 10
	if in.TimeoutSec != nil && *in.TimeoutSec > 0 {
		timeout = *in.TimeoutSec
	}
	result, err := login.Run(ctx, login.Options{Provider: session.CurrentProvider(), TimeoutSec: timeout})
	if err != nil {
		return nil, mcpCPFriscoToolOut{}, err
	}
	return mcpCPWrapFriscoResult(map[string]any{
		"saved":               result.Saved,
		"base_url":            result.BaseURL,
		"user_id":             result.UserID,
		"token_saved":         result.TokenSaved,
		"refresh_token_saved": result.RefreshTokenSaved,
		"cookie_saved":        result.CookieSaved,
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

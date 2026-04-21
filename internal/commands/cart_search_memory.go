package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wydrox/martmart-cli/internal/catalog"
	"github.com/wydrox/martmart-cli/internal/delio"
	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
)

var (
	cartSearchNow = func() time.Time { return time.Now().UTC() }

	catalogGetQueryBestEffort = func(provider, queryNorm string) (*catalog.QueryRecord, error) {
		db, err := catalog.Open()
		if err != nil {
			return nil, err
		}
		defer db.Close()
		return db.GetQuery(context.Background(), provider, queryNorm)
	}
	catalogUpsertQuerySuccessBestEffort = func(input catalog.QuerySuccessInput) error {
		db, err := catalog.Open()
		if err != nil {
			return err
		}
		defer db.Close()
		return db.UpsertQuerySuccess(context.Background(), input)
	}
	catalogUpsertQueryErrorBestEffort = func(provider, queryText, queryNorm, errorCode string, fallbackUsed bool, now time.Time) error {
		db, err := catalog.Open()
		if err != nil {
			return err
		}
		defer db.Close()
		return db.UpsertQueryError(context.Background(), provider, queryText, queryNorm, errorCode, now, fallbackUsed)
	}

	cartSearchIngest = ingestSearchBestEffort

	friscoSearchAndPick = resolveProductBySearchPayload
	friscoCartAdd       = addFriscoProductToCart

	delioResolveCoords      = delio.ResolveCoordinates
	delioSearchAndPick      = resolveDelioProductBySearchPayload
	delioCurrentCart        = delio.CurrentCart
	delioExtractCurrentCart = delio.ExtractCurrentCart
	delioUpdateCurrentCart  = delio.UpdateCurrentCart
	delioValidateCartUpdate = delio.ExtractUpdatedCart
)

func cartAddSearchWithMemoryFrisco(s *session.Session, uid, phrase, categoryID string, quantity int) (any, error) {
	now := cartSearchNow()
	queryNorm := catalog.BuildFriscoQueryNorm(phrase, categoryID)
	rec, err := catalogGetQueryBestEffort(session.ProviderFrisco, queryNorm)
	if err != nil {
		rec = nil
	}
	if rec == nil || catalog.IsStale(rec, now) || strings.TrimSpace(rec.LastSelectedProductID) == "" {
		return friscoLiveSearchAdd(s, uid, phrase, categoryID, quantity, queryNorm, false, now)
	}

	result, err := friscoCartAdd(s, uid, rec.LastSelectedProductID, quantity)
	if err == nil {
		_ = catalogUpsertQuerySuccessBestEffort(catalog.QuerySuccessInput{
			Provider:          session.ProviderFrisco,
			QueryText:         phrase,
			QueryNorm:         queryNorm,
			TTLDays:           rec.TTLDays,
			Now:               now,
			PreserveSelection: true,
		})
		return result, nil
	}
	if isFriscoFallbackEligible(err) {
		return friscoLiveSearchAdd(s, uid, phrase, categoryID, quantity, queryNorm, true, now)
	}
	_ = catalogUpsertQueryErrorBestEffort(session.ProviderFrisco, phrase, queryNorm, classifyQueryErrorCode(err), false, now)
	return nil, err
}

func friscoLiveSearchAdd(s *session.Session, uid, phrase, categoryID string, quantity int, queryNorm string, fallbackUsed bool, now time.Time) (any, error) {
	payload, productID, err := friscoSearchAndPick(s, uid, phrase, categoryID)
	if err != nil {
		_ = catalogUpsertQueryErrorBestEffort(session.ProviderFrisco, phrase, queryNorm, classifyQueryErrorCode(err), fallbackUsed, now)
		return nil, err
	}
	cartSearchIngest(session.ProviderFrisco, phrase, payload)
	result, err := friscoCartAdd(s, uid, productID, quantity)
	if err != nil {
		_ = catalogUpsertQueryErrorBestEffort(session.ProviderFrisco, phrase, queryNorm, classifyQueryErrorCode(err), fallbackUsed, now)
		return nil, err
	}
	_ = catalogUpsertQuerySuccessBestEffort(catalog.QuerySuccessInput{
		Provider:              session.ProviderFrisco,
		QueryText:             phrase,
		QueryNorm:             queryNorm,
		LastSelectedProductID: productID,
		Now:                   now,
		LiveSearchUsed:        true,
		FallbackUsed:          fallbackUsed,
	})
	return result, nil
}

func cartAddSearchWithMemoryDelio(s *session.Session, phrase string, coords *delio.Coordinates, quantity int) (any, error) {
	now := cartSearchNow()
	resolved, err := delioResolveCoords(s, coords)
	if err != nil {
		return nil, err
	}
	queryNorm := catalog.BuildDelioQueryNorm(phrase, resolved)
	rec, getErr := catalogGetQueryBestEffort(session.ProviderDelio, queryNorm)
	if getErr != nil {
		rec = nil
	}
	if rec == nil || catalog.IsStale(rec, now) || strings.TrimSpace(rec.LastSelectedProductID) == "" {
		return delioLiveSearchAdd(s, phrase, resolved, quantity, queryNorm, false, now)
	}

	result, err := attemptDelioCartAdd(s, rec.LastSelectedProductID, quantity)
	if err == nil {
		_ = catalogUpsertQuerySuccessBestEffort(catalog.QuerySuccessInput{
			Provider:          session.ProviderDelio,
			QueryText:         phrase,
			QueryNorm:         queryNorm,
			TTLDays:           rec.TTLDays,
			Now:               now,
			PreserveSelection: true,
		})
		return result, nil
	}
	if delio.IsUpdateCurrentCartBusinessError(err) {
		return delioLiveSearchAdd(s, phrase, resolved, quantity, queryNorm, true, now)
	}
	_ = catalogUpsertQueryErrorBestEffort(session.ProviderDelio, phrase, queryNorm, classifyQueryErrorCode(err), false, now)
	return nil, err
}

func delioLiveSearchAdd(s *session.Session, phrase string, coords *delio.Coordinates, quantity int, queryNorm string, fallbackUsed bool, now time.Time) (any, error) {
	payload, sku, err := delioSearchAndPick(s, phrase, coords)
	if err != nil {
		_ = catalogUpsertQueryErrorBestEffort(session.ProviderDelio, phrase, queryNorm, classifyQueryErrorCode(err), fallbackUsed, now)
		return nil, err
	}
	cartSearchIngest(session.ProviderDelio, phrase, payload)
	result, err := attemptDelioCartAdd(s, sku, quantity)
	if err != nil {
		_ = catalogUpsertQueryErrorBestEffort(session.ProviderDelio, phrase, queryNorm, classifyQueryErrorCode(err), fallbackUsed, now)
		return nil, err
	}
	_ = catalogUpsertQuerySuccessBestEffort(catalog.QuerySuccessInput{
		Provider:              session.ProviderDelio,
		QueryText:             phrase,
		QueryNorm:             queryNorm,
		LastSelectedProductID: sku,
		Now:                   now,
		LiveSearchUsed:        true,
		FallbackUsed:          fallbackUsed,
	})
	return result, nil
}

func attemptDelioCartAdd(s *session.Session, sku string, quantity int) (any, error) {
	current, err := delioCurrentCart(s)
	if err != nil {
		return nil, err
	}
	cart, err := delioExtractCurrentCart(current)
	if err != nil {
		return nil, err
	}
	result, err := delioUpdateCurrentCart(s, asString(cart["id"]), []map[string]any{{
		"AddLineItem": map[string]any{"quantity": quantity, "sku": sku},
	}})
	if err != nil {
		return nil, err
	}
	if _, err := delioValidateCartUpdate(result); err != nil {
		return nil, err
	}
	return result, nil
}

func addFriscoProductToCart(s *session.Session, uid, productID string, quantity int) (any, error) {
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", uid)
	body := map[string]any{
		"products": []any{
			map[string]any{"productId": productID, "quantity": quantity},
		},
	}
	return httpclient.RequestJSON(s, "PUT", path, httpclient.RequestOpts{Data: body, DataFormat: httpclient.FormatJSON})
}

func isFriscoFallbackEligible(err error) bool {
	details, ok := httpclient.ParseError(err)
	if !ok {
		return false
	}
	switch details.Status {
	case 400, 404, 409, 410, 422:
		return true
	default:
		return false
	}
}

func classifyQueryErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if details, ok := httpclient.ParseError(err); ok {
		return fmt.Sprintf("http_%d", details.Status)
	}
	if delio.IsUpdateCurrentCartBusinessError(err) {
		return "graphql_business_error"
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "no products found") || strings.Contains(msg, "no delio products found"):
		return "search_no_results"
	case strings.Contains(msg, "no strong match") || strings.Contains(msg, "use --product-id") || strings.Contains(msg, "no usable delio result"):
		return "search_no_match"
	case strings.Contains(msg, "connection error"):
		return "network_error"
	default:
		return "operation_error"
	}
}

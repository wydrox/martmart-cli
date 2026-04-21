package commands

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
)

const (
	checkoutPreviewPathPattern  = "/app/commerce/api/v2/users/%s/cart/checkout/preview"
	checkoutFinalizePathPattern = "/app/commerce/api/v2/users/%s/cart/checkout/finalize"
)

var (
	checkoutLoadSession     = loadSessionForSupportedProviders
	checkoutRequestJSON     = httpclient.RequestJSON
	checkoutGetShippingAddr = getShippingAddressFromAccount
)

func newCheckoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkout",
		Short: "Checkout preview and finalization.",
	}
	cmd.AddCommand(
		newCheckoutPreviewCmd(),
		newCheckoutFinalizeCmd(),
	)
	return cmd
}

func newCheckoutPreviewCmd() *cobra.Command {
	var userID, payloadFile string
	c := &cobra.Command{
		Use:   "preview",
		Short: "Preview Frisco checkout state and provisional checkout response.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := checkoutPayloadFromFile(payloadFile)
			if err != nil {
				return err
			}
			_, s, err := checkoutLoadSession(cmd, session.ProviderFrisco)
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			result, err := buildCheckoutPreview(s, uid, payload)
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&payloadFile, "payload-file", "", "Optional checkout payload JSON for provisional preview/finalize contract.")
	c.Flags().StringVar(&userID, "user-id", "", "")
	return c
}

func newCheckoutFinalizeCmd() *cobra.Command {
	var userID, payloadFile string
	var confirm bool
	c := &cobra.Command{
		Use:   "finalize",
		Short: "Finalize Frisco checkout. Requires explicit --confirm.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := checkoutPayloadFromFile(payloadFile)
			if err != nil {
				return err
			}
			_, s, err := checkoutLoadSession(cmd, session.ProviderFrisco)
			if err != nil {
				return err
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			if !confirm {
				preview, previewErr := buildCheckoutPreview(s, uid, payload)
				if previewErr != nil {
					preview = map[string]any{
						"mode":       "finalize",
						"userId":     uid,
						"payload":    payload,
						"previewErr": previewErr.Error(),
					}
				}
				guard := map[string]any{
					"aborted": true,
					"dryRun":  true,
					"guard": map[string]any{
						"requiresConfirm": true,
						"message":         "checkout finalize requires explicit --confirm; preview shown and no finalization request was sent",
					},
					"preview": preview,
				}
				if err := printJSON(guard); err != nil {
					return err
				}
				return errors.New("checkout finalize aborted: rerun with --confirm to submit the finalization request")
			}

			result, err := finalizeCheckout(s, uid, payload)
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&payloadFile, "payload-file", "", "Optional checkout payload JSON for provisional finalize request.")
	c.Flags().StringVar(&userID, "user-id", "", "")
	c.Flags().BoolVar(&confirm, "confirm", false, "Actually send the finalization request. Without this flag the command prints a dry-run preview and aborts.")
	return c
}

func checkoutPayloadFromFile(path string) (map[string]any, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	raw, err := loadJSONFile(path)
	if err != nil {
		return nil, err
	}
	payload, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("payload file must contain a JSON object")
	}
	return payload, nil
}

func buildCheckoutPreview(s *session.Session, userID string, payload map[string]any) (map[string]any, error) {
	cart, err := checkoutRequestJSON(s, "GET", fmt.Sprintf("/app/commerce/api/v1/users/%s/cart", userID), httpclient.RequestOpts{})
	if err != nil {
		return nil, err
	}
	reservation, _ := checkoutOptionalRequest(s, "GET", fmt.Sprintf("/app/commerce/api/v1/users/%s/cart/reservation", userID), httpclient.RequestOpts{})
	payments, _ := checkoutOptionalRequest(s, "GET", fmt.Sprintf("/app/commerce/api/v1/users/%s/payments", userID), httpclient.RequestOpts{})
	shippingAddress, _ := checkoutGetShippingAddr(s, userID)

	preview := map[string]any{
		"mode":        "preview",
		"provider":    session.ProviderFrisco,
		"userId":      userID,
		"payload":     payload,
		"cart":        cart,
		"cartSummary": summarizeCheckoutCart(cart),
	}
	if reservation != nil {
		preview["reservation"] = reservation
	}
	if shippingAddress != nil {
		preview["shippingAddress"] = shippingAddress
	}
	if payments != nil {
		preview["accountPayments"] = payments
	}
	if payload != nil {
		apiResponse, err := checkoutRequestJSON(s, "POST", fmt.Sprintf(checkoutPreviewPathPattern, userID), httpclient.RequestOpts{
			Data:       payload,
			DataFormat: httpclient.FormatJSON,
		})
		if err != nil {
			return nil, err
		}
		preview["checkoutPreview"] = apiResponse
		preview["checkoutPreviewEndpoint"] = fmt.Sprintf(checkoutPreviewPathPattern, userID)
	} else {
		preview["note"] = "no --payload-file supplied; showing live checkout context only"
	}
	return preview, nil
}

func finalizeCheckout(s *session.Session, userID string, payload map[string]any) (map[string]any, error) {
	ordersBeforeAny, _ := checkoutOptionalRequest(s, "GET", fmt.Sprintf("/app/commerce/api/v1/users/%s/orders", userID), httpclient.RequestOpts{
		Query: []string{"pageIndex=0", "pageSize=20"},
	})
	ordersBefore := extractOrdersList(ordersBeforeAny)

	result, err := checkoutRequestJSON(s, "POST", fmt.Sprintf(checkoutFinalizePathPattern, userID), httpclient.RequestOpts{
		Data:       payload,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, err
	}

	response := map[string]any{
		"provider":                 session.ProviderFrisco,
		"userId":                   userID,
		"confirm":                  true,
		"checkoutFinalizeEndpoint": fmt.Sprintf(checkoutFinalizePathPattern, userID),
		"apiResponse":              result,
	}

	if redirect := extractCheckoutRedirect(result); redirect != nil {
		response["finalized"] = false
		response["requiresAction"] = true
		response["redirect"] = redirect
		return response, nil
	}

	ordersAfterAny, _ := checkoutOptionalRequest(s, "GET", fmt.Sprintf("/app/commerce/api/v1/users/%s/orders", userID), httpclient.RequestOpts{
		Query: []string{"pageIndex=0", "pageSize=20"},
	})
	ordersAfter := extractOrdersList(ordersAfterAny)
	orderID := extractCheckoutOrderID(result)
	if orderID == "" {
		orderID = detectNewOrderID(ordersBefore, ordersAfter)
	}

	response["finalized"] = orderID != ""
	if orderID != "" {
		response["orderId"] = orderID
		readback := map[string]any{}
		if orderDetails, err := checkoutRequestJSON(s, "GET", fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s", userID, orderID), httpclient.RequestOpts{}); err == nil {
			readback["order"] = orderDetails
		}
		if delivery, _ := checkoutOptionalRequest(s, "GET", fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s/delivery", userID, orderID), httpclient.RequestOpts{}); delivery != nil {
			readback["delivery"] = delivery
		}
		if payments, _ := checkoutOptionalRequest(s, "GET", fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s/payments", userID, orderID), httpclient.RequestOpts{}); payments != nil {
			readback["payments"] = payments
		}
		if reservation, _ := checkoutOptionalRequest(s, "GET", fmt.Sprintf("/app/commerce/api/v1/users/%s/cart/reservation", userID), httpclient.RequestOpts{}); reservation != nil {
			readback["reservation"] = reservation
		}
		if len(readback) > 0 {
			response["readback"] = readback
		}
	}
	response["ordersBeforeCount"] = len(ordersBefore)
	response["ordersAfterCount"] = len(ordersAfter)
	return response, nil
}

func checkoutOptionalRequest(s *session.Session, method, path string, opts httpclient.RequestOpts) (any, error) {
	result, err := checkoutRequestJSON(s, method, path, opts)
	if err == nil {
		return result, nil
	}
	if details, ok := httpclient.ParseError(err); ok {
		switch details.Status {
		case 404, 204:
			return nil, nil
		}
	}
	return nil, err
}

func summarizeCheckoutCart(cart any) map[string]any {
	root, ok := cart.(map[string]any)
	if !ok {
		return nil
	}
	products, _ := root["products"].([]any)
	summary := map[string]any{
		"lineCount": len(products),
	}
	quantity := 0
	for _, raw := range products {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		quantity += asInt(row["quantity"])
	}
	summary["quantityTotal"] = quantity
	for _, key := range []string{"total", "totalPrice", "cartValue", "value", "grossValue"} {
		if v, ok := root[key]; ok {
			summary["reportedTotal"] = v
			break
		}
	}
	return summary
}

func extractCheckoutOrderID(v any) string {
	var found string
	var walk func(any)
	walk = func(cur any) {
		if found != "" || cur == nil {
			return
		}
		switch x := cur.(type) {
		case map[string]any:
			for _, key := range []string{"orderId", "idOrder", "orderID"} {
				if id, ok := x[key]; ok {
					if s := strings.TrimSpace(fmt.Sprint(id)); s != "" && s != "<nil>" {
						found = s
						return
					}
				}
			}
			if order, ok := x["order"].(map[string]any); ok {
				walk(order)
			}
			for _, value := range x {
				walk(value)
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(v)
	return found
}

func detectNewOrderID(before, after []map[string]any) string {
	seen := map[string]struct{}{}
	for _, order := range before {
		if id := extractCheckoutOrderID(order); id != "" {
			seen[id] = struct{}{}
		}
	}
	for _, order := range after {
		if id := extractCheckoutOrderID(order); id != "" {
			if _, ok := seen[id]; !ok {
				return id
			}
		}
	}
	return ""
}

func extractCheckoutRedirect(v any) map[string]any {
	var out map[string]any
	var walk func(any)
	walk = func(cur any) {
		if out != nil || cur == nil {
			return
		}
		switch x := cur.(type) {
		case map[string]any:
			for _, key := range []string{"redirectUrl", "redirectURL", "url", "href", "acsUrl", "threeDSUrl"} {
				if value, ok := x[key]; ok {
					url := strings.TrimSpace(fmt.Sprint(value))
					if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
						out = map[string]any{"url": url}
						for _, extra := range []string{"method", "transactionId", "transactionID", "md", "pareq", "paReq", "creq"} {
							if extraVal, ok := x[extra]; ok {
								out[extra] = extraVal
							}
						}
						if fields, ok := x["fields"].(map[string]any); ok {
							out["fields"] = fields
						}
						return
					}
				}
			}
			for _, value := range x {
				walk(value)
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(v)
	return out
}

func checkoutSubcommandNames(cmd *cobra.Command) []string {
	names := make([]string, 0, len(cmd.Commands()))
	for _, subcmd := range cmd.Commands() {
		names = append(names, subcmd.Name())
	}
	sort.Strings(names)
	return names
}

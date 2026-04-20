package mcpserver

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wydrox/martmart-cli/internal/httpclient"
)

// registerOrdersTools registers all orders-related MCP tools.
func registerOrdersTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "orders_list",
		Description: "List user orders (GET /app/commerce/api/v1/users/{user}/orders). Mirrors martmart orders list CLI.",
	}, orToolOrdersList)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "orders_details",
		Description: "Order details (GET .../users/{user}/orders/{orderId}). Mirrors martmart orders get.",
	}, orToolOrdersDetails)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "orders_delivery",
		Description: "Order delivery details (GET .../users/{user}/orders/{orderId}/delivery). Mirrors martmart orders delivery.",
	}, orToolOrdersDelivery)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "orders_payments",
		Description: "Order payments details (GET .../users/{user}/orders/{orderId}/payments). Mirrors martmart orders payments.",
	}, orToolOrdersPayments)
}

// orOrdersListIn is the input type for the orders_list tool.
type orOrdersListIn struct {
	UserID    string `json:"user_id,omitempty" jsonschema:"Optional override, defaults to session user_id"`
	PageIndex int    `json:"page_index,omitempty" jsonschema:"1-based page index, default 1"`
	PageSize  int    `json:"page_size,omitempty" jsonschema:"Page size, default 10"`
	AllPages  bool   `json:"all_pages,omitempty" jsonschema:"Fetch every page until empty or cap"`
	Raw       bool   `json:"raw,omitempty" jsonschema:"If true, only api_response (no compact summary)"`
}

// orOrdersListOut is the output type for the orders_list tool.
type orOrdersListOut struct {
	APIResponse map[string]any   `json:"api_response" jsonschema:"Normalized Frisco API JSON payload"`
	Summary     map[string]any   `json:"summary,omitempty"`
	Orders      []map[string]any `json:"orders,omitempty"`
}

// orOrdersDetailsIn is the input type for the orders_details tool.
type orOrdersDetailsIn struct {
	UserID  string `json:"user_id,omitempty"`
	OrderID string `json:"order_id" jsonschema:"Order id"`
}

// orOrdersDetailsOut is the output type for the orders_details tool.
type orOrdersDetailsOut struct {
	APIResponse map[string]any `json:"api_response"`
}

// orOrdersDeliveryIn is the input type for the orders_delivery tool.
type orOrdersDeliveryIn struct {
	UserID  string `json:"user_id,omitempty"`
	OrderID string `json:"order_id" jsonschema:"Order id"`
}

// orOrdersDeliveryOut is the output type for the orders_delivery tool.
type orOrdersDeliveryOut struct {
	APIResponse map[string]any `json:"api_response"`
}

// orOrdersPaymentsIn is the input type for the orders_payments tool.
type orOrdersPaymentsIn struct {
	UserID  string `json:"user_id,omitempty"`
	OrderID string `json:"order_id" jsonschema:"Order id"`
}

// orOrdersPaymentsOut is the output type for the orders_payments tool.
type orOrdersPaymentsOut struct {
	APIResponse map[string]any `json:"api_response"`
}

func orToolOrdersList(_ context.Context, _ *mcp.CallToolRequest, in orOrdersListIn) (*mcp.CallToolResult, orOrdersListOut, error) {
	s, uid, err := loadSessionAuth(in.UserID)
	if err != nil {
		return nil, orOrdersListOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders", uid)
	pageIndex := in.PageIndex
	if pageIndex <= 0 {
		pageIndex = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	var result any
	if in.AllPages {
		var allItems []map[string]any
		pi := pageIndex
		for {
			q := []string{
				fmt.Sprintf("pageIndex=%d", pi),
				fmt.Sprintf("pageSize=%d", pageSize),
			}
			payload, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
			if err != nil {
				return nil, orOrdersListOut{}, err
			}
			items := orExtractOrdersList(payload)
			if len(items) == 0 {
				break
			}
			allItems = append(allItems, items...)
			if len(items) < pageSize {
				break
			}
			pi++
			if pi-pageIndex > 100 {
				break
			}
		}
		result = allItems
	} else {
		q := []string{
			fmt.Sprintf("pageIndex=%d", pageIndex),
			fmt.Sprintf("pageSize=%d", pageSize),
		}
		result, err = httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: q})
		if err != nil {
			return nil, orOrdersListOut{}, err
		}
	}
	out := orOrdersListOut{APIResponse: orNormalizeAPIResponse(result)}
	if in.Raw {
		return nil, out, nil
	}
	items := orExtractOrdersList(result)
	var compact []map[string]any
	for _, order := range items {
		id := order["id"]
		if id == nil {
			id = order["orderId"]
		}
		st := order["status"]
		if st == nil {
			st = order["orderStatus"]
		}
		row := map[string]any{
			"id":        id,
			"status":    st,
			"createdAt": orExtractOrderDatetime(order),
		}
		if t := orExtractOrderTotal(order); t != nil {
			row["totalPLN"] = math.Round(*t*100) / 100
		} else {
			row["totalPLN"] = nil
		}
		compact = append(compact, row)
	}
	var totalVals []float64
	for _, x := range compact {
		if v, ok := x["totalPLN"].(float64); ok {
			totalVals = append(totalVals, v)
		}
	}
	summary := map[string]any{"count": len(compact)}
	if len(totalVals) > 0 {
		var sum float64
		for _, v := range totalVals {
			sum += v
		}
		summary["sumPLN"] = math.Round(sum*100) / 100
		summary["avgPLN"] = math.Round(sum/float64(len(totalVals))*100) / 100
	} else {
		summary["sumPLN"] = nil
		summary["avgPLN"] = nil
	}
	out.Summary = summary
	out.Orders = compact
	return nil, out, nil
}

func orToolOrdersDetails(_ context.Context, _ *mcp.CallToolRequest, in orOrdersDetailsIn) (*mcp.CallToolResult, orOrdersDetailsOut, error) {
	if strings.TrimSpace(in.OrderID) == "" {
		return nil, orOrdersDetailsOut{}, fmt.Errorf("order_id is required")
	}
	s, uid, err := loadSessionAuth(in.UserID)
	if err != nil {
		return nil, orOrdersDetailsOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s", uid, in.OrderID)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, orOrdersDetailsOut{}, err
	}
	return nil, orOrdersDetailsOut{APIResponse: orNormalizeAPIResponse(result)}, nil
}

func orToolOrdersDelivery(_ context.Context, _ *mcp.CallToolRequest, in orOrdersDeliveryIn) (*mcp.CallToolResult, orOrdersDeliveryOut, error) {
	if strings.TrimSpace(in.OrderID) == "" {
		return nil, orOrdersDeliveryOut{}, fmt.Errorf("order_id is required")
	}
	s, uid, err := loadSessionAuth(in.UserID)
	if err != nil {
		return nil, orOrdersDeliveryOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s/delivery", uid, in.OrderID)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, orOrdersDeliveryOut{}, err
	}
	return nil, orOrdersDeliveryOut{APIResponse: orNormalizeAPIResponse(result)}, nil
}

func orToolOrdersPayments(_ context.Context, _ *mcp.CallToolRequest, in orOrdersPaymentsIn) (*mcp.CallToolResult, orOrdersPaymentsOut, error) {
	if strings.TrimSpace(in.OrderID) == "" {
		return nil, orOrdersPaymentsOut{}, fmt.Errorf("order_id is required")
	}
	s, uid, err := loadSessionAuth(in.UserID)
	if err != nil {
		return nil, orOrdersPaymentsOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/orders/%s/payments", uid, in.OrderID)
	result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, orOrdersPaymentsOut{}, err
	}
	return nil, orOrdersPaymentsOut{APIResponse: orNormalizeAPIResponse(result)}, nil
}

// --- Helpers for orders ---

// orExtractOrdersList extracts an orders slice from various API response shapes.
func orExtractOrdersList(payload any) []map[string]any {
	switch p := payload.(type) {
	case []map[string]any:
		return p
	case []any:
		var out []map[string]any
		for _, x := range p {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		for _, key := range []string{"items", "orders", "results", "data"} {
			if v, ok := p[key].([]any); ok {
				var out []map[string]any
				for _, x := range v {
					if m, ok := x.(map[string]any); ok {
						out = append(out, m)
					}
				}
				return out
			}
		}
	}
	return nil
}

// orExtractOrderDatetime returns the first non-empty date/time string found in an order map.
func orExtractOrderDatetime(order map[string]any) string {
	for _, key := range []string{"createdAt", "created", "placedAt", "orderDate", "date"} {
		if v, ok := order[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// orExtractOrderTotal searches common pricing fields in an order map and returns
// the largest positive value found, or nil when no numeric total is present.
func orExtractOrderTotal(order map[string]any) *float64 {
	var candidates []float64
	for _, key := range []string{"total", "totalValue", "amount", "grossValue", "orderValue", "finalPrice"} {
		orAddNumber(order[key], &candidates)
		if m, ok := order[key].(map[string]any); ok {
			orAddNumber(m["_total"], &candidates)
		}
	}
	for _, sectionKey := range []string{"pricing", "payment", "summary", "totals", "orderPricing"} {
		section, ok := order[sectionKey].(map[string]any)
		if !ok {
			continue
		}
		for _, valueKey := range []string{
			"totalPayment",
			"totalWithDeliveryCostAfterVoucherPayment",
			"totalWithDeliveryCost",
			"total",
		} {
			orAddNumber(section[valueKey], &candidates)
			if m, ok := section[valueKey].(map[string]any); ok {
				orAddNumber(m["_total"], &candidates)
			}
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	var positives []float64
	for _, x := range candidates {
		if x > 0 {
			positives = append(positives, x)
		}
	}
	var best float64
	if len(positives) > 0 {
		best = positives[0]
		for _, x := range positives[1:] {
			if x > best {
				best = x
			}
		}
	} else {
		best = candidates[0]
		for _, x := range candidates[1:] {
			if x > best {
				best = x
			}
		}
	}
	return &best
}

// orAddNumber appends v to candidates if v is a numeric type.
func orAddNumber(v any, candidates *[]float64) {
	switch n := v.(type) {
	case float64:
		*candidates = append(*candidates, n)
	case int:
		*candidates = append(*candidates, float64(n))
	case int64:
		*candidates = append(*candidates, float64(n))
	}
}

// orTruthy returns true when v is a bool with value true.
func orTruthy(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

// orNonEmptyStr converts v to a trimmed string and reports whether it is non-empty.
func orNonEmptyStr(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s, s != ""
}

// orNormalizeAPIResponse keeps output schema stable as an object while preserving payload.
func orNormalizeAPIResponse(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{"value": v}
}

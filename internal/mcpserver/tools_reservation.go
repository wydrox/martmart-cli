package mcpserver

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
)

// registerReservationTools registers all reservation-related MCP tools.
func registerReservationTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_delivery_options",
		Description: "Delivery and payment options by postcode. Mirrors martmart reservation delivery-options.",
	}, orToolReservationDeliveryOptions)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_calendar",
		Description: "Calendar data for shipping address; optional date. Mirrors martmart reservation calendar.",
	}, orToolReservationCalendar)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_slots",
		Description: "Available delivery slots for consecutive days. Mirrors martmart reservation slots.",
	}, orToolReservationSlots)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_reserve",
		Description: "Reserve a cart delivery window by date and HH:MM range. Mirrors martmart reservation reserve.",
	}, orToolReservationReserve)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_plan",
		Description: "Plan cart reservation from payload JSON object. Mirrors martmart reservation plan.",
	}, orToolReservationPlan)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reservation_cancel",
		Description: "Cancel active cart reservation (DELETE .../cart/reservation). Mirrors martmart reservation cancel.",
	}, orToolReservationCancel)
}

// orReservationDeliveryOptionsIn is the input type for the reservation_delivery_options tool.
type orReservationDeliveryOptionsIn struct {
	Postcode string `json:"postcode" jsonschema:"postcode, e.g. 00-001"`
}

// orReservationDeliveryOptionsOut is the output type for the reservation_delivery_options tool.
type orReservationDeliveryOptionsOut struct {
	APIResponse map[string]any `json:"api_response"`
}

// orReservationCalendarIn is the input type for the reservation_calendar tool.
type orReservationCalendarIn struct {
	UserID          string         `json:"user_id,omitempty"`
	ShippingAddress map[string]any `json:"shipping_address" jsonschema:"shipping address object"`
	Date            string         `json:"date,omitempty" jsonschema:"optional YYYY-M-D or YYYY-MM-DD"`
}

// orReservationCalendarOut is the output type for the reservation_calendar tool.
type orReservationCalendarOut struct {
	APIResponse map[string]any `json:"api_response"`
}

// orReservationSlotsIn is the input type for the reservation_slots tool.
type orReservationSlotsIn struct {
	UserID          string         `json:"user_id,omitempty"`
	Days            int            `json:"days,omitempty" jsonschema:"Consecutive days to check, default 3"`
	StartDate       string         `json:"start_date,omitempty" jsonschema:"YYYY-MM-DD, default today"`
	ShippingAddress map[string]any `json:"shipping_address,omitempty" jsonschema:"Optional, defaults to account shipping address"`
	Raw             bool           `json:"raw,omitempty"`
}

// orReservationSlotsOut is the output type for the reservation_slots tool.
type orReservationSlotsOut struct {
	APIByDate map[string]any   `json:"api_by_date,omitempty" jsonschema:"Per-day raw API payloads keyed by YYYY-MM-DD"`
	Days      []map[string]any `json:"days,omitempty" jsonschema:"Pretty slots per day when raw is false"`
}

// orReservationReserveIn is the input type for the reservation_reserve tool.
type orReservationReserveIn struct {
	UserID          string         `json:"user_id,omitempty"`
	Date            string         `json:"date" jsonschema:"YYYY-MM-DD"`
	FromTime        string         `json:"from_time" jsonschema:"HH:MM window start"`
	ToTime          string         `json:"to_time" jsonschema:"HH:MM window end"`
	ShippingAddress map[string]any `json:"shipping_address,omitempty"`
}

// orReservationReserveOut is the output type for the reservation_reserve tool.
type orReservationReserveOut struct {
	Reserved    bool           `json:"reserved"`
	Slot        map[string]any `json:"slot,omitempty"`
	APIResponse map[string]any `json:"api_response"`
}

// orReservationCancelIn is the input type for the reservation_cancel tool.
type orReservationCancelIn struct {
	UserID string `json:"user_id,omitempty"`
}

// orReservationCancelOut is the output type for the reservation_cancel tool.
type orReservationCancelOut struct {
	APIResponse map[string]any `json:"api_response"`
}

// orReservationPlanIn is the input type for the reservation_plan tool.
type orReservationPlanIn struct {
	UserID  string         `json:"user_id,omitempty"`
	Payload map[string]any `json:"payload" jsonschema:"reservation payload object"`
}

// orReservationPlanOut is the output type for the reservation_plan tool.
type orReservationPlanOut struct {
	APIResponse map[string]any `json:"api_response"`
}

func orToolReservationDeliveryOptions(_ context.Context, _ *mcp.CallToolRequest, in orReservationDeliveryOptionsIn) (*mcp.CallToolResult, orReservationDeliveryOptionsOut, error) {
	if strings.TrimSpace(in.Postcode) == "" {
		return nil, orReservationDeliveryOptionsOut{}, fmt.Errorf("postcode is required")
	}
	s, err := session.Load()
	if err != nil {
		return nil, orReservationDeliveryOptionsOut{}, err
	}
	if !session.IsAuthenticated(s) {
		return nil, orReservationDeliveryOptionsOut{}, errNotAuthenticated
	}
	result, err := httpclient.RequestJSON(s, "GET", "/app/commerce/api/v1/calendar/delivery-payment", httpclient.RequestOpts{
		Query: []string{"postcode=" + url.QueryEscape(in.Postcode)},
	})
	if err != nil {
		return nil, orReservationDeliveryOptionsOut{}, err
	}
	return nil, orReservationDeliveryOptionsOut{APIResponse: orNormalizeAPIResponse(result)}, nil
}

func orToolReservationCalendar(_ context.Context, _ *mcp.CallToolRequest, in orReservationCalendarIn) (*mcp.CallToolResult, orReservationCalendarOut, error) {
	if len(in.ShippingAddress) == 0 {
		return nil, orReservationCalendarOut{}, fmt.Errorf("shipping_address is required")
	}
	s, uid, err := loadSessionAuth(in.UserID)
	if err != nil {
		return nil, orReservationCalendarOut{}, err
	}
	body := map[string]any{"shippingAddress": in.ShippingAddress}

	path := fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van", uid)
	if strings.TrimSpace(in.Date) != "" {
		y, m, d, err := parseCalendarDate(in.Date)
		if err != nil {
			return nil, orReservationCalendarOut{}, err
		}
		path = fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van/%d/%d/%d", uid, y, m, d)
	}
	result, err := httpclient.RequestJSON(s, "POST", path, httpclient.RequestOpts{
		Data:       body,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, orReservationCalendarOut{}, err
	}
	return nil, orReservationCalendarOut{APIResponse: orNormalizeAPIResponse(result)}, nil
}

func orToolReservationSlots(_ context.Context, _ *mcp.CallToolRequest, in orReservationSlotsIn) (*mcp.CallToolResult, orReservationSlotsOut, error) {
	s, uid, err := loadSessionAuth(in.UserID)
	if err != nil {
		return nil, orReservationSlotsOut{}, err
	}
	days := in.Days
	if days <= 0 {
		days = 3
	}
	var shippingAddress map[string]any
	if len(in.ShippingAddress) > 0 {
		shippingAddress = in.ShippingAddress
	} else {
		shippingAddress, err = orGetShippingAddressFromAccount(s, uid)
		if err != nil {
			return nil, orReservationSlotsOut{}, err
		}
	}
	var baseDate time.Time
	if in.StartDate != "" {
		baseDate, err = time.Parse("2006-01-02", in.StartDate)
		if err != nil {
			return nil, orReservationSlotsOut{}, err
		}
	} else {
		baseDate = time.Now().Truncate(24 * time.Hour)
	}
	allDays := map[string]any{}
	var pretty []map[string]any
	for i := 0; i < days; i++ {
		d := baseDate.AddDate(0, 0, i)
		calPath := fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van/%d/%d/%d",
			uid, d.Year(), int(d.Month()), d.Day())
		dayData, err := httpclient.RequestJSON(s, "POST", calPath, httpclient.RequestOpts{
			Data:       map[string]any{"shippingAddress": shippingAddress},
			DataFormat: httpclient.FormatJSON,
		})
		if err != nil {
			return nil, orReservationSlotsOut{}, err
		}
		dayKey := d.Format("2006-01-02")
		allDays[dayKey] = dayData
		pretty = append(pretty, map[string]any{
			"date":  dayKey,
			"slots": orExtractDeliveryWindows(dayData),
		})
	}
	out := orReservationSlotsOut{APIByDate: allDays}
	if in.Raw {
		return nil, out, nil
	}
	out.Days = pretty
	return nil, out, nil
}

func orToolReservationReserve(_ context.Context, _ *mcp.CallToolRequest, in orReservationReserveIn) (*mcp.CallToolResult, orReservationReserveOut, error) {
	s, uid, err := loadSessionAuth(in.UserID)
	if err != nil {
		return nil, orReservationReserveOut{}, err
	}
	targetDate, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return nil, orReservationReserveOut{}, err
	}
	var shippingAddress map[string]any
	if len(in.ShippingAddress) > 0 {
		shippingAddress = in.ShippingAddress
	} else {
		shippingAddress, err = orGetShippingAddressFromAccount(s, uid)
		if err != nil {
			return nil, orReservationReserveOut{}, err
		}
	}
	calPath := fmt.Sprintf("/app/commerce/api/v2/users/%s/calendar/Van/%d/%d/%d",
		uid, targetDate.Year(), int(targetDate.Month()), targetDate.Day())
	dayData, err := httpclient.RequestJSON(s, "POST", calPath, httpclient.RequestOpts{
		Data:       map[string]any{"shippingAddress": shippingAddress},
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, orReservationReserveOut{}, err
	}
	windows := orExtractReservableWindows(dayData)
	if len(windows) == 0 {
		return nil, orReservationReserveOut{}, fmt.Errorf("no reservable slots for the given date")
	}
	var selected map[string]any
	var possible []string
	for _, w := range windows {
		startsAt := fmt.Sprint(w["startsAt"])
		endsAt := fmt.Sprint(w["endsAt"])
		d1, h1 := orDateAndHHMMFromISO(startsAt)
		d2, h2 := orDateAndHHMMFromISO(endsAt)
		if d1 == in.Date && d2 == in.Date {
			possible = append(possible, fmt.Sprintf("%s-%s", h1, h2))
		}
		if d1 == in.Date && d2 == in.Date && h1 == in.FromTime && h2 == in.ToTime {
			selected = w
			break
		}
	}
	if selected == nil {
		return nil, orReservationReserveOut{}, fmt.Errorf("slot %s-%s not found for %s; available: %s",
			in.FromTime, in.ToTime, in.Date, strings.Join(possible, ", "))
	}
	payload := map[string]any{
		"extendedRange":   nil,
		"deliveryWindow":  selected,
		"shippingAddress": shippingAddress,
	}
	reservePath := fmt.Sprintf("/app/commerce/api/v2/users/%s/cart/reservation", uid)
	result, err := httpclient.RequestJSON(s, "POST", reservePath, httpclient.RequestOpts{
		Data:       payload,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, orReservationReserveOut{}, err
	}
	out := orReservationReserveOut{
		Reserved: true,
		Slot: map[string]any{
			"startsAt":       selected["startsAt"],
			"endsAt":         selected["endsAt"],
			"deliveryMethod": selected["deliveryMethod"],
			"warehouse":      selected["warehouse"],
		},
		APIResponse: orNormalizeAPIResponse(result),
	}
	return nil, out, nil
}

func orToolReservationCancel(_ context.Context, _ *mcp.CallToolRequest, in orReservationCancelIn) (*mcp.CallToolResult, orReservationCancelOut, error) {
	s, uid, err := loadSessionAuth(in.UserID)
	if err != nil {
		return nil, orReservationCancelOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/cart/reservation", uid)
	result, err := httpclient.RequestJSON(s, "DELETE", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, orReservationCancelOut{}, err
	}
	return nil, orReservationCancelOut{APIResponse: orNormalizeAPIResponse(result)}, nil
}

func orToolReservationPlan(_ context.Context, _ *mcp.CallToolRequest, in orReservationPlanIn) (*mcp.CallToolResult, orReservationPlanOut, error) {
	if len(in.Payload) == 0 {
		return nil, orReservationPlanOut{}, fmt.Errorf("payload is required")
	}
	s, uid, err := loadSessionAuth(in.UserID)
	if err != nil {
		return nil, orReservationPlanOut{}, err
	}
	path := fmt.Sprintf("/app/commerce/api/v2/users/%s/cart/reservation", uid)
	result, err := httpclient.RequestJSON(s, "POST", path, httpclient.RequestOpts{
		Data:       in.Payload,
		DataFormat: httpclient.FormatJSON,
	})
	if err != nil {
		return nil, orReservationPlanOut{}, err
	}
	return nil, orReservationPlanOut{APIResponse: orNormalizeAPIResponse(result)}, nil
}

// --- Helpers for reservation ---

// orGetShippingAddressFromAccount fetches the user's saved shipping addresses and
// returns the preferred (default/current) one, falling back to the first entry.
func orGetShippingAddressFromAccount(s *session.Session, userID string) (map[string]any, error) {
	path := fmt.Sprintf("/app/commerce/api/v1/users/%s/addresses/shipping-addresses", userID)
	data, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{})
	if err != nil {
		return nil, err
	}
	list, ok := data.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("no saved shipping addresses for user")
	}
	var preferred map[string]any
	for _, item := range list {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if orTruthy(row["isDefault"]) || orTruthy(row["isCurrent"]) || orTruthy(row["isSelected"]) {
			preferred = row
			break
		}
	}
	chosen := preferred
	if chosen == nil {
		if row, ok := list[0].(map[string]any); ok {
			chosen = row
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("no saved shipping addresses for user")
	}
	if sa, ok := chosen["shippingAddress"].(map[string]any); ok {
		return sa, nil
	}
	return chosen, nil
}

// orExtractDeliveryWindows recursively walks a calendar API response and collects
// unique delivery window objects (those with startsAt and endsAt), sorted by start time.
func orExtractDeliveryWindows(data any) []map[string]any {
	var windows []map[string]any
	var walk func(any)
	walk = func(obj any) {
		switch o := obj.(type) {
		case map[string]any:
			_, sok := orNonEmptyStr(o["startsAt"])
			_, eok := orNonEmptyStr(o["endsAt"])
			if sok && eok {
				windows = append(windows, map[string]any{
					"startsAt":       o["startsAt"],
					"endsAt":         o["endsAt"],
					"deliveryMethod": o["deliveryMethod"],
					"warehouse":      o["warehouse"],
					"closesAt":       o["closesAt"],
					"finalAt":        o["finalAt"],
				})
			}
			for _, v := range o {
				walk(v)
			}
		case []any:
			for _, v := range o {
				walk(v)
			}
		}
	}
	walk(data)
	uniq := make(map[string]map[string]any)
	for _, w := range windows {
		key := fmt.Sprintf("%v|%v|%v|%v", w["startsAt"], w["endsAt"], w["deliveryMethod"], w["warehouse"])
		uniq[key] = w
	}
	out := make([]map[string]any, 0, len(uniq))
	for _, w := range uniq {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		return fmt.Sprint(out[i]["startsAt"]) < fmt.Sprint(out[j]["startsAt"])
	})
	return out
}

// orExtractReservableWindows is like orExtractDeliveryWindows but only returns windows
// that also have a deliveryMethod and warehouse (required for the reserve API call).
func orExtractReservableWindows(data any) []map[string]any {
	var windows []map[string]any
	var walk func(any)
	walk = func(obj any) {
		switch o := obj.(type) {
		case map[string]any:
			_, sok := orNonEmptyStr(o["startsAt"])
			_, eok := orNonEmptyStr(o["endsAt"])
			_, dmok := orNonEmptyStr(o["deliveryMethod"])
			_, whok := orNonEmptyStr(o["warehouse"])
			if sok && eok && dmok && whok {
				windows = append(windows, o)
			}
			for _, v := range o {
				walk(v)
			}
		case []any:
			for _, v := range o {
				walk(v)
			}
		}
	}
	walk(data)
	uniq := make(map[string]map[string]any)
	for _, w := range windows {
		key := fmt.Sprintf("%v|%v|%v|%v", w["startsAt"], w["endsAt"], w["deliveryMethod"], w["warehouse"])
		uniq[key] = w
	}
	out := make([]map[string]any, 0, len(uniq))
	for _, w := range uniq {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		return fmt.Sprint(out[i]["startsAt"]) < fmt.Sprint(out[j]["startsAt"])
	})
	return out
}

// orDateAndHHMMFromISO splits an ISO 8601 timestamp into a date part (YYYY-MM-DD)
// and an HH:MM time part.
func orDateAndHHMMFromISO(ts string) (datePart, hhmm string) {
	idx := strings.IndexByte(ts, 'T')
	if idx < 0 {
		return ts, ""
	}
	end := idx + 6
	if end > len(ts) {
		end = len(ts)
	}
	return ts[:idx], ts[idx+1 : end]
}

// parseCalendarDate parses a YYYY-M-D or YYYY-MM-DD date string into year, month, day.
func parseCalendarDate(date string) (int, int, int, error) {
	parts := strings.Split(date, "-")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("date must be in format YYYY-M-D or YYYY-MM-DD")
	}
	y, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid year in date: %w", err)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid month in date: %w", err)
	}
	d, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid day in date: %w", err)
	}
	return y, m, d, nil
}

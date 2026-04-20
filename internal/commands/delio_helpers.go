package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/delio"
	"github.com/wydrox/martmart-cli/internal/picker"
	"github.com/wydrox/martmart-cli/internal/session"
	"github.com/wydrox/martmart-cli/internal/shared"
)

func providerIs(name string) bool {
	return strings.EqualFold(session.CurrentProvider(), name)
}

func providerUnsupported(commandName string) error {
	return fmt.Errorf("%s is not implemented for provider %q", commandName, session.CurrentProvider())
}

func delioCoordsFromFlags(cmd *cobra.Command, lat, long float64) (*delio.Coordinates, error) {
	latChanged := cmd.Flags().Changed("lat")
	longChanged := cmd.Flags().Changed("long")
	if !latChanged && !longChanged {
		return nil, nil
	}
	if latChanged != longChanged {
		return nil, fmt.Errorf("provide both --lat and --long")
	}
	return &delio.Coordinates{Lat: lat, Long: long}, nil
}

func delioMoneyString(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return "-"
	}
	cent, ok := m["centAmount"].(float64)
	if !ok {
		return "-"
	}
	return fmt.Sprintf("%.2f", cent/100.0)
}

func delioProductDiscountedPrice(product map[string]any) string {
	price, _ := product["price"].(map[string]any)
	if price == nil {
		return "-"
	}
	discounted, _ := price["discounted"].(map[string]any)
	if discounted == nil {
		return "-"
	}
	return delioMoneyString(discounted["value"])
}

func delioListField(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	out, _ := m[key].([]any)
	return out
}

func extractDelioSearchResults(v any) ([]map[string]any, error) {
	search, err := delio.ExtractProductSearch(v)
	if err != nil {
		return nil, err
	}
	raw := delioListField(search, "results")
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func printDelioProductSearchTable(v any) error {
	results, err := extractDelioSearchResults(v)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SKU\tNAME\tPRICE\tDISCOUNTED\tAVAILABLE\tSLUG")
	for _, p := range results {
		price, _ := p["price"].(map[string]any)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			asString(p["sku"]),
			shared.TruncateText(asString(p["name"]), 54),
			delioMoneyString(price["value"]),
			delioProductDiscountedPrice(p),
			asString(p["availableQuantity"]),
			asString(p["slug"]),
		)
	}
	_ = w.Flush()
	return nil
}

func printDelioCartSummary(v any) error {
	cart, err := delio.ExtractCurrentCart(v)
	if err != nil {
		return err
	}
	items := delioListField(cart, "lineItems")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SKU\tNAME\tQTY\tUNIT PRICE\tLINE TOTAL")
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		product, _ := row["product"].(map[string]any)
		price, _ := product["price"].(map[string]any)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			asString(product["sku"]),
			shared.TruncateText(asString(product["name"]), 54),
			asString(row["quantity"]),
			delioMoneyString(price["value"]),
			delioMoneyString(row["totalPrice"]),
		)
	}
	_ = w.Flush()
	if total := delioMoneyString(cart["totalPrice"]); total != "-" {
		_, _ = fmt.Printf("\nCart total: %s\n", total)
	}
	return nil
}

type delioSlotRow struct {
	date          string
	from          string
	to            string
	available     string
	bookableUntil string
	dateFrom      time.Time
	dateTo        time.Time
}

func printDelioSlotsTable(v any, startDate string, days int) error {
	rawSlots, err := delio.ExtractDeliveryScheduleSlots(v)
	if err != nil {
		return err
	}
	rows := make([]delioSlotRow, 0, len(rawSlots))
	var base time.Time
	if strings.TrimSpace(startDate) != "" {
		base, err = time.Parse("2006-01-02", startDate)
		if err != nil {
			return err
		}
	}
	for _, item := range rawSlots {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fromRaw := asString(m["dateFrom"])
		toRaw := asString(m["dateTo"])
		fromTS, err1 := time.Parse(time.RFC3339, fromRaw)
		toTS, err2 := time.Parse(time.RFC3339, toRaw)
		if err1 != nil || err2 != nil {
			continue
		}
		if !base.IsZero() && fromTS.Before(base) {
			continue
		}
		if !base.IsZero() && days > 0 && fromTS.After(base.AddDate(0, 0, days)) {
			continue
		}
		rows = append(rows, delioSlotRow{
			date:          fromTS.Format("2006-01-02"),
			from:          fromTS.Format("15:04"),
			to:            toTS.Format("15:04"),
			available:     asString(m["available"]),
			bookableUntil: asString(m["bookableUntil"]),
			dateFrom:      fromTS,
			dateTo:        toTS,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].dateFrom.Before(rows[j].dateFrom) })
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "DATE\tFROM\tTO\tAVAILABLE\tBOOKABLE UNTIL")
	for _, row := range rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", row.date, row.from, row.to, row.available, row.bookableUntil)
	}
	_ = w.Flush()
	return nil
}

func resolveDelioProductBySearch(s *session.Session, phrase string, coords *delio.Coordinates) (string, error) {
	result, err := delio.SearchProducts(s, phrase, 10, 0, coords)
	if err != nil {
		return "", fmt.Errorf("Delio product search failed: %w", err)
	}
	results, err := extractDelioSearchResults(result)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", fmt.Errorf("no Delio products found for search phrase %q", phrase)
	}
	bestScore := -1.0
	bestIdx := -1
	for i, p := range results {
		candidate := strings.TrimSpace(strings.Join([]string{asString(p["name"]), asString(p["slug"]), asString(p["sku"])}, " "))
		score := picker.Score(candidate, phrase)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return "", fmt.Errorf("no usable Delio result for %q", phrase)
	}
	best := results[bestIdx]
	fmt.Printf("Picked: %s  [%s]\n", asString(best["name"]), asString(best["sku"]))
	return asString(best["sku"]), nil
}

func delioCartItemQuantity(cart map[string]any, sku string) int {
	for _, item := range delioListField(cart, "lineItems") {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		product, _ := row["product"].(map[string]any)
		if strings.EqualFold(asString(product["sku"]), sku) {
			return asInt(row["quantity"])
		}
	}
	return 0
}

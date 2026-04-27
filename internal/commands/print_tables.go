// Tabular/console renderers for Frisco API payloads. Shared formatting helpers
// (cellValue, hhmm, formatStreet, parseMoneyFloat, etc.) and display-adjacent
// types (cartShowOpts, cartItem, orderProduct) live next to the commands that
// use them outside of printing.
package commands

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/wydrox/martmart-cli/internal/shared"
)

// printProfileTable renders a user profile API response as a key-value table.
func printProfileTable(v any) error {
	profile, ok := v.(map[string]any)
	if !ok {
		return printJSON(v)
	}

	// Build Name from fullName.firstName + lastName.
	name := "—"
	if fn, ok := profile["fullName"].(map[string]any); ok {
		first := cellValue(fn["firstName"])
		last := cellValue(fn["lastName"])
		parts := []string{}
		if first != "—" {
			parts = append(parts, first)
		}
		if last != "—" {
			parts = append(parts, last)
		}
		if len(parts) > 0 {
			name = strings.Join(parts, " ")
		}
	}

	// Extract registeredAt as YYYY-MM-DD.
	registered := cellValue(profile["registeredAt"])
	if len(registered) >= 10 {
		registered = registered[:10]
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	rows := []struct{ label, value string }{
		{"Name", name},
		{"Email", cellValue(profile["email"])},
		{"Phone", cellValue(profile["phoneNumber"])},
		{"Postcode", cellValue(profile["postcode"])},
		{"Language", cellValue(profile["language"])},
		{"Profile", cellValue(profile["profileType"])},
		{"Adult", cellValue(profile["isAdult"])},
		{"Registered", registered},
	}
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", r.label, r.value)
	}
	return w.Flush()
}

// printAddressesTable renders a shipping address list as a tabwriter table.
func printAddressesTable(v any) error {
	list, ok := v.([]any)
	if !ok {
		return printJSON(v)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "id\trecipient\tstreet\tcity\tpostcode\tphone")
	for _, item := range list {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := cellValue(row["id"])
		addr, _ := row["shippingAddress"].(map[string]any)
		recipient := cellValue(addr["recipient"])
		street := formatStreet(addr)
		city := cellValue(addr["city"])
		postcode := cellValue(addr["postcode"])
		phone := cellValue(addr["phoneNumber"])
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", id, recipient, street, city, postcode, phone)
	}
	return w.Flush()
}

// printConsentsTable renders a consent key→bool map as a sorted tabwriter table.
func printConsentsTable(consents map[string]any) error {
	keys := make([]string, 0, len(consents))
	for k := range consents {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "key\tenabled")
	for _, k := range keys {
		_, _ = fmt.Fprintf(w, "%s\t%v\n", k, consents[k])
	}
	return w.Flush()
}

// printPaymentsTable renders a paginated payments API response as a tabwriter table.
func printPaymentsTable(v any) error {
	page, ok := v.(map[string]any)
	if !ok {
		return printJSON(v)
	}

	// Pagination info.
	pageIndex := int(toFloat(page["pageIndex"]))
	pageCount := int(toFloat(page["pageCount"]))
	totalCount := int(toFloat(page["totalCount"]))
	fmt.Printf("Page %d/%d (%d total)\n\n", pageIndex, pageCount, totalCount)

	items := toSlice(page["items"])
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "date\tstatus\tchannel\tcard\torderId")
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		date := cellValue(row["createdAt"])
		if len(date) >= 10 {
			date = date[:10]
		}
		status := cellValue(row["status"])
		channel := cellValue(row["channelName"])
		card := cellValue(row["creditCardBrand"])
		orderID := cellValue(row["orderId"])
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", date, status, channel, card, orderID)
	}
	return w.Flush()
}

// printPointsHistoryTable renders a paginated membership points response as a tabwriter table.
func printPointsHistoryTable(v any) error {
	page, ok := v.(map[string]any)
	if !ok {
		return printJSON(v)
	}
	items := toSlice(page["items"])
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "date\toperation\tpoints\torderId")
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		date := cellValue(row["createdAt"])
		if len(date) >= 10 {
			date = date[:10]
		}
		operation := cellValue(row["operation"])
		points := cellValue(row["membershipPoints"])
		orderID := cellValue(row["orderId"])
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", date, operation, points, orderID)
	}
	return w.Flush()
}

// printOrderProductsTable renders products as a tabwriter table and prints a
// summary line. sortBy may be "total", "name", or "" (API order).
func printOrderProductsTable(products []orderProduct, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "total":
		sort.Slice(products, func(i, j int) bool {
			return products[i].Total > products[j].Total
		})
	case "name":
		sort.Slice(products, func(i, j int) bool {
			return strings.ToLower(products[i].Name) < strings.ToLower(products[j].Name)
		})
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tQTY\tPRICE\tTOTAL\tGRAMMAGE\tUNIT")
	for _, p := range products {
		qty := fmt.Sprintf("%.0f", p.Quantity)
		price := fmt.Sprintf("%.2f", p.Price)
		total := fmt.Sprintf("%.2f", p.Total)
		grammage := ""
		if p.Grammage != 0 {
			grammage = fmt.Sprintf("%g", p.Grammage)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			p.Name, qty, price, total, grammage, p.Unit)
	}
	_ = w.Flush()

	var orderTotal float64
	for _, p := range products {
		orderTotal += p.Total
	}
	fmt.Printf("\nOrder total: %.2f PLN (%d items)\n",
		math.Round(orderTotal*100)/100, len(products))
}

// printProductSearchTable renders a product search API response as a tabwriter table.
func printProductSearchTable(result any) error {
	m, ok := result.(map[string]any)
	if !ok {
		return printPretty(result)
	}

	// Pagination info.
	pageIndex, _ := m["pageIndex"].(float64)
	pageCount, _ := m["pageCount"].(float64)
	totalCount, _ := m["totalCount"].(float64)
	fmt.Printf("Page %.0f/%.0f (%.0f results)\n\n", pageIndex, pageCount, totalCount)

	rawProducts, _ := m["products"].([]any)
	if len(rawProducts) == 0 {
		fmt.Println("No products found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "id\tname\tbrand\tprice\tgrammage\tunit\tprice/kg\tavailable")
	for _, raw := range rawProducts {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := cellValue(entry["productId"])
		inner, _ := entry["product"].(map[string]any)

		name := ""
		brand := ""
		price := ""
		grammage := ""
		unit := ""
		pricePerKg := ""
		available := ""
		if inner != nil {
			name = shared.LocalizedString(inner["name"])
			brand, _ = inner["brand"].(string)
			var priceVal float64
			if priceObj, ok := inner["price"].(map[string]any); ok {
				if pv, ok := priceObj["price"].(float64); ok {
					priceVal = pv
					price = fmt.Sprintf("%.2f", pv)
				}
			}
			var gramVal float64
			if gv, ok := inner["grammage"].(float64); ok && gv > 0 {
				gramVal = gv
				grammage = fmt.Sprintf("%.3g", gv)
			}
			unit, _ = inner["unitOfMeasure"].(string)
			// Default unit to "kg" when grammage is present but unit is null.
			if unit == "" && gramVal > 0 {
				unit = "kg"
			}
			if priceVal > 0 && gramVal > 0 {
				u := strings.ToLower(unit)
				if u == "kilogram" || u == "kg" || u == "litre" {
					pricePerKg = fmt.Sprintf("%.2f", priceVal/gramVal)
				}
			}
			if av, ok := inner["isAvailable"].(bool); ok {
				available = fmt.Sprintf("%v", av)
			}
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			id, name, brand, price, grammage, unit, pricePerKg, available)
	}
	return w.Flush()
}

// printSlotsTable renders a slice of day/slots maps as a per-day tabwriter table.
func printSlotsTable(days []map[string]any) error {
	for _, day := range days {
		date := cellValue(day["date"])
		fmt.Println(date)
		slots, _ := day["slots"].([]map[string]any)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "from\tto\tmethod\twarehouse")
		for _, slot := range slots {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				hhmm(slot["startsAt"]),
				hhmm(slot["endsAt"]),
				cellValue(slot["deliveryMethod"]),
				cellValue(slot["warehouse"]),
			)
		}
		_ = w.Flush()
		fmt.Println()
	}
	return nil
}

// printCartSummary renders a Frisco cart payload as a tabwriter table.
// Items can be sorted and limited via opts.
func printCartSummary(v any, opts cartShowOpts) error {
	root, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected cart payload")
	}
	rawProducts, ok := root["products"].([]any)
	if !ok {
		return fmt.Errorf("missing products list")
	}

	// Build normalised items.
	items := make([]cartItem, 0, len(rawProducts))
	for _, raw := range rawProducts {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		pid := asString(entry["productId"])
		qty := asInt(entry["quantity"])

		var product map[string]any
		if p, ok := entry["product"].(map[string]any); ok {
			product = p
		}

		name := shared.ProductNameFromMap(product)
		if name == "" {
			name = pid
		}

		// grammage and unit from product
		grammage := ""
		unit := ""
		if product != nil {
			if g, ok := product["grammage"].(string); ok {
				grammage = strings.TrimSpace(g)
			}
			if u, ok := product["unitOfMeasure"].(string); ok {
				unit = strings.TrimSpace(u)
			}
		}

		// unit price: prefer item-level price, fall back to product price
		unitPriceStr := shared.MoneyString(entry["price"])
		if unitPriceStr == "" && product != nil {
			unitPriceStr = shared.MoneyString(product["price"])
		}
		unitPrice, _ := parseMoneyFloat(unitPriceStr)

		// total
		totalStr := shared.MoneyString(entry["total"])
		var lineTotal float64
		if totalStr != "" {
			lineTotal, _ = parseMoneyFloat(totalStr)
		} else if unitPrice > 0 && qty > 0 {
			lineTotal = unitPrice * float64(qty)
		}

		// price per kg/litre — calculated when unit is weight/volume based
		pricePerKg := 0.0
		unitNorm := strings.ToLower(unit)
		if unitNorm == "kilogram" || unitNorm == "litre" {
			if g, ok := parseGrammageKg(grammage); ok && g > 0 && unitPrice > 0 {
				pricePerKg = unitPrice / g
			}
		}
		// also try pricePerUnit from product map as a direct fallback
		if pricePerKg == 0 && product != nil {
			if pkObj, ok := product["pricePerUnit"].(map[string]any); ok {
				if pv, ok := pkObj["price"].(float64); ok {
					pricePerKg = pv
				}
			}
			if pricePerKg == 0 {
				if pv, ok := product["pricePerKg"].(float64); ok {
					pricePerKg = pv
				}
			}
		}

		items = append(items, cartItem{
			pid:        pid,
			name:       name,
			qty:        qty,
			grammage:   grammage,
			unit:       unit,
			unitPrice:  unitPrice,
			total:      lineTotal,
			pricePerKg: pricePerKg,
		})
	}

	// Sort.
	switch strings.ToLower(strings.TrimSpace(opts.sortBy)) {
	case "total":
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].total > items[j].total
		})
	case "price-per-kg":
		sort.SliceStable(items, func(i, j int) bool {
			a, b := items[i].pricePerKg, items[j].pricePerKg
			if a == 0 {
				return false
			}
			if b == 0 {
				return true
			}
			return a > b
		})
	case "name":
		sort.SliceStable(items, func(i, j int) bool {
			return strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
		})
	}

	// Limit.
	if opts.top > 0 && opts.top < len(items) {
		items = items[:opts.top]
	}

	// Print table.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tPRODUCT ID\tQTY\tGRAMMAGE\tUNIT\tUNIT PRICE\tPRICE/KG\tTOTAL")
	grandTotal := 0.0
	for _, it := range items {
		ppkgStr := "-"
		if it.pricePerKg > 0 {
			ppkgStr = fmt.Sprintf("%.2f", it.pricePerKg)
		}
		unitPriceStr := "-"
		if it.unitPrice > 0 {
			unitPriceStr = fmt.Sprintf("%.2f", it.unitPrice)
		}
		totalStr := "-"
		if it.total > 0 {
			totalStr = fmt.Sprintf("%.2f", it.total)
			grandTotal += it.total
		}
		_, _ = fmt.Fprintf(
			w,
			"%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			shared.TruncateText(it.name, 48),
			it.pid,
			it.qty,
			fallbackDash(it.grammage),
			fallbackDash(it.unit),
			unitPriceStr,
			ppkgStr,
			totalStr,
		)
	}
	_ = w.Flush()

	if totalByStore, ok := root["total"].(map[string]any); ok {
		if val := shared.MoneyString(totalByStore["_total"]); val != "" {
			_, _ = fmt.Printf("\nCart total: %s\n", val)
		}
	} else if grandTotal > 0 {
		_, _ = fmt.Printf("\nCart total: %.2f\n", grandTotal)
	}
	return nil
}

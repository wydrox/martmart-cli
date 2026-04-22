package upmenu

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func ParseMenuHTML(src string) (*Menu, error) {
	root, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("parse menu html: %w", err)
	}
	menu := &Menu{}
	categories := findAll(root, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "theme-product-group")
	})
	for _, catNode := range categories {
		category := MenuCategory{
			ID: strings.TrimPrefix(attr(catNode, "id"), "pg-"),
			Name: strings.TrimSpace(textContent(firstDescendant(catNode, func(n *html.Node) bool {
				return hasClass(n, "theme-product-group-name") || hasClass(n, "theme-category")
			}))),
			Description: strings.TrimSpace(textContent(firstDescendant(catNode, func(n *html.Node) bool { return hasClass(n, "theme-category-description") }))),
		}
		productsList := nextElementSibling(catNode)
		if productsList == nil || !hasClass(productsList, "theme-products-list") {
			continue
		}
		productNodes := findAll(productsList, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "theme-product")
		})
		for _, productNode := range productNodes {
			product := parseProductNode(productNode, category)
			if product.Name == "" || product.ProductPriceID == "" {
				continue
			}
			category.Products = append(category.Products, product)
			menu.Products = append(menu.Products, product)
		}
		if category.Name != "" || len(category.Products) > 0 {
			menu.Categories = append(menu.Categories, category)
		}
	}
	return menu, nil
}

func parseProductNode(node *html.Node, category MenuCategory) MenuProduct {
	product := MenuProduct{
		ID:           strings.TrimPrefix(attr(node, "id"), "p-"),
		CategoryID:   category.ID,
		CategoryName: category.Name,
		Name:         strings.TrimSpace(textContent(firstDescendant(node, func(n *html.Node) bool { return hasClass(n, "theme-product-name") }))),
		Description: normalizeWhitespace(textContent(firstDescendant(node, func(n *html.Node) bool {
			return hasClass(n, "theme-product-description") || hasClass(n, "theme-product-desc")
		}))),
	}
	if image := firstDescendant(node, func(n *html.Node) bool {
		return hasClass(n, "theme-product-image") || hasClass(n, "_buying-flow-image")
	}); image != nil {
		product.ImageURL = firstNonEmpty(attr(image, "data-src"), extractBackgroundURL(attr(image, "style")))
		if product.ImageURL == "" {
			if child := firstDescendant(image, func(n *html.Node) bool { return attr(n, "data-src") != "" }); child != nil {
				product.ImageURL = attr(child, "data-src")
			}
		}
	}
	for _, addNode := range findAll(node, func(n *html.Node) bool {
		return n.Type == html.ElementNode && attr(n, "data-id") != "" && hasClass(n, "_add-to-cart")
	}) {
		product.ProductPriceID = attr(addNode, "data-id")
		break
	}
	if priceNode := firstDescendant(node, func(n *html.Node) bool { return hasClass(n, "theme-price-value") }); priceNode != nil {
		if price, ok := parsePrice(textContent(priceNode)); ok {
			product.BasePrice = &price
		}
	}
	return product
}

func textContent(node *html.Node) string {
	if node == nil {
		return ""
	}
	var buf bytes.Buffer
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			_, _ = io.WriteString(&buf, n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)
	return buf.String()
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\u00a0", " ")), " ")
}

func extractBackgroundURL(style string) string {
	style = strings.TrimSpace(style)
	if style == "" {
		return ""
	}
	prefix := "background-image: url("
	idx := strings.Index(style, prefix)
	if idx < 0 {
		return ""
	}
	rest := style[idx+len(prefix):]
	end := strings.Index(rest, ")")
	if end < 0 {
		return ""
	}
	return strings.Trim(rest[:end], `"' `)
}

func parsePrice(s string) (float64, bool) {
	s = normalizeWhitespace(strings.TrimSuffix(strings.TrimSpace(s), "PLN"))
	s = strings.ReplaceAll(s, ",", ".")
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func findAll(root *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if match(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func firstDescendant(root *html.Node, match func(*html.Node) bool) *html.Node {
	for _, n := range findAll(root, match) {
		return n
	}
	return nil
}

func attr(node *html.Node, key string) string {
	if node == nil {
		return ""
	}
	for _, a := range node.Attr {
		if strings.EqualFold(a.Key, key) {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func nextElementSibling(node *html.Node) *html.Node {
	if node == nil {
		return nil
	}
	for sib := node.NextSibling; sib != nil; sib = sib.NextSibling {
		if sib.Type == html.ElementNode {
			return sib
		}
	}
	return nil
}

func hasClass(node *html.Node, class string) bool {
	for _, part := range strings.Fields(attr(node, "class")) {
		if part == class {
			return true
		}
	}
	return false
}

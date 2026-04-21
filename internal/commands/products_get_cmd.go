package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/delio"
	"github.com/wydrox/martmart-cli/internal/httpclient"
	"github.com/wydrox/martmart-cli/internal/session"
)

func newProductsGetCmd() *cobra.Command {
	var (
		userID    string
		productID string
		slug      string
		lat       float64
		long      float64
		rawOutput bool
	)
	c := &cobra.Command{
		Use:   "get",
		Short: "Fetch a single product by ID/SKU or slug.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(productID) == "" && strings.TrimSpace(slug) == "" {
				return fmt.Errorf("one of --product-id or --slug is required")
			}
			provider, s, err := loadSessionForRequest(cmd)
			if err != nil {
				return err
			}
			if provider == session.ProviderDelio {
				coords, err := delioCoordsFromFlags(cmd, lat, long)
				if err != nil {
					return err
				}
				result, err := delio.GetProduct(s, slug, productID, coords)
				if err != nil {
					return err
				}
				return printJSON(result)
			}
			uid, err := session.RequireUserID(s, userID)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/app/commerce/api/v1/users/%s/offer/products", uid)
			result, err := httpclient.RequestJSON(s, "GET", path, httpclient.RequestOpts{Query: []string{"productIds=" + strings.TrimSpace(productID)}})
			if err != nil {
				return err
			}
			if rawOutput || strings.EqualFold(outputFormat, "json") {
				return printJSON(result)
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&productID, "product-id", "", "Frisco product ID or Delio SKU.")
	c.Flags().StringVar(&slug, "slug", "", "Delio product slug.")
	c.Flags().StringVar(&userID, "user-id", "", "Frisco user ID override.")
	c.Flags().Float64Var(&lat, "lat", 0, "Delio latitude override.")
	c.Flags().Float64Var(&long, "long", 0, "Delio longitude override.")
	c.Flags().BoolVar(&rawOutput, "raw", false, "Show full API response")
	return c
}

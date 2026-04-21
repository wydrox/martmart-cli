package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/session"
)

func newRestaurantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restaurant",
		Short: "Public restaurant storefront commands for UpMenu/Dobra Buła.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, ok, err := explicitProvider(cmd)
			if err != nil {
				return err
			}
			if ok && provider != session.ProviderUpMenu {
				return unsupportedProviderError(cmd, provider, session.ProviderUpMenu)
			}
			return cmd.Help()
		},
	}
	cmd.AddCommand(newRestaurantInfoCmd(), newRestaurantMenuCmd())
	return cmd
}

func newRestaurantInfoCmd() *cobra.Command {
	var restaurantURL string
	var language string
	c := &cobra.Command{
		Use:   "info",
		Short: "Fetch public UpMenu restaurant information.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, ok, err := explicitProvider(cmd)
			if err != nil {
				return err
			}
			if ok && provider != session.ProviderUpMenu {
				return unsupportedProviderError(cmd, provider, session.ProviderUpMenu)
			}
			client, err := newUpMenuCLIClient(nil, restaurantURL, language)
			if err != nil {
				return err
			}
			result, err := client.RestaurantInfo(context.Background())
			if err != nil {
				return err
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&restaurantURL, "restaurant-url", "", "Absolute UpMenu restaurant page URL (defaults to the Dobra Buła MVP storefront).")
	c.Flags().StringVar(&language, "language", "", "Optional UpMenu storefront language header (default: pl).")
	return c
}

func newRestaurantMenuCmd() *cobra.Command {
	var restaurantURL string
	var language string
	c := &cobra.Command{
		Use:   "menu",
		Short: "Fetch public UpMenu restaurant menu.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, ok, err := explicitProvider(cmd)
			if err != nil {
				return err
			}
			if ok && provider != session.ProviderUpMenu {
				return unsupportedProviderError(cmd, provider, session.ProviderUpMenu)
			}
			client, err := newUpMenuCLIClient(nil, restaurantURL, language)
			if err != nil {
				return err
			}
			result, err := client.Menu(context.Background())
			if err != nil {
				return err
			}
			if strings.EqualFold(outputFormat, "json") {
				return printJSON(result)
			}
			if m, ok := result.(map[string]any); ok {
				if body, ok := m["body"].(string); ok && strings.TrimSpace(body) != "" {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), body)
					return nil
				}
			}
			return printJSON(result)
		},
	}
	c.Flags().StringVar(&restaurantURL, "restaurant-url", "", "Absolute UpMenu restaurant page URL (defaults to the Dobra Buła MVP storefront).")
	c.Flags().StringVar(&language, "language", "", "Optional UpMenu storefront language header (default: pl).")
	return c
}

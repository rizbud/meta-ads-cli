package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rizbud/meta-ads-cli/api"
	"github.com/rizbud/meta-ads-cli/campaign"
	"github.com/rizbud/meta-ads-cli/money"
)

func newAccountCommand(r *Runner) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Show the configured Meta ad account summary.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			client, err := r.NewClient(false)
			if err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}
			data, err := client.GetAdAccount()
			if err != nil {
				fmt.Fprintf(out, "API Error: %s\n", err)
				return errNonZero
			}
			if jsonOutput {
				echoJSON(out, data)
				return nil
			}
			fmt.Fprintln(out, "Meta ad account")
			currency := strings.ToUpper(data["currency"])
			for _, key := range []string{"id", "name", "account_status", "currency", "timezone_name", "amount_spent", "balance"} {
				val, ok := data[key]
				if !ok {
					val = "N/A"
				} else if key == "amount_spent" || key == "balance" {
					if n, err := strconv.ParseInt(val, 10, 64); err == nil {
						val = money.Format(n, currency)
					}
				}
				fmt.Fprintf(out, "  %s: %s\n", key, val)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json-output", false, "Print raw JSON.")
	return cmd
}

func newListCommand(name string, r *Runner) *cobra.Command {
	var limit int
	var jsonOutput bool

	short := map[string]string{
		"campaigns": "List campaigns in the configured ad account.",
		"adsets":    "List ad sets in the configured ad account.",
		"ads":       "List ads in the configured ad account.",
	}[name]

	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			client, err := r.NewClient(false)
			if err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}

			var rows []map[string]string
			var kind string
			switch name {
			case "campaigns":
				kind = "campaigns"
				rows, err = client.ListCampaigns(boundedLimit(limit))
			case "adsets":
				kind = "ad_sets"
				rows, err = client.ListAdSets(boundedLimit(limit))
			case "ads":
				kind = "ads"
				rows, err = client.ListAds(boundedLimit(limit))
			}
			if err != nil {
				fmt.Fprintf(out, "API Error: %s\n", err)
				return errNonZero
			}
			if jsonOutput {
				echoJSON(out, map[string]any{kind: rows})
				return nil
			}
			for _, row := range rows {
				switch name {
				case "adsets":
					budget := row["daily_budget"]
					if budget == "" {
						budget = "N/A"
					}
					fmt.Fprintf(out, "%s  %s  %s  %s\n", row["id"], row["status"], budget, row["name"])
				default:
					fmt.Fprintf(out, "%s  %s  %s\n", row["id"], row["status"], row["name"])
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Number of rows to return, max 100.")
	cmd.Flags().BoolVar(&jsonOutput, "json-output", false, "Print raw JSON.")
	return cmd
}

func newInsightsCommand(r *Runner) *cobra.Command {
	var level string
	var datePreset string
	var limit int
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "insights [object-id]",
		Short: "Show account, campaign, ad set, or ad insights.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			client, err := r.NewClient(false)
			if err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}
			target := ""
			if len(args) > 0 {
				target = args[0]
			} else {
				target = client.ActID()
			}

			rows, err := client.GetInsights(api.GetInsightsParams{
				ObjectID:   target,
				Level:      level,
				DatePreset: datePreset,
				Limit:      boundedLimit(limit),
			})
			if err != nil {
				fmt.Fprintf(out, "API Error: %s\n", err)
				return errNonZero
			}

			var levelOut any
			if level != "" {
				levelOut = level
			}
			payload := map[string]any{
				"object_id":   target,
				"level":       levelOut,
				"date_preset": datePreset,
				"insights":    rows,
			}
			if jsonOutput {
				echoJSON(out, payload)
				return nil
			}
			for _, row := range rows {
				name := row["campaign_name"]
				if name == "" {
					name = target
				}
				spend := row["spend"]
				if spend == "" {
					spend = "0"
				}
				clicks := row["clicks"]
				if clicks == "" {
					clicks = "0"
				}
				impressions := row["impressions"]
				if impressions == "" {
					impressions = "0"
				}
				fmt.Fprintf(out, "%s  spend=%s  clicks=%s  impressions=%s\n", name, spend, clicks, impressions)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&level, "level", "", "Optional account breakdown: campaign, adset, or ad.")
	cmd.Flags().StringVar(&datePreset, "date-preset", "last_7d", "Meta date preset.")
	cmd.Flags().IntVar(&limit, "limit", 25, "Number of rows to return, max 100.")
	cmd.Flags().BoolVar(&jsonOutput, "json-output", false, "Print raw JSON.")
	return cmd
}

func newStatusCommand(r *Runner) *cobra.Command {
	return &cobra.Command{
		Use:   "status <campaign-id>",
		Short: "Show the status of a campaign and its ads.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			client, err := r.NewClient(false)
			if err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}
			if err := campaign.PrintCampaignStatus(out, client, args[0], currencyFor(client, false)); err != nil {
				fmt.Fprintf(out, "API Error: %s\n", err)
				return errNonZero
			}
			return nil
		},
	}
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rizbud/meta-ads-cli/api"
	"github.com/rizbud/meta-ads-cli/campaign"
	"github.com/rizbud/meta-ads-cli/config"
)

func newCreateCommand(r *Runner) *cobra.Command {
	var configPath string
	var dryRun bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a full campaign from a YAML config file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				fmt.Fprintf(out, "Config error:\n%s\n", err)
				return errNonZero
			}
			if err := config.ValidateConfig(cfg); err != nil {
				fmt.Fprintf(out, "Config error:\n%s\n", err)
				return errNonZero
			}

			campaignName := cfg.Campaign.Name
			status := cfg.Campaign.Status
			budget := float64(cfg.AdSet.DailyBudget) / 100
			numAds := len(cfg.Ads)

			fmt.Fprintln(out, "==================================================")
			fmt.Fprintln(out, "meta-ads create")
			fmt.Fprintln(out, "==================================================")
			fmt.Fprintf(out, "Campaign:  %s\n", campaignName)
			fmt.Fprintf(out, "Budget:    $%.2f/day\n", budget)
			fmt.Fprintf(out, "Ads:       %d\n", numAds)
			fmt.Fprintf(out, "Status:    %s\n", status)
			mode := "DRY RUN"
			if !dryRun {
				mode = "LIVE"
			}
			fmt.Fprintf(out, "Mode:      %s\n", mode)

			if err := checkDailyBudgetLimit(cfg.AdSet.DailyBudget, "create"); err != nil {
				fmt.Fprintf(out, "Budget error: %s\n", err)
				return errNonZero
			}

			if !dryRun && !yes {
				fmt.Fprintln(out)
				if !r.Confirm("This will create real campaigns. Continue?") {
					fmt.Fprintln(out, "Aborted.")
					return nil
				}
			}
			confirmed := !dryRun
			if confirmed {
				if err := checkAuditLogWritable(); err != nil {
					fmt.Fprintf(out, "%s\n", err)
					return errNonZero
				}
			}

			client, err := r.NewClient(dryRun)
			if err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}
			if dryRun {
				client.SetDryRunOut(out)
			}

			request := map[string]any{
				"config_path":        configPath,
				"campaign_name":      campaignName,
				"daily_budget_cents": cfg.AdSet.DailyBudget,
				"ad_count":           numAds,
				"dry_run":            dryRun,
				"confirmed":          confirmed,
			}

			res, err := campaign.CreateFullCampaign(client, cfg)
			if err != nil {
				partial := map[string]any{
					"campaign_id": res.CampaignID,
					"ad_set_id":   res.AdSetID,
					"creatives":   res.Creatives,
					"ads":         res.Ads,
				}
				metaCode := 0
				if apiErr, ok := err.(*api.APIError); ok {
					metaCode = apiErr.MetaCode
				}
				failure := map[string]any{
					"success":        false,
					"dry_run":        dryRun,
					"confirmed":      confirmed,
					"partial_result": partial,
					"error":          err.Error(),
					"error_code":     metaCode,
				}
				if warning := writeAudit("create", request, failure); warning != "" {
					fmt.Fprintf(out, "Warning: %s\n", warning)
				}
				fmt.Fprintf(out, "\nAPI Error: %s\n", err)
				if metaCode != 0 {
					fmt.Fprintf(out, "Error code: %d\n", metaCode)
				}
				if res.CampaignID != "" || res.AdSetID != "" {
					fmt.Fprintln(out, "\nPartial resources were created and left in place:")
					if res.CampaignID != "" {
						fmt.Fprintf(out, "  Campaign: %s\n", res.CampaignID)
					}
					if res.AdSetID != "" {
						fmt.Fprintf(out, "  Ad Set:   %s\n", res.AdSetID)
					}
					fmt.Fprintln(out, "\nClean up with:")
					if res.AdSetID != "" {
						fmt.Fprintf(out, "  meta-ads delete %s --yes\n", res.AdSetID)
					}
					fmt.Fprintf(out, "  meta-ads delete %s --yes\n", res.CampaignID)
				}
				return errNonZero
			}

			result := map[string]any{
				"campaign_id": res.CampaignID,
				"ad_set_id":   res.AdSetID,
				"creatives":   res.Creatives,
				"ads":         res.Ads,
			}
			if warning := writeAudit("create", request, result); warning != "" {
				fmt.Fprintf(out, "Warning: %s\n", warning)
			}

			fmt.Fprintln(out, "\n==================================================")
			fmt.Fprintln(out, "Done!")
			fmt.Fprintln(out, "==================================================")
			fmt.Fprintf(out, "Campaign:  %s (%s)\n", res.CampaignID, status)
			fmt.Fprintf(out, "Ad Set:    %s (%s)\n", res.AdSetID, status)
			fmt.Fprintf(out, "Creatives: %d\n", len(res.Creatives))
			fmt.Fprintf(out, "Ads:       %d\n", len(res.Ads))
			if !dryRun {
				fmt.Fprintln(out, "\nView in Ads Manager:")
				fmt.Fprintf(out, "  https://adsmanager.facebook.com/adsmanager/manage/campaigns?act=%s\n", client.AdAccountID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "campaign.yaml", "Path to campaign YAML config.")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be created without making API calls.")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt.")
	return cmd
}

func newValidateCommand() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a campaign YAML config without making API calls.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cfg, err := config.LoadConfig(configPath)
			if err == nil {
				err = config.ValidateConfig(cfg)
			}
			if err != nil {
				fmt.Fprintf(out, "Validation failed:\n%s\n", err)
				return errNonZero
			}
			budget := float64(cfg.AdSet.DailyBudget) / 100
			fmt.Fprintln(out, "Config is valid.")
			fmt.Fprintf(out, "  Campaign: %s\n", cfg.Campaign.Name)
			fmt.Fprintf(out, "  Budget:   $%.2f/day\n", budget)
			fmt.Fprintf(out, "  Ads:      %d\n", len(cfg.Ads))
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "campaign.yaml", "Path to campaign YAML config.")
	return cmd
}

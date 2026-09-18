package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rizbud/meta-ads-cli/api"
	"github.com/rizbud/meta-ads-cli/money"
)

func apiUploadParams(filePath, base string, data []byte) api.UploadImageParams {
	return api.UploadImageParams{
		FilePath:    filePath,
		FileName:    base,
		ContentType: "image/png",
		Data:        data,
	}
}

func apiErrorCode(err error) int {
	if apiErr, ok := err.(*api.APIError); ok {
		return apiErr.MetaCode
	}
	return 0
}

func newPauseCommand(r *Runner) *cobra.Command {
	return &cobra.Command{
		Use:   "pause <campaign-id>",
		Short: "Pause a campaign.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			campaignID := args[0]
			if err := checkAuditLogWritable(); err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}
			client, err := r.NewClient(false)
			if err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}
			if err := client.UpdateStatus(campaignID, "PAUSED"); err != nil {
				fmt.Fprintf(out, "API Error: %s\n", err)
				return errNonZero
			}
			if warning := writeAudit("pause", map[string]any{"campaign_id": campaignID}, map[string]any{"success": true, "campaign_id": campaignID}); warning != "" {
				fmt.Fprintf(out, "Warning: %s\n", warning)
			}
			fmt.Fprintf(out, "Campaign %s paused.\n", campaignID)
			return nil
		},
	}
}

func newActivateCommand(r *Runner) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "activate <campaign-id>",
		Short: "Activate a campaign. This will start spending your budget.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			campaignID := args[0]
			if !yes {
				if !r.Confirm("This will start spending your ad budget. Continue?") {
					fmt.Fprintln(out, "Aborted.")
					return nil
				}
			}
			confirmed := true
			if err := checkAuditLogWritable(); err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}
			client, err := r.NewClient(false)
			if err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}
			if err := client.UpdateStatus(campaignID, "ACTIVE"); err != nil {
				fmt.Fprintf(out, "API Error: %s\n", err)
				return errNonZero
			}
			if warning := writeAudit("activate", map[string]any{"campaign_id": campaignID, "confirmed": confirmed}, map[string]any{"success": true, "campaign_id": campaignID, "confirmed": confirmed}); warning != "" {
				fmt.Fprintf(out, "Warning: %s\n", warning)
			}
			fmt.Fprintf(out, "Campaign %s activated.\n", campaignID)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt.")
	return cmd
}

func newDeleteCommand(r *Runner) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <campaign-id>",
		Short: "Delete a campaign. This cannot be undone.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			campaignID := args[0]
			if !yes {
				if !r.Confirm("This will permanently delete the campaign. Continue?") {
					fmt.Fprintln(out, "Aborted.")
					return nil
				}
			}
			confirmed := true
			if err := checkAuditLogWritable(); err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}
			client, err := r.NewClient(false)
			if err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}
			if err := client.DeleteCampaign(campaignID); err != nil {
				fmt.Fprintf(out, "API Error: %s\n", err)
				return errNonZero
			}
			if warning := writeAudit("delete", map[string]any{"campaign_id": campaignID, "confirmed": confirmed}, map[string]any{"success": true, "campaign_id": campaignID, "confirmed": confirmed}); warning != "" {
				fmt.Fprintf(out, "Warning: %s\n", warning)
			}
			fmt.Fprintf(out, "Campaign %s deleted.\n", campaignID)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt.")
	return cmd
}

func newBudgetCommand(r *Runner) *cobra.Command {
	var live bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "budget <object-id> <daily-budget-cents>",
		Short: "Update campaign or ad set daily budget. Defaults to dry run.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			objectID := args[0]
			var dailyBudgetCents int
			if _, err := fmt.Sscanf(args[1], "%d", &dailyBudgetCents); err != nil {
				fmt.Fprintf(out, "daily_budget_cents must be an integer.\n")
				return errNonZero
			}

			dryRun := !live
			confirmed := live

			if err := checkDailyBudgetLimit(dailyBudgetCents, "budget"); err != nil {
				fmt.Fprintf(out, "Budget error: %s\n", err)
				return errNonZero
			}
			client, err := r.NewClient(dryRun)
			if err != nil {
				fmt.Fprintf(out, "%s\n", err)
				return errNonZero
			}
			currency := currencyFor(client, dryRun)
			if !dryRun && !yes {
				if !r.Confirm(fmt.Sprintf("Set %s to %s/day?", objectID, money.Format(int64(dailyBudgetCents), currency))) {
					fmt.Fprintln(out, "Aborted.")
					return nil
				}
			}
			if confirmed {
				if err := checkAuditLogWritable(); err != nil {
					fmt.Fprintf(out, "%s\n", err)
					return errNonZero
				}
			}
			if err := client.UpdateDailyBudget(objectID, dailyBudgetCents); err != nil {
				fmt.Fprintf(out, "API Error: %s\n", err)
				return errNonZero
			}
			result := map[string]any{
				"success":            true,
				"object_id":          objectID,
				"daily_budget_cents": dailyBudgetCents,
				"dry_run":            dryRun,
				"confirmed":          confirmed,
			}
			if warning := writeAudit("budget", map[string]any{
				"object_id":          objectID,
				"daily_budget_cents": dailyBudgetCents,
				"dry_run":            dryRun,
				"confirmed":          confirmed,
			}, result); warning != "" {
				fmt.Fprintf(out, "Warning: %s\n", warning)
			}
			fmt.Fprintln(out, "Budget update accepted.")
			if dryRun {
				fmt.Fprintln(out, "Dry run only. No live Meta change was made.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&live, "live", false, "Make the live Meta API change. Defaults to dry run.")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt.")
	return cmd
}

func newUploadImageCommand(r *Runner) *cobra.Command {
	var live bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "upload-image <image-path>",
		Short: "Upload an image and return the Meta image hash. Defaults to dry run.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			imagePath := args[0]
			data, err := os.ReadFile(imagePath)
			if err != nil {
				fmt.Fprintf(out, "Image not found: %s\n", imagePath)
				return errNonZero
			}

			dryRun := !live
			confirmed := live
			base := filepath.Base(imagePath)

			if !dryRun && !yes {
				if !r.Confirm(fmt.Sprintf("Upload %s to Meta?", base)) {
					fmt.Fprintln(out, "Aborted.")
					return nil
				}
			}
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
			imageHash, err := client.UploadImage(apiUploadParams(imagePath, base, data))
			if err != nil {
				fmt.Fprintf(out, "API Error: %s\n", err)
				return errNonZero
			}
			result := map[string]any{"success": true, "image_hash": imageHash, "dry_run": dryRun, "confirmed": confirmed}
			if warning := writeAudit("upload-image", map[string]any{"image_path": imagePath, "dry_run": dryRun, "confirmed": confirmed}, result); warning != "" {
				fmt.Fprintf(out, "Warning: %s\n", warning)
			}
			fmt.Fprintf(out, "Image hash: %s\n", imageHash)
			if dryRun {
				fmt.Fprintln(out, "Dry run only. No live Meta upload was made.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&live, "live", false, "Upload to Meta. Defaults to dry run.")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt.")
	return cmd
}

func newBulkStatusCommand(r *Runner) *cobra.Command {
	var live bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "bulk-status <status> <campaign-id...>",
		Short: "Bulk pause, activate, or delete campaigns. Defaults to dry run.",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			status := strings.ToUpper(args[0])
			campaignIDs := args[1:]

			if status != "PAUSED" && status != "ACTIVE" && status != "DELETED" {
				fmt.Fprintf(out, "status must be PAUSED, ACTIVE, or DELETED.\n")
				return errNonZero
			}

			dryRun := !live
			confirmed := live

			if !dryRun && !yes {
				if !r.Confirm(fmt.Sprintf("Set %d campaigns to %s?", len(campaignIDs), status)) {
					fmt.Fprintln(out, "Aborted.")
					return nil
				}
			}
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
			changed := []map[string]string{}
			current := ""
			for _, campaignID := range campaignIDs {
				current = campaignID
				if err := client.UpdateStatus(campaignID, status); err != nil {
					if warning := writeAudit("bulk-status", map[string]any{
						"campaign_ids": campaignIDs,
						"status":       status,
						"dry_run":      dryRun,
						"confirmed":    confirmed,
					}, map[string]any{
						"success":            false,
						"dry_run":            dryRun,
						"confirmed":          confirmed,
						"changed":            changed,
						"failed_campaign_id": current,
						"error":              err.Error(),
						"error_code":         apiErrorCode(err),
					}); warning != "" {
						fmt.Fprintf(out, "Warning: %s\n", warning)
					}
					fmt.Fprintf(out, "API Error: %s\n", err)
					return errNonZero
				}
				changed = append(changed, map[string]string{"campaign_id": campaignID, "status": status})
			}
			result := map[string]any{"success": true, "dry_run": dryRun, "confirmed": confirmed, "changed": changed}
			if warning := writeAudit("bulk-status", map[string]any{
				"campaign_ids": campaignIDs,
				"status":       status,
				"dry_run":      dryRun,
				"confirmed":    confirmed,
			}, result); warning != "" {
				fmt.Fprintf(out, "Warning: %s\n", warning)
			}
			fmt.Fprintf(out, "%d campaign updates accepted.\n", len(changed))
			if dryRun {
				fmt.Fprintln(out, "Dry run only. No live Meta change was made.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&live, "live", false, "Make the live Meta API changes. Defaults to dry run.")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt.")
	return cmd
}

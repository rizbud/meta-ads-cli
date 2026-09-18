package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rizbud/meta-ads-cli/api"
	"github.com/rizbud/meta-ads-cli/audit"
)

// errNonZero signals a command that should exit with a non-zero status. The
// root command suppresses cobra's automatic error printing because commands
// write their own messages to their output writer.
var errNonZero = errors.New("exit 1")

// Version matches the upstream release.
const Version = "0.2.3"

// Runner holds the injectable seams the CLI needs. A nil NewClient builds the
// client from environment variables.
type Runner struct {
	Confirm   func(prompt string) bool
	NewClient func(dryRun bool) (*api.Client, error)
}

func defaultConfirm(prompt string) bool {
	fmt.Fprintf(os.Stdout, "%s [y/N]: ", prompt)
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func newClientFromEnv(dryRun bool) (*api.Client, error) {
	accessToken := os.Getenv("META_ACCESS_TOKEN")
	adAccountID := os.Getenv("META_AD_ACCOUNT_ID")
	pageID := os.Getenv("META_PAGE_ID")
	apiVersion := os.Getenv("META_API_VERSION")

	var missing []string
	if accessToken == "" {
		missing = append(missing, "META_ACCESS_TOKEN")
	}
	if adAccountID == "" {
		missing = append(missing, "META_AD_ACCOUNT_ID")
	}
	if pageID == "" {
		missing = append(missing, "META_PAGE_ID")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("Missing required environment variables:\n  %s\n\nSet them in .env or export them in your shell.\nSee: https://github.com/rizbud/meta-ads-cli#configuration", strings.Join(missing, "\n  "))
	}

	return api.New(api.Config{
		AccessToken: accessToken,
		AdAccountID: adAccountID,
		PageID:      pageID,
		APIVersion:  apiVersion,
		DryRun:      dryRun,
	}), nil
}

// NewRoot builds the meta-ads root command. A nil runner (or nil fields) uses
// real environment variables, stdin confirmation, and live Meta API clients.
func NewRoot(r *Runner) *cobra.Command {
	if r == nil {
		r = &Runner{}
	}
	if r.Confirm == nil {
		r.Confirm = defaultConfirm
	}
	if r.NewClient == nil {
		r.NewClient = newClientFromEnv
	}
	runner := r

	root := &cobra.Command{
		Use:           "meta-ads",
		Short:         "Create and manage Meta ad campaigns from your terminal.",
		Version:       Version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.AddCommand(
		newSetupCommand(),
		newCreateCommand(runner),
		newValidateCommand(),
		newAccountCommand(runner),
		newListCommand("campaigns", runner),
		newListCommand("adsets", runner),
		newListCommand("ads", runner),
		newInsightsCommand(runner),
		newStatusCommand(runner),
		newBudgetCommand(runner),
		newUploadImageCommand(runner),
		newBulkStatusCommand(runner),
		newPauseCommand(runner),
		newActivateCommand(runner),
		newDeleteCommand(runner),
	)

	return root
}

func newSetupCommand() *cobra.Command {
	var client string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Print a one-step setup snippet for common AI clients.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			client = strings.ToLower(client)
			if client == "" {
				client = "claude"
			}
			config := map[string]any{
				"mcpServers": map[string]any{
					"meta-ads": map[string]any{
						"command": "uvx",
						"args":    []string{"meta-ads-manager-mcp"},
						"env": map[string]string{
							"META_ACCESS_TOKEN":               "your-token-here",
							"META_AD_ACCOUNT_ID":              "your-account-id",
							"META_PAGE_ID":                    "your-page-id",
							"META_ADS_MAX_DAILY_BUDGET_CENTS": "5000",
						},
					},
				},
			}
			if client == "chatgpt" {
				fmt.Fprintln(out, "ChatGPT setup needs a hosted MCP endpoint. Use the hosted roadmap until remote MCP is shipped.")
				fmt.Fprintln(out, "For now, use Claude, Cursor, or Codex with the local config below.")
			} else {
				fmt.Fprintf(out, "Add this block to your %s MCP config:\n", client)
			}
			echoJSON(out, config)
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "client", "claude", "Client setup snippet to print: claude, cursor, codex, or chatgpt.")
	return cmd
}

func echoJSON(w io.Writer, data any) {
	encoded, _ := json.MarshalIndent(data, "", "  ")
	fmt.Fprintln(w, string(encoded))
}

func boundedLimit(limit int) int {
	if limit < 1 {
		return 1
	}
	if limit > 100 {
		return 100
	}
	return limit
}

// checkDailyBudgetLimit enforces the META_ADS_MAX_DAILY_BUDGET_CENTS guardrail.
func checkDailyBudgetLimit(cents int, action string) error {
	if cents <= 0 {
		return fmt.Errorf("%s daily_budget_cents must be positive.", action)
	}
	raw := os.Getenv("META_ADS_MAX_DAILY_BUDGET_CENTS")
	if raw == "" {
		return nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("META_ADS_MAX_DAILY_BUDGET_CENTS must be an integer.")
	}
	if cents > limit {
		return fmt.Errorf("%s budget %d exceeds META_ADS_MAX_DAILY_BUDGET_CENTS=%d.", action, cents, limit)
	}
	return nil
}

func checkAuditLogWritable() error {
	path := audit.DefaultPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("Audit log is not writable at %s: %v", path, err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("Audit log is not writable at %s: %v", path, err)
	}
	f.Close()
	return nil
}

// writeAudit appends an audit event and returns a warning string on failure.
func writeAudit(action string, request, result any) string {
	path := audit.DefaultPath()
	if err := audit.Write(path, action, os.Getenv("META_AD_ACCOUNT_ID"), request, result); err != nil {
		return fmt.Sprintf("Audit log write failed at %s: %v", path, err)
	}
	return ""
}

func fatal(err error) error { return errNonZero }

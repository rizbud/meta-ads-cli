package cmd

import (
	"os"
	"strings"

	"github.com/rizbud/meta-ads-cli/api"
)

// currencyFor resolves the currency used to format monetary amounts. Live
// commands read it from the ad account; offline commands (dry run, validate)
// make no API calls and fall back to the optional META_CURRENCY env var.
func currencyFor(client *api.Client, dryRun bool) string {
	if !dryRun && client != nil {
		if c := client.Currency(); c != "" {
			return c
		}
	}
	return strings.ToUpper(strings.TrimSpace(os.Getenv("META_CURRENCY")))
}

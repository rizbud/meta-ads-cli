package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Event is a single JSONL audit record. Credentials are never written.
type Event struct {
	Timestamp   string `json:"timestamp"`
	Action      string `json:"action"`
	AdAccountID string `json:"ad_account_id"`
	Request     any    `json:"request"`
	Result      any    `json:"result"`
}

// DefaultPath returns the audit log path from META_ADS_AUDIT_LOG_PATH or the
// default ~/.meta-ads-cli/audit.jsonl.
func DefaultPath() string {
	if configured := os.Getenv("META_ADS_AUDIT_LOG_PATH"); configured != "" {
		return expandUser(configured)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".meta-ads-cli/audit.jsonl"
	}
	return filepath.Join(home, ".meta-ads-cli", "audit.jsonl")
}

func expandUser(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// Write appends one JSON event to the audit log, creating parents as needed.
func Write(path, action, adAccountID string, request, result any) error {
	event := Event{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Action:      action,
		AdAccountID: adAccountID,
		Request:     request,
		Result:      result,
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, string(data)); err != nil {
		return err
	}
	return nil
}

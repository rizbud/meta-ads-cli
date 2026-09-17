package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPathUsesEnv(t *testing.T) {
	t.Setenv("META_ADS_AUDIT_LOG_PATH", "~/custom/audit.jsonl")
	got := DefaultPath()
	if got != filepath.Join(homeDir(t), "custom", "audit.jsonl") {
		t.Errorf("DefaultPath = %q", got)
	}
}

func TestDefaultPathFallback(t *testing.T) {
	t.Setenv("META_ADS_AUDIT_LOG_PATH", "")
	got := DefaultPath()
	want := filepath.Join(homeDir(t), ".meta-ads-cli", "audit.jsonl")
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

func homeDir(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	return home
}

func TestWriteAppendsJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	err := Write(path, "create", "act_123", map[string]any{"campaign_name": "C"}, map[string]any{"success": true})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := Write(path, "pause", "act_123", map[string]any{"campaign_id": "c1"}, map[string]any{"success": true}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2: %s", len(lines), raw)
	}
	first := lines[0]
	for _, want := range []string{`"action":"create"`, `"ad_account_id":"act_123"`, `"campaign_name":"C"`, `"success":true`} {
		if !strings.Contains(first, want) {
			t.Errorf("line missing %s: %s", want, first)
		}
	}
	if strings.Contains(first, "META_TOKEN") || strings.Contains(first, "password") {
		t.Errorf("audit event leaks secrets: %s", first)
	}
}

func TestWriteCreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "audit.jsonl")
	if err := Write(path, "activate", "", nil, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("audit file not created: %v", err)
	}
}

func TestWriteJSONSortedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := Write(path, "delete", "", map[string]any{"z": 1, "a": 2}, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	raw, _ := os.ReadFile(path)
	line := string(raw)
	if !strings.Contains(line, `{"a":2,"z":1}`) {
		t.Errorf("map keys not sorted: %s", line)
	}
}

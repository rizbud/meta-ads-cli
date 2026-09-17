package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rizbud/meta-ads-cli/api"
)

type captured struct {
	path   string
	query  url.Values
	method string
}

func newServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]captured) {
	t.Helper()
	var mu sync.Mutex
	var reqs []captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs = append(reqs, captured{path: r.URL.Path, query: r.URL.Query(), method: r.Method})
		mu.Unlock()
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs
}

func jsonResp(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func testRunner(t *testing.T, srv *httptest.Server) *Runner {
	t.Helper()
	return &Runner{
		Confirm: func(string) bool { return false },
		NewClient: func(dryRun bool) (*api.Client, error) {
			cfg := api.Config{
				AccessToken: "tok",
				AdAccountID: "123456",
				PageID:      "999",
				DryRun:      dryRun,
			}
			if srv != nil {
				cfg.BaseURL = srv.URL
			}
			return api.New(cfg), nil
		},
	}
}

func runCmd(t *testing.T, r *Runner, args ...string) (string, error) {
	t.Helper()
	var out strings.Builder
	cmd := NewRoot(r)
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func auditDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	t.Setenv("META_ADS_AUDIT_LOG_PATH", path)
	return path
}

func writeImageFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ad.png")
	if err := os.WriteFile(path, []byte("pngdata"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	img := writeImageFile(t)
	content = strings.ReplaceAll(content, "{{IMAGE}}", img)
	path := filepath.Join(t.TempDir(), "campaign.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const sampleYAML = `
campaign:
  name: "My Campaign"
  objective: OUTCOME_TRAFFIC
  status: PAUSED
ad_set:
  name: "My Ad Set"
  daily_budget: 1000
  optimization_goal: LINK_CLICKS
  targeting:
    countries: ["US"]
ads:
  - name: "My Ad"
    image: {{IMAGE}}
    primary_text: "Copy"
    headline: "Headline"
    link: "https://example.com"
`

func TestRootVersion(t *testing.T) {
	out, err := runCmd(t, testRunner(t, nil), "--version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out, "0.2.0") {
		t.Errorf("version output = %q", out)
	}
}

func TestSetupClaude(t *testing.T) {
	out, err := runCmd(t, testRunner(t, nil), "setup", "--client", "claude")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !strings.Contains(out, `"mcpServers"`) || !strings.Contains(out, "meta-ads") {
		t.Errorf("setup output = %q", out)
	}
}

func TestSetupChatGPTNotesHosted(t *testing.T) {
	out, err := runCmd(t, testRunner(t, nil), "setup", "--client", "chatgpt")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !strings.Contains(out, "hosted MCP endpoint") {
		t.Errorf("chatgpt output = %q", out)
	}
}

func TestValidateValidConfig(t *testing.T) {
	path := writeConfigFile(t, sampleYAML)
	out, err := runCmd(t, testRunner(t, nil), "validate", "--config", path)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	for _, want := range []string{"Config is valid.", "Campaign: My Campaign", "$10.00/day"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in: %s", want, out)
		}
	}
}

func TestValidateInvalidConfigFails(t *testing.T) {
	path := writeConfigFile(t, `
campaign:
  objective: OUTCOME_NOPE
ads:
  - name: "a"
    primary_text: "b"
`)
	out, err := runCmd(t, testRunner(t, nil), "validate", "--config", path)
	if err == nil {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(out, "Validation failed") {
		t.Errorf("output = %q", out)
	}
}

func TestCreateDryRun(t *testing.T) {
	auditPath := auditDir(t)
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("dry run must not hit the API")
	})
	path := writeConfigFile(t, sampleYAML)

	pt := testRunner(t, srv)
	out, err := runCmd(t, pt, "create", "--config", path, "--dry-run")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.Contains(out, "Mode:      DRY RUN") {
		t.Errorf("output missing dry run mode:\n%s", out)
	}
	if !strings.Contains(out, "[DRY RUN] POST act_123456/campaigns") {
		t.Errorf("output missing dry run preview:\n%s", out)
	}
	if !strings.Contains(out, "dry_run_") {
		t.Errorf("output missing dry run id:\n%s", out)
	}
	if len(*reqs) != 0 {
		t.Errorf("expected no API requests in dry run")
	}
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("dry run should still write an audit event: %v", err)
	}
	if !strings.Contains(string(raw), `"dry_run":true`) || !strings.Contains(string(raw), `"action":"create"`) {
		t.Errorf("audit = %s", raw)
	}
}

func TestCreateLiveYes(t *testing.T) {
	auditPath := auditDir(t)
	var n int
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		n++
		if strings.HasSuffix(r.URL.Path, "/adimages") {
			jsonResp(w, 200, map[string]any{"images": map[string]any{"k": map[string]string{"hash": "h" + strings.TrimPrefix(r.URL.Query().Get("x"), "")}}})
			return
		}
		jsonResp(w, 200, map[string]string{"id": "x_1"})
	})
	path := writeConfigFile(t, sampleYAML)

	out, err := runCmd(t, testRunner(t, srv), "create", "--config", path, "--yes")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	wantPaths := []string{
		"/act_123456/campaigns",
		"/act_123456/adsets",
		"/act_123456/adcreatives",
		"/act_123456/ads",
		"/act_123456/adimages",
	}
	seen := map[string]bool{}
	for _, r := range *reqs {
		seen[r.path] = true
	}
	for _, p := range wantPaths {
		if !seen[p] {
			t.Errorf("missing request to %s, got %v", p, seen)
		}
	}
	if len(seen) != len(wantPaths) {
		t.Errorf("request count = %d, want %d", len(seen), len(wantPaths))
	}
	if !strings.Contains(out, "Done!") || !strings.Contains(out, "x_1") {
		t.Errorf("output:\n%s", out)
	}
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit not written: %v", err)
	}
	if !strings.Contains(string(raw), `"action":"create"`) {
		t.Errorf("audit = %s", raw)
	}
}

func TestCreateConfigError(t *testing.T) {
	path := writeConfigFile(t, `campaign:
  objective: OUTCOME_NOPE
`)
	out, err := runCmd(t, testRunner(t, nil), "create", "--config", path)
	if err == nil {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(out, "Config error") {
		t.Errorf("output = %q", out)
	}
}

func TestCreateDryRunConfirmationNotAsked(t *testing.T) {
	path := writeConfigFile(t, sampleYAML)
	out, err := runCmd(t, testRunner(t, nil), "create", "--config", path, "--dry-run")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if strings.Contains(out, "Continue?") {
		t.Errorf("dry run must not ask for confirmation:\n%s", out)
	}
}

func TestAccountJSON(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]any{"id": "act_123456", "name": "Acme", "account_status": "1"})
	})
	out, err := runCmd(t, testRunner(t, srv), "account", "--json-output")
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	if !strings.Contains(out, `"name": "Acme"`) {
		t.Errorf("output = %q", out)
	}
	if (*reqs)[0].path != "/act_123456" {
		t.Errorf("path = %q", (*reqs)[0].path)
	}
}

func TestCampaignsJSON(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]any{"data": []map[string]any{{"id": "1", "name": "A", "status": "PAUSED"}}})
	})
	out, err := runCmd(t, testRunner(t, srv), "campaigns", "--json-output", "--limit", "5")
	if err != nil {
		t.Fatalf("campaigns: %v", err)
	}
	var payload struct {
		Campaigns []map[string]any `json:"campaigns"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if len(payload.Campaigns) != 1 {
		t.Errorf("campaigns = %v", payload.Campaigns)
	}
	if (*reqs)[0].query.Get("limit") != "5" {
		t.Errorf("limit param = %q", (*reqs)[0].query.Get("limit"))
	}
}

func TestCampaignsLimitBounded(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]any{"data": []map[string]any{}})
	})
	if _, err := runCmd(t, testRunner(t, srv), "campaigns", "--limit", "9999"); err != nil {
		t.Fatalf("campaigns: %v", err)
	}
	if got := (*reqs)[0].query.Get("limit"); got != "100" {
		t.Errorf("limit = %q, want bounded to 100", got)
	}
}

func TestInsightsJSON(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]any{"data": []map[string]any{{"spend": "10", "clicks": "2"}}})
	})
	out, err := runCmd(t, testRunner(t, srv), "insights", "--level", "campaign", "--date-preset", "last_30d", "--json-output")
	if err != nil {
		t.Fatalf("insights: %v", err)
	}
	var payload struct {
		ObjectID   string           `json:"object_id"`
		Level      string           `json:"level"`
		DatePreset string           `json:"date_preset"`
		Insights   []map[string]any `json:"insights"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if payload.ObjectID != "act_123456" {
		t.Errorf("object_id = %q", payload.ObjectID)
	}
	q := (*reqs)[0].query
	if q.Get("level") != "campaign" || q.Get("date_preset") != "last_30d" {
		t.Errorf("query = %v", q)
	}
}

func TestStatusCommand(t *testing.T) {
	srv, _ := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/camp123":
			jsonResp(w, 200, map[string]any{"id": "camp123", "name": "Big", "status": "PAUSED", "objective": "OUTCOME_TRAFFIC"})
		case "/camp123/adsets":
			jsonResp(w, 200, map[string]any{"data": []map[string]any{{"name": "Set", "status": "PAUSED", "daily_budget": "1000"}}})
		default:
			jsonResp(w, 200, map[string]any{"data": []map[string]any{{"name": "Ad", "status": "ACTIVE", "effective_status": "ACTIVE"}}})
		}
	})
	out, err := runCmd(t, testRunner(t, srv), "status", "camp123")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "Campaign: Big") || !strings.Contains(out, "Set: PAUSED ($10.00/day)") {
		t.Errorf("output = %q", out)
	}
}

func TestPauseCommand(t *testing.T) {
	auditPath := auditDir(t)
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]any{"success": true})
	})
	out, err := runCmd(t, testRunner(t, srv), "pause", "camp123")
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if !strings.Contains(out, "paused") {
		t.Errorf("output = %q", out)
	}
	if (*reqs)[0].path != "/camp123" || (*reqs)[0].query.Get("status") != "PAUSED" {
		t.Errorf("req = %+v", (*reqs)[0])
	}
	raw, _ := os.ReadFile(auditPath)
	if !strings.Contains(string(raw), `"action":"pause"`) {
		t.Errorf("audit = %s", raw)
	}
}

func TestActivateYes(t *testing.T) {
	auditPath := auditDir(t)
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]any{"success": true})
	})
	out, err := runCmd(t, testRunner(t, srv), "activate", "camp123", "--yes")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !strings.Contains(out, "activated") {
		t.Errorf("output = %q", out)
	}
	if (*reqs)[0].query.Get("status") != "ACTIVE" {
		t.Errorf("status = %q", (*reqs)[0].query.Get("status"))
	}
	raw, _ := os.ReadFile(auditPath)
	if !strings.Contains(string(raw), `"action":"activate"`) {
		t.Errorf("audit = %s", raw)
	}
}

func TestActivateDeclinedConfirm(t *testing.T) {
	auditPath := auditDir(t)
	out, err := runCmd(t, testRunner(t, nil), "activate", "camp123")
	if err != nil {
		t.Fatalf("activate should exit 0 when declined, got err: %v", err)
	}
	if !strings.Contains(out, "Aborted.") {
		t.Errorf("output = %q", out)
	}
	if _, err := os.Stat(auditPath); err == nil {
		t.Errorf("declined activate must not write audit")
	}
}

func TestDeleteYes(t *testing.T) {
	auditPath := auditDir(t)
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]any{"success": true})
	})
	out, err := runCmd(t, testRunner(t, srv), "delete", "camp123", "--yes")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !strings.Contains(out, "deleted") {
		t.Errorf("output = %q", out)
	}
	if (*reqs)[0].query.Get("status") != "DELETED" {
		t.Errorf("status = %q", (*reqs)[0].query.Get("status"))
	}
	raw, _ := os.ReadFile(auditPath)
	if !strings.Contains(string(raw), `"action":"delete"`) {
		t.Errorf("audit = %s", raw)
	}
}

func TestBudgetDryRun(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("dry run must not hit API")
	})
	out, err := runCmd(t, testRunner(t, srv), "budget", "camp123", "3000")
	if err != nil {
		t.Fatalf("budget: %v", err)
	}
	if !strings.Contains(out, "Budget update accepted.") || !strings.Contains(out, "Dry run only") {
		t.Errorf("output = %q", out)
	}
	if len(*reqs) != 0 {
		t.Errorf("expected no requests in dry run")
	}
}

func TestBudgetLiveYes(t *testing.T) {
	auditPath := auditDir(t)
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]any{"success": true})
	})
	_, err := runCmd(t, testRunner(t, srv), "budget", "camp123", "3000", "--live", "--yes")
	if err != nil {
		t.Fatalf("budget: %v", err)
	}
	if (*reqs)[0].path != "/camp123" || (*reqs)[0].query.Get("daily_budget") != "3000" {
		t.Errorf("req = %+v", (*reqs)[0])
	}
	raw, _ := os.ReadFile(auditPath)
	if !strings.Contains(string(raw), `"action":"budget"`) {
		t.Errorf("audit = %s", raw)
	}
}

func TestBudgetAboveCapBlocked(t *testing.T) {
	t.Setenv("META_ADS_MAX_DAILY_BUDGET_CENTS", "5000")
	out, err := runCmd(t, testRunner(t, nil), "budget", "camp123", "9000", "--live", "--yes")
	if err == nil {
		t.Fatal("expected error for budget above cap")
	}
	if !strings.Contains(out, "exceeds") {
		t.Errorf("output = %q", out)
	}
}

func TestBudgetNonPositiveBlocked(t *testing.T) {
	out, err := runCmd(t, testRunner(t, nil), "budget", "camp123", "0")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(out, "must be positive") {
		t.Errorf("output = %q", out)
	}
}

func TestUploadImageLiveYes(t *testing.T) {
	auditPath := auditDir(t)
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]any{"images": map[string]any{"k": map[string]string{"hash": "abc123"}}})
	})
	img := writeImageFile(t)
	out, err := runCmd(t, testRunner(t, srv), "upload-image", img, "--live", "--yes")
	if err != nil {
		t.Fatalf("upload-image: %v", err)
	}
	if !strings.Contains(out, "abc123") {
		t.Errorf("output = %q", out)
	}
	if len(*reqs) != 1 {
		t.Errorf("reqs = %d", len(*reqs))
	}
	raw, _ := os.ReadFile(auditPath)
	if !strings.Contains(string(raw), `"action":"upload-image"`) {
		t.Errorf("audit = %s", raw)
	}
}

func TestUploadImageDryRun(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("dry run must not hit API")
	})
	img := writeImageFile(t)
	out, err := runCmd(t, testRunner(t, srv), "upload-image", img)
	if err != nil {
		t.Fatalf("upload-image: %v", err)
	}
	if !strings.Contains(out, "dry_run_hash") {
		t.Errorf("output = %q", out)
	}
	if len(*reqs) != 0 {
		t.Errorf("reqs = %d, want 0", len(*reqs))
	}
}

func TestUploadImageMissingFile(t *testing.T) {
	out, err := runCmd(t, testRunner(t, nil), "upload-image", "/does/not/exist.png", "--live", "--yes")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(out, "Image not found") {
		t.Errorf("output = %q", out)
	}
}

func TestBulkStatusLiveYes(t *testing.T) {
	auditPath := auditDir(t)
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]any{"success": true})
	})
	out, err := runCmd(t, testRunner(t, srv), "bulk-status", "PAUSED", "111", "222", "--live", "--yes")
	if err != nil {
		t.Fatalf("bulk-status: %v", err)
	}
	if !strings.Contains(out, "2 campaign updates accepted") {
		t.Errorf("output = %q", out)
	}
	if len(*reqs) != 2 {
		t.Errorf("reqs = %d, want 2", len(*reqs))
	}
	for _, r := range *reqs {
		if r.query.Get("status") != "PAUSED" {
			t.Errorf("status = %q", r.query.Get("status"))
		}
	}
	raw, _ := os.ReadFile(auditPath)
	if !strings.Contains(string(raw), `"action":"bulk-status"`) {
		t.Errorf("audit = %s", raw)
	}
}

func TestBulkStatusDryRun(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("dry run must not hit API")
	})
	out, err := runCmd(t, testRunner(t, srv), "bulk-status", "PAUSED", "111", "222")
	if err != nil {
		t.Fatalf("bulk-status: %v", err)
	}
	if !strings.Contains(out, "Dry run only") {
		t.Errorf("output = %q", out)
	}
	if len(*reqs) != 0 {
		t.Errorf("reqs = %d, want 0", len(*reqs))
	}
}

func TestBulkStatusInvalidStatus(t *testing.T) {
	out, err := runCmd(t, testRunner(t, nil), "bulk-status", "NOPE", "111")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(out, "PAUSED, ACTIVE, or DELETED") {
		t.Errorf("output = %q", out)
	}
}

func TestMissingEnvVars(t *testing.T) {
	for _, key := range []string{"META_ACCESS_TOKEN", "META_AD_ACCOUNT_ID", "META_PAGE_ID"} {
		t.Setenv(key, "")
	}
	pt := &Runner{
		Confirm:   func(string) bool { return false },
		NewClient: nil,
	}
	out, err := runCmd(t, pt, "account")
	if err == nil {
		t.Fatal("expected error when env missing")
	}
	if !strings.Contains(out, "Missing required environment variables") {
		t.Errorf("output = %q", out)
	}
}

func TestSetupDefaultClient(t *testing.T) {
	out, err := runCmd(t, nil, "setup")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !strings.Contains(out, "mcpServers") {
		t.Errorf("output = %q", out)
	}
}

func TestBoundedLimit(t *testing.T) {
	cases := []struct{ in, want int }{
		{25, 25}, {0, 1}, {-5, 1}, {100, 100}, {9999, 100},
	}
	for _, c := range cases {
		if got := boundedLimit(c.in); got != c.want {
			t.Errorf("boundedLimit(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestRandomAuditWarningOnFailure(t *testing.T) {
	auditPath := auditDir(t)
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 400, map[string]any{"error": map[string]any{"message": "Invalid token", "code": 190}})
	})
	path := writeConfigFile(t, sampleYAML)
	out, err := runCmd(t, testRunner(t, srv), "create", "--config", path, "--yes")
	if err == nil {
		t.Fatal("expected error for API failure")
	}
	if !strings.Contains(out, "API Error: Invalid token") {
		t.Errorf("output = %q", out)
	}
	if len(*reqs) == 0 {
		t.Error("expected API requests")
	}
	raw, _ := os.ReadFile(auditPath)
	if !strings.Contains(string(raw), `"action":"create"`) || !strings.Contains(string(raw), `"success":false`) {
		t.Errorf("audit = %s", raw)
	}
}

func TestInsightsDefaultObjectID(t *testing.T) {
	srv, reqs := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]any{"data": []map[string]any{}})
	})
	if _, err := runCmd(t, testRunner(t, srv), "insights", "camp555", "--date-preset", "yesterday", "--json-output"); err != nil {
		t.Fatalf("insights: %v", err)
	}
	req := (*reqs)[0]
	if req.path != "/camp555/insights" {
		t.Errorf("path = %q", req.path)
	}
	if req.query.Get("date_preset") != "yesterday" {
		t.Errorf("date_preset = %q", req.query.Get("date_preset"))
	}
}

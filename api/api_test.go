package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type capturedRequest struct {
	method string
	path   string
	query  url.Values
	body   []byte
	r      *http.Request
}

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]capturedRequest) {
	t.Helper()
	var mu sync.Mutex
	var reqs []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqs = append(reqs, capturedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
			body:   body,
			r:      r,
		})
		mu.Unlock()
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs
}

func jsonResponse(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func newClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return New(Config{
		AccessToken: "token-123",
		AdAccountID: "123456",
		PageID:      "98765",
		BaseURL:     srv.URL,
	})
}

func TestNewDefaultsToV21(t *testing.T) {
	c := New(Config{AdAccountID: "1"})
	if !strings.HasSuffix(c.BaseURL, "/v21.0") {
		t.Errorf("BaseURL = %q, want suffix v21.0", c.BaseURL)
	}
}

func TestActID(t *testing.T) {
	c := New(Config{AdAccountID: "555"})
	if got := c.ActID(); got != "act_555" {
		t.Errorf("ActID = %q, want act_555", got)
	}
}

func TestCreateCampaign(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"id": "camp1"})
	})
	c := newClient(t, srv)

	id, err := c.CreateCampaign(CreateCampaignParams{
		Name:                "My Campaign",
		Objective:           "OUTCOME_TRAFFIC",
		Status:              "PAUSED",
		SpecialAdCategories: []string{"CREDIT"},
	})
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	if id != "camp1" {
		t.Errorf("id = %q, want camp1", id)
	}
	req := (*reqs)[0]
	if req.method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.method)
	}
	if req.path != "/act_123456/campaigns" {
		t.Errorf("path = %q", req.path)
	}
	if req.query.Get("access_token") != "token-123" {
		t.Errorf("access_token missing")
	}
	if req.query.Get("name") != "My Campaign" {
		t.Errorf("name param = %q", req.query.Get("name"))
	}
	if req.query.Get("objective") != "OUTCOME_TRAFFIC" {
		t.Errorf("objective param = %q", req.query.Get("objective"))
	}
	if req.query.Get("status") != "PAUSED" {
		t.Errorf("status param = %q", req.query.Get("status"))
	}
	if req.query.Get("special_ad_categories") != `["CREDIT"]` {
		t.Errorf("special_ad_categories param = %q", req.query.Get("special_ad_categories"))
	}
}

func TestCreateCampaignEmptyCategories(t *testing.T) {
	srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"id": "c"})
	})
	c := newClient(t, srv)
	if _, err := c.CreateCampaign(CreateCampaignParams{Name: "n"}); err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
}

func TestCreateAdSet(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"id": "set1"})
	})
	c := newClient(t, srv)

	id, err := c.CreateAdSet(CreateAdSetParams{
		Name:             "My Set",
		CampaignID:       "camp1",
		DailyBudget:      1000,
		OptimizationGoal: "LINK_CLICKS",
		Targeting: Targeting{
			AgeMin:             18,
			AgeMax:             65,
			Genders:            []int{0},
			Countries:          []string{"US"},
			Interests:          []Interest{{ID: "6003139266461", Name: "Fitness"}},
			Platforms:          []string{"facebook", "instagram"},
			FacebookPositions:  []string{"feed"},
			InstagramPositions: []string{"stream", "story", "reels"},
		},
	})
	if err != nil {
		t.Fatalf("CreateAdSet: %v", err)
	}
	if id != "set1" {
		t.Errorf("id = %q, want set1", id)
	}
	req := (*reqs)[0]
	if req.method != http.MethodPost || req.path != "/act_123456/adsets" {
		t.Fatalf("got %s %s", req.method, req.path)
	}
	if req.query.Get("campaign_id") != "camp1" {
		t.Errorf("campaign_id = %q", req.query.Get("campaign_id"))
	}
	if req.query.Get("daily_budget") != "1000" {
		t.Errorf("daily_budget = %q", req.query.Get("daily_budget"))
	}
	var spec map[string]any
	if err := json.Unmarshal([]byte(req.query.Get("targeting")), &spec); err != nil {
		t.Fatalf("targeting is not valid JSON: %v", err)
	}
	if spec["age_min"] != float64(18) || spec["age_max"] != float64(65) {
		t.Errorf("age mismatch: %v", spec["age_min"])
	}
	geo := spec["geo_locations"].(map[string]any)
	countries := geo["countries"].([]any)
	if countries[0] != "US" {
		t.Errorf("countries = %v", countries)
	}
	if spec["publisher_platforms"] == nil {
		t.Errorf("publisher_platforms missing")
	}
	fspec := spec["flexible_spec"].([]any)
	interests := fspec[0].(map[string]any)["interests"].([]any)
	if interests[0].(map[string]any)["id"] != "6003139266461" {
		t.Errorf("interests = %v", interests)
	}
}

func TestCreateAdSetDefaultsTargeting(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"id": "set1"})
	})
	c := newClient(t, srv)
	if _, err := c.CreateAdSet(CreateAdSetParams{Name: "s", CampaignID: "c", DailyBudget: 100}); err != nil {
		t.Fatalf("CreateAdSet: %v", err)
	}
	req := (*reqs)[0]
	var spec map[string]any
	json.Unmarshal([]byte(req.query.Get("targeting")), &spec)
	if spec["age_min"] != float64(18) {
		t.Errorf("default age_min = %v", spec["age_min"])
	}
	if spec["age_max"] != float64(65) {
		t.Errorf("default age_max = %v", spec["age_max"])
	}
	if spec["publisher_platforms"] == nil {
		t.Errorf("default platforms missing")
	}
	if spec["facebook_positions"] == nil {
		t.Errorf("default facebook_positions missing")
	}
	if spec["instagram_positions"] == nil {
		t.Errorf("default instagram_positions missing")
	}
}

func TestCreateAdCreative(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"id": "cr1"})
	})
	c := newClient(t, srv)

	id, err := c.CreateAdCreative(CreateAdCreativeParams{
		Name:        "Creative",
		ImageHash:   "hash1",
		PrimaryText: "Copy here",
		Headline:    "Big Headline",
		Description: "Desc",
		Link:        "https://example.com",
		CTA:         "LEARN_MORE",
	})
	if err != nil {
		t.Fatalf("CreateAdCreative: %v", err)
	}
	if id != "cr1" {
		t.Errorf("id = %q, want cr1", id)
	}
	req := (*reqs)[0]
	if req.method != http.MethodPost || req.path != "/act_123456/adcreatives" {
		t.Fatalf("got %s %s", req.method, req.path)
	}
	var spec map[string]any
	if err := json.Unmarshal([]byte(req.query.Get("object_story_spec")), &spec); err != nil {
		t.Fatalf("object_story_spec invalid JSON: %v", err)
	}
	if spec["page_id"] != "98765" {
		t.Errorf("page_id = %v", spec["page_id"])
	}
	linkData := spec["link_data"].(map[string]any)
	if linkData["image_hash"] != "hash1" {
		t.Errorf("image_hash = %v", linkData["image_hash"])
	}
	if linkData["message"] != "Copy here" {
		t.Errorf("message = %v", linkData["message"])
	}
	cta := linkData["call_to_action"].(map[string]any)
	if cta["type"] != "LEARN_MORE" {
		t.Errorf("cta type = %v", cta["type"])
	}
}

func TestCreateAd(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"id": "ad1"})
	})
	c := newClient(t, srv)

	id, err := c.CreateAd(CreateAdParams{Name: "Ad", AdSetID: "set1", CreativeID: "cr1", Status: "PAUSED"})
	if err != nil {
		t.Fatalf("CreateAd: %v", err)
	}
	if id != "ad1" {
		t.Errorf("id = %q, want ad1", id)
	}
	req := (*reqs)[0]
	if req.method != http.MethodPost || req.path != "/act_123456/ads" {
		t.Fatalf("got %s %s", req.method, req.path)
	}
	if req.query.Get("adset_id") != "set1" {
		t.Errorf("adset_id = %q", req.query.Get("adset_id"))
	}
	if req.query.Get("creative") != `{"creative_id":"cr1"}` {
		t.Errorf("creative = %q", req.query.Get("creative"))
	}
	if req.query.Get("status") != "PAUSED" {
		t.Errorf("status = %q", req.query.Get("status"))
	}
}

func TestUploadImage(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{
			"images": map[string]any{
				"k": map[string]string{"hash": "abc123"},
			},
		})
	})
	c := newClient(t, srv)

	hash, err := c.UploadImage(UploadImageParams{
		FilePath:    "/tmp/ad.png",
		FileName:    "ad.png",
		ContentType: "image/png",
		Data:        []byte("PNGDATA"),
	})
	if err != nil {
		t.Fatalf("UploadImage: %v", err)
	}
	if hash != "abc123" {
		t.Errorf("hash = %q, want abc123", hash)
	}
	req := (*reqs)[0]
	if req.method != http.MethodPost || req.path != "/act_123456/adimages" {
		t.Fatalf("got %s %s", req.method, req.path)
	}
	contentType := req.r.Header.Get("Content-Type")
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	mr := multipart.NewReader(bytes.NewReader(req.body), params["boundary"])
	form, err := mr.ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("parse multipart: %v", err)
	}
	f := form.File["filename"]
	if len(f) != 1 {
		t.Fatalf("filename part count = %d, want 1", len(f))
	}
	if f[0].Filename != "ad.png" {
		t.Errorf("filename = %q", f[0].Filename)
	}
	file, _ := f[0].Open()
	data, _ := io.ReadAll(file)
	if string(data) != "PNGDATA" {
		t.Errorf("file bytes = %q", string(data))
	}
}

func TestUploadImageNoHash(t *testing.T) {
	srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"images": map[string]any{}})
	})
	c := newClient(t, srv)
	_, err := c.UploadImage(UploadImageParams{Data: []byte("x")})
	if err == nil {
		t.Fatal("expected error when response has no images")
	}
}

func TestUpdateStatus(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"success": "true"})
	})
	c := newClient(t, srv)
	if err := c.UpdateStatus("camp1", "PAUSED"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	req := (*reqs)[0]
	if req.method != http.MethodPost || req.path != "/camp1" {
		t.Fatalf("got %s %s", req.method, req.path)
	}
	if req.query.Get("status") != "PAUSED" {
		t.Errorf("status = %q", req.query.Get("status"))
	}
}

func TestUpdateDailyBudget(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"success": "true"})
	})
	c := newClient(t, srv)
	if err := c.UpdateDailyBudget("set1", 3000); err != nil {
		t.Fatalf("UpdateDailyBudget: %v", err)
	}
	req := (*reqs)[0]
	if req.path != "/set1" {
		t.Errorf("path = %q", req.path)
	}
	if req.query.Get("daily_budget") != "3000" {
		t.Errorf("daily_budget = %q", req.query.Get("daily_budget"))
	}
}

func TestDeleteCampaignUsesDELETED(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{"success": "true"})
	})
	c := newClient(t, srv)
	if err := c.DeleteCampaign("camp1"); err != nil {
		t.Fatalf("DeleteCampaign: %v", err)
	}
	req := (*reqs)[0]
	if req.query.Get("status") != "DELETED" {
		t.Errorf("status = %q, want DELETED", req.query.Get("status"))
	}
}

func TestListCampaigns(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{
			"data": []map[string]string{
				{"id": "1", "name": "A", "status": "PAUSED"},
				{"id": "2", "name": "B", "status": "ACTIVE"},
			},
		})
	})
	c := newClient(t, srv)
	rows, err := c.ListCampaigns(10)
	if err != nil {
		t.Fatalf("ListCampaigns: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0]["id"] != "1" || rows[1]["name"] != "B" {
		t.Errorf("rows = %v", rows)
	}
	req := (*reqs)[0]
	if req.method != http.MethodGet || req.path != "/act_123456/campaigns" {
		t.Fatalf("got %s %s", req.method, req.path)
	}
	if req.query.Get("limit") != "10" {
		t.Errorf("limit = %q", req.query.Get("limit"))
	}
}

func TestGetAdAccount(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"id": "act_123456", "name": "My Account"})
	})
	c := newClient(t, srv)
	data, err := c.GetAdAccount()
	if err != nil {
		t.Fatalf("GetAdAccount: %v", err)
	}
	if data["name"] != "My Account" {
		t.Errorf("name = %v", data["name"])
	}
	req := (*reqs)[0]
	if req.path != "/act_123456" {
		t.Errorf("path = %q", req.path)
	}
	if req.query.Get("fields") == "" {
		t.Errorf("fields param missing")
	}
}

func TestGetCampaign(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"id": "camp1", "name": "C", "status": "PAUSED", "objective": "OUTCOME_TRAFFIC"})
	})
	c := newClient(t, srv)
	data, err := c.GetCampaign("camp1", "name,status,objective")
	if err != nil {
		t.Fatalf("GetCampaign: %v", err)
	}
	if data["objective"] != "OUTCOME_TRAFFIC" {
		t.Errorf("objective = %v", data["objective"])
	}
	req := (*reqs)[0]
	if req.path != "/camp1" {
		t.Errorf("path = %q", req.path)
	}
	if req.query.Get("fields") != "name,status,objective" {
		t.Errorf("fields = %q", req.query.Get("fields"))
	}
}

func TestGetAdSetsAndAdsNested(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"data": []map[string]string{{"id": "s1", "name": "Set"}}})
	})
	c := newClient(t, srv)
	if _, err := c.GetAdSets("camp1", "name,status,daily_budget"); err != nil {
		t.Fatalf("GetAdSets: %v", err)
	}
	if _, err := c.GetAds("camp1", "name,status,effective_status"); err != nil {
		t.Fatalf("GetAds: %v", err)
	}
	if (*reqs)[0].path != "/camp1/adsets" {
		t.Errorf("adsets path = %q", (*reqs)[0].path)
	}
	if (*reqs)[1].path != "/camp1/ads" {
		t.Errorf("ads path = %q", (*reqs)[1].path)
	}
}

func TestGetInsights(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"data": []map[string]string{{"spend": "12.34"}}})
	})
	c := newClient(t, srv)
	rows, err := c.GetInsights(GetInsightsParams{
		ObjectID:   "act_123456",
		Level:      "campaign",
		DatePreset: "last_7d",
		Limit:      25,
	})
	if err != nil {
		t.Fatalf("GetInsights: %v", err)
	}
	if len(rows) != 1 || rows[0]["spend"] != "12.34" {
		t.Errorf("rows = %v", rows)
	}
	req := (*reqs)[0]
	if req.path != "/act_123456/insights" {
		t.Errorf("path = %q", req.path)
	}
	if req.query.Get("level") != "campaign" {
		t.Errorf("level = %q", req.query.Get("level"))
	}
	if req.query.Get("date_preset") != "last_7d" {
		t.Errorf("date_preset = %q", req.query.Get("date_preset"))
	}
	if req.query.Get("fields") == "" {
		t.Errorf("fields param missing")
	}
}

func TestGetInsightsWithoutLevel(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"data": []map[string]string{}})
	})
	c := newClient(t, srv)
	if _, err := c.GetInsights(GetInsightsParams{ObjectID: "camp1", DatePreset: "yesterday", Limit: 5}); err != nil {
		t.Fatalf("GetInsights: %v", err)
	}
	req := (*reqs)[0]
	if req.query.Get("level") != "" {
		t.Errorf("level should be empty, got %q", req.query.Get("level"))
	}
	if req.query.Get("date_preset") != "yesterday" {
		t.Errorf("date_preset = %q", req.query.Get("date_preset"))
	}
	if req.query.Get("limit") != "5" {
		t.Errorf("limit = %q", req.query.Get("limit"))
	}
}

func TestAPIErrorParsesMetaBody(t *testing.T) {
	srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 400, map[string]any{
			"error": map[string]any{
				"message": "Invalid parameter",
				"code":    100,
			},
		})
	})
	c := newClient(t, srv)
	_, err := c.GetCampaign("bad", "name")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != 400 || apiErr.MetaCode != 100 || !strings.Contains(apiErr.Message, "Invalid parameter") {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestAPIErrorNonJSONBody(t *testing.T) {
	srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("boom"))
	})
	c := newClient(t, srv)
	_, err := c.GetCampaign("bad", "name")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want body text", err)
	}
}

func TestDryRunCreateReturnsFakeIDs(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("dry run must not issue HTTP requests")
	})
	c := New(Config{
		AccessToken: "t",
		AdAccountID: "123456",
		BaseURL:     srv.URL,
		DryRun:      true,
	})

	id, err := c.CreateCampaign(CreateCampaignParams{Name: "c"})
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	if id != "dry_run_1" {
		t.Errorf("id = %q, want dry_run_1", id)
	}
	if _, err := c.CreateAdSet(CreateAdSetParams{Name: "s"}); err != nil {
		t.Fatalf("CreateAdSet: %v", err)
	}
	if len(*reqs) != 0 {
		t.Errorf("expected no HTTP requests, got %d", len(*reqs))
	}
}

func TestDryRunUploadImage(t *testing.T) {
	srv, reqs := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("dry run must not issue HTTP requests")
	})
	c := New(Config{AdAccountID: "1", BaseURL: srv.URL, DryRun: true})
	hash, err := c.UploadImage(UploadImageParams{Data: []byte("x")})
	if err != nil {
		t.Fatalf("UploadImage: %v", err)
	}
	if hash != "dry_run_hash" {
		t.Errorf("hash = %q, want dry_run_hash", hash)
	}
	if len(*reqs) != 0 {
		t.Errorf("expected no HTTP requests in dry run")
	}
}

func TestDryRunListEmpty(t *testing.T) {
	c := New(Config{AdAccountID: "1", DryRun: true})
	rows, err := c.ListCampaigns(10)
	if err != nil {
		t.Fatalf("ListCampaigns: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0 in dry run", len(rows))
	}
}

func TestDryRunUpdateStatus(t *testing.T) {
	c := New(Config{AdAccountID: "1", DryRun: true})
	if err := c.UpdateStatus("camp1", "PAUSED"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
}

func TestDryRunPreviewOutput(t *testing.T) {
	var out strings.Builder
	c := New(Config{AdAccountID: "1", DryRun: true, DryRunOut: &out})
	if _, err := c.CreateCampaign(CreateCampaignParams{Name: "C", Objective: "OUTCOME_TRAFFIC"}); err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "[DRY RUN] POST act_1/campaigns") {
		t.Errorf("missing dry run line: %q", s)
	}
	if strings.Contains(s, "token") {
		t.Errorf("dry run preview must not leak access token: %q", s)
	}
	if !strings.Contains(s, "OUTCOME_TRAFFIC") {
		t.Errorf("preview should include params: %q", s)
	}
}

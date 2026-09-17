package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const defaultAPIVersion = "v21.0"

// APIError is returned when the Meta Graph API responds with an error.
type APIError struct {
	StatusCode int
	MetaCode   int
	Message    string
}

func (e *APIError) Error() string {
	if e.MetaCode != 0 {
		return fmt.Sprintf("%s (HTTP %d, error %d)", e.Message, e.StatusCode, e.MetaCode)
	}
	return fmt.Sprintf("%s (HTTP %d)", e.Message, e.StatusCode)
}

// Config holds client settings.
type Config struct {
	AccessToken string
	AdAccountID string
	PageID      string
	APIVersion  string
	BaseURL     string
	HTTPClient  *http.Client
	DryRun      bool
	// DryRunOut, when set, receives dry-run preview output.
	DryRunOut io.Writer
}

// Client is a lightweight wrapper around the Meta Marketing API.
type Client struct {
	AccessToken  string
	AdAccountID  string
	PageID       string
	APIVersion   string
	BaseURL      string
	DryRun       bool
	dryRunNumber int
	httpClient   *http.Client
	dryRunOut    io.Writer

	// PartialCampaignResult is set by the campaign orchestrator when a
	// creation fails partway, to preserve what was already created.
	PartialCampaignResult map[string]any
}

// New creates a Meta API client from the given config. The base URL defaults
// to https://graph.facebook.com/{api_version}.
func New(cfg Config) *Client {
	version := cfg.APIVersion
	if version == "" {
		version = defaultAPIVersion
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://graph.facebook.com/" + version
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	return &Client{
		AccessToken: cfg.AccessToken,
		AdAccountID: cfg.AdAccountID,
		PageID:      cfg.PageID,
		APIVersion:  version,
		BaseURL:     baseURL,
		DryRun:      cfg.DryRun,
		httpClient:  hc,
		dryRunOut:   cfg.DryRunOut,
	}
}

// ActID returns the ad account ID with the act_ prefix.
func (c *Client) ActID() string {
	return "act_" + c.AdAccountID
}

// SetDryRunOut enables dry-run preview output to w.
func (c *Client) SetDryRunOut(w io.Writer) {
	c.dryRunOut = w
}

// request performs an API request, defaulting to query-string params. In dry
// run mode it returns a fake success body without making an HTTP call.
func (c *Client) request(method, endpoint string, params url.Values) ([]byte, error) {
	u := c.BaseURL + "/" + strings.TrimPrefix(endpoint, "/")
	q := params
	if q == nil {
		q = url.Values{}
	}
	q.Set("access_token", c.AccessToken)

	if c.DryRun {
		c.dryRunNumber++
		c.logDryRun(method, endpoint, q)
		return []byte(fmt.Sprintf(`{"id":"dry_run_%d"}`, c.dryRunNumber)), nil
	}

	req, err := http.NewRequest(method, u+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp.StatusCode, body)
	}
	return body, nil
}

func (c *Client) logDryRun(method, endpoint string, params url.Values) {
	if c.dryRunOut == nil {
		return
	}
	fmt.Fprintf(c.dryRunOut, "  [DRY RUN] %s %s\n", method, strings.TrimPrefix(endpoint, "/"))
	preview := map[string]string{}
	for k, vs := range params {
		if k == "access_token" {
			continue
		}
		preview[k] = vJoin(vs)
	}
	if len(preview) > 0 {
		data, _ := json.MarshalIndent(preview, "", "  ")
		out := string(data)
		if len(out) > 500 {
			out = out[:500] + "..."
		}
		fmt.Fprintf(c.dryRunOut, "  Params: %s\n", out)
	}
}

func vJoin(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

func parseAPIError(statusCode int, body []byte) error {
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	message := string(body)
	metaCode := 0
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error.Message != "" {
		message = payload.Error.Message
		metaCode = payload.Error.Code
	}
	return &APIError{StatusCode: statusCode, MetaCode: metaCode, Message: message}
}

func extractID(body []byte) (string, error) {
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.ID == "" {
		return "dry_run_id", nil
	}
	return result.ID, nil
}

func listData(body []byte) ([]map[string]string, error) {
	var result struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	rows := make([]map[string]string, 0, len(result.Data))
	for _, raw := range result.Data {
		rows = append(rows, coerceStringMap(raw))
	}
	return rows, nil
}

// decodeObject decodes a JSON object into string-keyed string values,
// coercing non-string fields (numbers, booleans, nested structures) to
// their JSON string form so a numeric API field never breaks decoding.
func decodeObject(body []byte) (map[string]string, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return coerceStringMap(raw), nil
}

func coerceStringMap(raw map[string]any) map[string]string {
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = stringifyField(v)
	}
	return out
}

func stringifyField(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}

// CreateCampaignParams configures campaign creation.
type CreateCampaignParams struct {
	Name                string
	Objective           string
	Status              string
	SpecialAdCategories []string
}

// CreateCampaign creates an ad campaign and returns its ID.
func (c *Client) CreateCampaign(p CreateCampaignParams) (string, error) {
	categories := p.SpecialAdCategories
	if categories == nil {
		categories = []string{}
	}
	categoriesJSON, _ := json.Marshal(categories)
	body, err := c.request(http.MethodPost, c.ActID()+"/campaigns", url.Values{
		"name":                            {p.Name},
		"objective":                       {p.Objective},
		"status":                          {p.Status},
		"special_ad_categories":           {string(categoriesJSON)},
		"is_adset_budget_sharing_enabled": {"false"},
	})
	if err != nil {
		return "", err
	}
	return extractID(body)
}

// Interest is a Meta targeting interest.
type Interest struct {
	ID   string
	Name string
}

// Targeting is the ad set targeting spec.
type Targeting struct {
	AgeMin             int
	AgeMax             int
	Genders            []int
	Countries          []string
	Interests          []Interest
	Platforms          []string
	FacebookPositions  []string
	InstagramPositions []string
}

// CreateAdSetParams configures ad set creation.
type CreateAdSetParams struct {
	Name             string
	CampaignID       string
	DailyBudget      int
	OptimizationGoal string
	BillingEvent     string
	BidStrategy      string
	Status           string
	Targeting        Targeting
}

// CreateAdSet creates an ad set with targeting and returns its ID.
func (c *Client) CreateAdSet(p CreateAdSetParams) (string, error) {
	t := p.Targeting
	spec := map[string]any{}
	spec["age_min"] = t.AgeMin
	if spec["age_min"] == 0 {
		spec["age_min"] = 18
	}
	spec["age_max"] = t.AgeMax
	if spec["age_max"] == 0 {
		spec["age_max"] = 65
	}
	genders := t.Genders
	if genders == nil {
		genders = []int{0}
	}
	spec["genders"] = genders

	countries := t.Countries
	if countries == nil {
		countries = []string{"US"}
	}
	spec["geo_locations"] = map[string]any{"countries": countries}

	if len(t.Interests) > 0 {
		interests := make([]map[string]string, 0, len(t.Interests))
		for _, in := range t.Interests {
			interests = append(interests, map[string]string{"id": in.ID, "name": in.Name})
		}
		spec["flexible_spec"] = []any{map[string]any{"interests": interests}}
	}

	platforms := t.Platforms
	if platforms == nil {
		platforms = []string{"facebook", "instagram"}
	}
	spec["publisher_platforms"] = platforms
	for _, platform := range platforms {
		if platform == "facebook" {
			positions := t.FacebookPositions
			if positions == nil {
				positions = []string{"feed"}
			}
			spec["facebook_positions"] = positions
		}
		if platform == "instagram" {
			positions := t.InstagramPositions
			if positions == nil {
				positions = []string{"stream", "story", "reels"}
			}
			spec["instagram_positions"] = positions
		}
	}

	targetingJSON, _ := json.Marshal(spec)
	body, err := c.request(http.MethodPost, c.ActID()+"/adsets", url.Values{
		"name":              {p.Name},
		"campaign_id":       {p.CampaignID},
		"daily_budget":      {fmt.Sprintf("%d", p.DailyBudget)},
		"billing_event":     {p.BillingEvent},
		"optimization_goal": {p.OptimizationGoal},
		"bid_strategy":      {p.BidStrategy},
		"status":            {p.Status},
		"targeting":         {string(targetingJSON)},
	})
	if err != nil {
		return "", err
	}
	return extractID(body)
}

// CreateAdCreativeParams configures ad creative creation.
type CreateAdCreativeParams struct {
	Name        string
	ImageHash   string
	PrimaryText string
	Headline    string
	Description string
	Link        string
	CTA         string
}

// CreateAdCreative creates an ad creative and returns its ID.
func (c *Client) CreateAdCreative(p CreateAdCreativeParams) (string, error) {
	story := map[string]any{
		"link_data": map[string]any{
			"image_hash":  p.ImageHash,
			"link":        p.Link,
			"message":     p.PrimaryText,
			"name":        p.Headline,
			"description": p.Description,
			"call_to_action": map[string]any{
				"type":  p.CTA,
				"value": map[string]any{"link": p.Link},
			},
		},
		"page_id": c.PageID,
	}
	storyJSON, _ := json.Marshal(story)
	body, err := c.request(http.MethodPost, c.ActID()+"/adcreatives", url.Values{
		"name":              {p.Name},
		"object_story_spec": {string(storyJSON)},
	})
	if err != nil {
		return "", err
	}
	return extractID(body)
}

// CreateAdParams configures ad creation.
type CreateAdParams struct {
	Name       string
	AdSetID    string
	CreativeID string
	Status     string
}

// CreateAd creates an ad linking a creative to an ad set and returns its ID.
func (c *Client) CreateAd(p CreateAdParams) (string, error) {
	creativeJSON, _ := json.Marshal(map[string]string{"creative_id": p.CreativeID})
	body, err := c.request(http.MethodPost, c.ActID()+"/ads", url.Values{
		"name":     {p.Name},
		"adset_id": {p.AdSetID},
		"creative": {string(creativeJSON)},
		"status":   {p.Status},
	})
	if err != nil {
		return "", err
	}
	return extractID(body)
}

// UploadImageParams configures an image upload.
type UploadImageParams struct {
	FilePath    string
	FileName    string
	ContentType string
	Data        []byte
}

// UploadImage uploads an ad image to the ad account and returns its hash.
func (c *Client) UploadImage(p UploadImageParams) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("filename", p.FileName)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(p.Data); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	if c.DryRun {
		c.dryRunNumber++
		if c.dryRunOut != nil {
			fmt.Fprintf(c.dryRunOut, "  [DRY RUN] POST %s/adimages\n", strings.TrimPrefix(c.ActID(), "/"))
			fmt.Fprintf(c.dryRunOut, "  Files: [%s]\n", p.FileName)
		}
		return "dry_run_hash", nil
	}

	params := url.Values{"access_token": {c.AccessToken}}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/"+c.ActID()+"/adimages"+"?"+params.Encode(), &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", parseAPIError(resp.StatusCode, body)
	}

	var result struct {
		Images map[string]struct {
			Hash string `json:"hash"`
		} `json:"images"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	for _, val := range result.Images {
		if val.Hash != "" {
			return val.Hash, nil
		}
	}
	return "", &APIError{StatusCode: 0, Message: fmt.Sprintf("Unexpected image upload response: %s", string(body))}
}

// GetCampaign fetches campaign details.
func (c *Client) GetCampaign(campaignID, fields string) (map[string]string, error) {
	body, err := c.request(http.MethodGet, campaignID, url.Values{"fields": {fields}})
	if err != nil {
		return nil, err
	}
	return decodeObject(body)
}

// GetAdAccount fetches configured ad account details.
func (c *Client) GetAdAccount() (map[string]string, error) {
	body, err := c.request(http.MethodGet, c.ActID(), url.Values{"fields": {"id,name,account_status,currency,timezone_name,amount_spent,balance"}})
	if err != nil {
		return nil, err
	}
	return decodeObject(body)
}

// ListCampaigns lists campaigns in the configured ad account.
func (c *Client) ListCampaigns(limit int) ([]map[string]string, error) {
	return c.listCollection(c.ActID()+"/campaigns", "id,name,status,effective_status,objective,created_time,updated_time", limit)
}

// ListAdSets lists ad sets in the configured ad account.
func (c *Client) ListAdSets(limit int) ([]map[string]string, error) {
	return c.listCollection(c.ActID()+"/adsets", "id,name,status,effective_status,daily_budget,campaign_id", limit)
}

// ListAds lists ads in the configured ad account.
func (c *Client) ListAds(limit int) ([]map[string]string, error) {
	return c.listCollection(c.ActID()+"/ads", "id,name,status,effective_status,adset_id,campaign_id", limit)
}

func (c *Client) listCollection(endpoint, fields string, limit int) ([]map[string]string, error) {
	body, err := c.request(http.MethodGet, endpoint, url.Values{
		"fields": {fields},
		"limit":  {fmt.Sprintf("%d", limit)},
	})
	if err != nil {
		return nil, err
	}
	return listData(body)
}

// GetAdSets gets ad sets for a campaign.
func (c *Client) GetAdSets(campaignID, fields string) ([]map[string]string, error) {
	body, err := c.request(http.MethodGet, campaignID+"/adsets", url.Values{"fields": {fields}})
	if err != nil {
		return nil, err
	}
	return listData(body)
}

// GetAds gets ads for a campaign.
func (c *Client) GetAds(campaignID, fields string) ([]map[string]string, error) {
	body, err := c.request(http.MethodGet, campaignID+"/ads", url.Values{"fields": {fields}})
	if err != nil {
		return nil, err
	}
	return listData(body)
}

// UpdateStatus updates the status of a campaign, ad set, or ad.
func (c *Client) UpdateStatus(objectID, status string) error {
	_, err := c.request(http.MethodPost, objectID, url.Values{"status": {status}})
	return err
}

// GetInsightsParams configures an insights request.
type GetInsightsParams struct {
	ObjectID   string
	Level      string
	DatePreset string
	Limit      int
}

// GetInsights gets insights for an account, campaign, ad set, or ad.
func (c *Client) GetInsights(p GetInsightsParams) ([]map[string]string, error) {
	params := url.Values{
		"fields":      {"impressions,clicks,spend,cpc,cpm,ctr,actions"},
		"date_preset": {p.DatePreset},
		"limit":       {fmt.Sprintf("%d", p.Limit)},
	}
	if p.Level != "" {
		params.Set("level", p.Level)
	}
	body, err := c.request(http.MethodGet, p.ObjectID+"/insights", params)
	if err != nil {
		return nil, err
	}
	return listData(body)
}

// UpdateDailyBudget updates the daily budget for a campaign or ad set.
func (c *Client) UpdateDailyBudget(objectID string, dailyBudgetCents int) error {
	_, err := c.request(http.MethodPost, objectID, url.Values{"daily_budget": {fmt.Sprintf("%d", dailyBudgetCents)}})
	return err
}

// DeleteCampaign deletes a campaign (sets status to DELETED).
func (c *Client) DeleteCampaign(campaignID string) error {
	return c.UpdateStatus(campaignID, "DELETED")
}

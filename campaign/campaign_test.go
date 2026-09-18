package campaign

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rizbud/meta-ads-cli/api"
	"github.com/rizbud/meta-ads-cli/config"
)

type fakeAPI struct {
	uploadImageCalls    []api.UploadImageParams
	createCampaignCalls []api.CreateCampaignParams
	createAdSetCalls    []api.CreateAdSetParams
	createCreativeCalls []api.CreateAdCreativeParams
	createAdCalls       []api.CreateAdParams

	uploadErr   error
	campaignErr error
	adSetErr    error
	creativeErr error
	adErr       error

	imageHashes []string
	campaignIDs []string
	adSetIDs    []string
	creativeIDs []string
	adIDs       []string
}

func (f *fakeAPI) UploadImage(p api.UploadImageParams) (string, error) {
	f.uploadImageCalls = append(f.uploadImageCalls, p)
	if f.uploadErr != nil {
		return "", f.uploadErr
	}
	h := "hash_1"
	if len(f.imageHashes) > 0 {
		h = f.imageHashes[0]
		f.imageHashes = f.imageHashes[1:]
	}
	return h, nil
}

func (f *fakeAPI) CreateCampaign(p api.CreateCampaignParams) (string, error) {
	f.createCampaignCalls = append(f.createCampaignCalls, p)
	if f.campaignErr != nil {
		return "", f.campaignErr
	}
	id := "camp_1"
	if len(f.campaignIDs) > 0 {
		id = f.campaignIDs[0]
		f.campaignIDs = f.campaignIDs[1:]
	}
	return id, nil
}

func (f *fakeAPI) CreateAdSet(p api.CreateAdSetParams) (string, error) {
	f.createAdSetCalls = append(f.createAdSetCalls, p)
	if f.adSetErr != nil {
		return "", f.adSetErr
	}
	id := "set_1"
	if len(f.adSetIDs) > 0 {
		id = f.adSetIDs[0]
		f.adSetIDs = f.adSetIDs[1:]
	}
	return id, nil
}

func (f *fakeAPI) CreateAdCreative(p api.CreateAdCreativeParams) (string, error) {
	f.createCreativeCalls = append(f.createCreativeCalls, p)
	if f.creativeErr != nil {
		return "", f.creativeErr
	}
	id := "cr_1"
	if len(f.creativeIDs) > 0 {
		id = f.creativeIDs[0]
		f.creativeIDs = f.creativeIDs[1:]
	}
	return id, nil
}

func (f *fakeAPI) CreateAd(p api.CreateAdParams) (string, error) {
	f.createAdCalls = append(f.createAdCalls, p)
	if f.adErr != nil {
		return "", f.adErr
	}
	id := "ad_1"
	if len(f.adIDs) > 0 {
		id = f.adIDs[0]
		f.adIDs = f.adIDs[1:]
	}
	return id, nil
}

func sampleConfig(t *testing.T) *config.Config {
	t.Helper()
	img1 := filepath.Join(t.TempDir(), "ad-one.png")
	img2 := filepath.Join(t.TempDir(), "ad-two.png")
	for _, p := range []string{img1, img2} {
		if err := os.WriteFile(p, []byte("png"), 0o644); err != nil {
			t.Fatalf("write image: %v", err)
		}
	}
	return &config.Config{
		Campaign: &config.CampaignConfig{
			Name:      "My Campaign",
			Objective: "OUTCOME_TRAFFIC",
			Status:    "PAUSED",
		},
		AdSet: &config.AdSetConfig{
			Name:             "My Ad Set",
			DailyBudget:      1000,
			OptimizationGoal: "LINK_CLICKS",
			Targeting: config.TargetingConfig{
				Countries: []string{"US"},
			},
		},
		Ads: []config.AdConfig{
			{
				Name:        "Ad One",
				Image:       img1,
				PrimaryText: "  Copy one  ",
				Headline:    "Headline One",
				Description: "Desc One",
				Link:        "https://example.com/1",
				CTA:         "LEARN_MORE",
			},
			{
				Name:        "Ad Two",
				Image:       img2,
				PrimaryText: "Copy two",
				Headline:    "Headline Two",
				Link:        "https://example.com/2",
			},
		},
	}
}

func TestCreateFullCampaignHappyPath(t *testing.T) {
	f := &fakeAPI{}
	res, err := CreateFullCampaign(f, sampleConfig(t))
	if err != nil {
		t.Fatalf("CreateFullCampaign: %v", err)
	}
	if res.CampaignID == "" || res.AdSetID == "" {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Creatives) != 2 || len(res.Ads) != 2 {
		t.Fatalf("creatives=%v ads=%v", res.Creatives, res.Ads)
	}
	if len(f.uploadImageCalls) != 2 {
		t.Fatalf("upload calls = %d, want 2", len(f.uploadImageCalls))
	}
	if f.createCampaignCalls[0].Name != "My Campaign" {
		t.Errorf("campaign name = %q", f.createCampaignCalls[0].Name)
	}
	if f.createCampaignCalls[0].Objective != "OUTCOME_TRAFFIC" || f.createCampaignCalls[0].Status != "PAUSED" {
		t.Errorf("campaign params = %+v", f.createCampaignCalls[0])
	}
	if f.createAdSetCalls[0].DailyBudget != 1000 {
		t.Errorf("daily budget = %d", f.createAdSetCalls[0].DailyBudget)
	}
	if f.createAdSetCalls[0].CampaignID != res.CampaignID {
		t.Errorf("ad set campaign id = %q, want %q", f.createAdSetCalls[0].CampaignID, res.CampaignID)
	}
	if len(res.Ads) != 2 {
		t.Errorf("ads = %v, want 2", res.Ads)
	}
}

func TestCreateFullCampaignRealIDsPropagate(t *testing.T) {
	f := &fakeAPI{
		campaignIDs: []string{"real_camp"},
		adSetIDs:    []string{"real_set"},
		creativeIDs: []string{"real_cr1", "real_cr2"},
		adIDs:       []string{"real_ad1", "real_ad2"},
		imageHashes: []string{"real_hash_1", "real_hash_2"},
	}
	res, err := CreateFullCampaign(f, sampleConfig(t))
	if err != nil {
		t.Fatalf("CreateFullCampaign: %v", err)
	}
	if res.CampaignID != "real_camp" || res.AdSetID != "real_set" {
		t.Errorf("result ids = %+v", res)
	}
	if res.Creatives[0] != "real_cr1" || res.Creatives[1] != "real_cr2" {
		t.Errorf("creatives = %v", res.Creatives)
	}
	if res.Ads[0] != "real_ad1" || res.Ads[1] != "real_ad2" {
		t.Errorf("ads = %v", res.Ads)
	}
	for i, call := range f.createAdCalls {
		if call.AdSetID != "real_set" {
			t.Errorf("ad %d set id = %q", i, call.AdSetID)
		}
	}
	if f.createCreativeCalls[0].ImageHash != "real_hash_1" || f.createCreativeCalls[1].ImageHash != "real_hash_2" {
		t.Errorf("image hashes = %v", f.createCreativeCalls)
	}
}

func TestCreateFullCampaignPassesStatusToAllLevels(t *testing.T) {
	cfg := sampleConfig(t)
	cfg.Campaign.Status = "ACTIVE"
	cfg.AdSet.Status = ""
	f := &fakeAPI{}
	if _, err := CreateFullCampaign(f, cfg); err != nil {
		t.Fatalf("CreateFullCampaign: %v", err)
	}
	if f.createCampaignCalls[0].Status != "ACTIVE" {
		t.Errorf("campaign status = %q", f.createCampaignCalls[0].Status)
	}
	if f.createAdSetCalls[0].Status != "ACTIVE" {
		t.Errorf("ad set status = %q", f.createAdSetCalls[0].Status)
	}
	for _, call := range f.createAdCalls {
		if call.Status != "ACTIVE" {
			t.Errorf("ad status = %q", call.Status)
		}
	}
}

func TestCreateFullCampaignStripsPrimaryText(t *testing.T) {
	f := &fakeAPI{}
	if _, err := CreateFullCampaign(f, sampleConfig(t)); err != nil {
		t.Fatalf("CreateFullCampaign: %v", err)
	}
	if f.createCreativeCalls[0].PrimaryText != "Copy one" {
		t.Errorf("primary text = %q, want stripped", f.createCreativeCalls[0].PrimaryText)
	}
}

func TestCreateFullCampaignCreativeNameSuffix(t *testing.T) {
	f := &fakeAPI{}
	if _, err := CreateFullCampaign(f, sampleConfig(t)); err != nil {
		t.Fatalf("CreateFullCampaign: %v", err)
	}
	if f.createCreativeCalls[0].Name != "Ad One - Creative" {
		t.Errorf("creative name = %q", f.createCreativeCalls[0].Name)
	}
}

func TestCreateFullCampaignAdDefaults(t *testing.T) {
	f := &fakeAPI{}
	if _, err := CreateFullCampaign(f, sampleConfig(t)); err != nil {
		t.Fatalf("CreateFullCampaign: %v", err)
	}
	second := f.createCreativeCalls[1]
	if second.CTA != "LEARN_MORE" {
		t.Errorf("default cta = %q", second.CTA)
	}
	if second.Description != "" {
		t.Errorf("default description = %q", second.Description)
	}
	if f.createAdSetCalls[0].BillingEvent != "IMPRESSIONS" {
		t.Errorf("default billing event = %q", f.createAdSetCalls[0].BillingEvent)
	}
	if f.createAdSetCalls[0].BidStrategy != "LOWEST_COST_WITHOUT_CAP" {
		t.Errorf("default bid strategy = %q", f.createAdSetCalls[0].BidStrategy)
	}
}

func TestCreateFullCampaignUploadError(t *testing.T) {
	f := &fakeAPI{uploadErr: errors.New("upload failed")}
	res, err := CreateFullCampaign(f, sampleConfig(t))
	if err == nil {
		t.Fatal("expected error")
	}
	if res.CampaignID != "" {
		t.Errorf("partial campaign id = %q, want empty", res.CampaignID)
	}
	if len(f.createCampaignCalls) != 0 {
		t.Errorf("campaign should not be created")
	}
}

func TestCreateFullCampaignPartialProgress(t *testing.T) {
	f := &fakeAPI{adSetErr: errors.New("adset failed")}
	res, err := CreateFullCampaign(f, sampleConfig(t))
	if err == nil {
		t.Fatal("expected error")
	}
	if res.CampaignID == "" {
		t.Errorf("campaign should be created before adset failure")
	}
	if res.AdSetID != "" {
		t.Errorf("ad set should not exist")
	}
}

func TestCreateFullCampaignStopsAfterCreativeFailure(t *testing.T) {
	f := &fakeAPI{creativeErr: errors.New("creative failed")}
	res, err := CreateFullCampaign(f, sampleConfig(t))
	if err == nil {
		t.Fatal("expected error")
	}
	if len(res.Creatives) != 0 {
		t.Errorf("creatives = %v, want empty", res.Creatives)
	}
	if len(f.createAdCalls) != 0 {
		t.Errorf("no ads should be created after creative failure")
	}
}

type statusAPI struct {
	campaign    map[string]string
	adSets      []map[string]string
	ads         []map[string]string
	campaignErr error
}

func (s *statusAPI) GetCampaign(id, fields string) (map[string]string, error) {
	if s.campaignErr != nil {
		return nil, s.campaignErr
	}
	cp := map[string]string{}
	for k, v := range s.campaign {
		cp[k] = v
	}
	cp["id"] = id
	return cp, nil
}

func (s *statusAPI) GetAdSets(id, fields string) ([]map[string]string, error) {
	return s.adSets, nil
}

func (s *statusAPI) GetAds(id, fields string) ([]map[string]string, error) {
	return s.ads, nil
}

func TestPrintCampaignStatus(t *testing.T) {
	api := &statusAPI{
		campaign: map[string]string{"name": "Big Launch", "status": "PAUSED", "objective": "OUTCOME_TRAFFIC"},
		adSets: []map[string]string{
			{"name": "Broad", "status": "ACTIVE", "daily_budget": "1000"},
		},
		ads: []map[string]string{
			{"name": "Feed Ad", "status": "ACTIVE", "effective_status": "ACTIVE"},
		},
	}
	var out strings.Builder
	if err := PrintCampaignStatus(&out, api, "camp_123", "USD"); err != nil {
		t.Fatalf("PrintCampaignStatus: %v", err)
	}
	s := out.String()
	for _, want := range []string{
		"Campaign: Big Launch",
		"ID: camp_123",
		"Status: PAUSED",
		"Objective: OUTCOME_TRAFFIC",
		"Broad: ACTIVE (USD 10.00/day)",
		"Feed Ad: ACTIVE",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}
}

func TestPrintCampaignStatusUsesEffectiveStatus(t *testing.T) {
	api := &statusAPI{
		campaign: map[string]string{"name": "C", "status": "ACTIVE"},
		adSets:   []map[string]string{},
		ads: []map[string]string{
			{"name": "Ad", "status": "ACTIVE", "effective_status": "ARCHIVED"},
		},
	}
	var out strings.Builder
	if err := PrintCampaignStatus(&out, api, "1", "USD"); err != nil {
		t.Fatalf("PrintCampaignStatus: %v", err)
	}
	if !strings.Contains(out.String(), "Ad: ARCHIVED") {
		t.Errorf("output = %q", out.String())
	}
}

func TestPrintCampaignStatusError(t *testing.T) {
	api := &statusAPI{campaignErr: errors.New("api down")}
	var out strings.Builder
	if err := PrintCampaignStatus(&out, api, "1", "USD"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPrintCampaignStatusZeroBudgetFormat(t *testing.T) {
	api := &statusAPI{
		campaign: map[string]string{"name": "C", "status": "PAUSED"},
		adSets:   []map[string]string{{"name": "Set", "status": "PAUSED"}},
		ads:      []map[string]string{},
	}
	var out strings.Builder
	if err := PrintCampaignStatus(&out, api, "1", "USD"); err != nil {
		t.Fatalf("PrintCampaignStatus: %v", err)
	}
	if !strings.Contains(out.String(), "Set: PAUSED (USD 0.00/day)") {
		t.Errorf("output = %q", out.String())
	}
}

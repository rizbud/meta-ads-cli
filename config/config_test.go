package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func mustLoad(t *testing.T, content string) *Config {
	t.Helper()
	path := writeTemp(t, "campaign.yaml", content)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

func TestLoadConfigValid(t *testing.T) {
	path := writeTemp(t, "campaign.yaml", `
campaign:
  name: "My Campaign"
  objective: OUTCOME_TRAFFIC
ad_set:
  name: "My Ad Set"
  daily_budget: 1000
  targeting:
    countries: ["US"]
ads:
  - name: "My Ad"
    image: ./images/ad.png
    primary_text: "Copy"
    headline: "Headline"
    link: "https://example.com"
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Campaign.Name != "My Campaign" {
		t.Errorf("campaign name = %q, want %q", cfg.Campaign.Name, "My Campaign")
	}
	if cfg.Campaign.Objective != "OUTCOME_TRAFFIC" {
		t.Errorf("objective = %q, want OUTCOME_TRAFFIC", cfg.Campaign.Objective)
	}
	if cfg.AdSet.DailyBudget != 1000 {
		t.Errorf("daily_budget = %d, want 1000", cfg.AdSet.DailyBudget)
	}
	if len(cfg.Ads) != 1 {
		t.Fatalf("ads = %d, want 1", len(cfg.Ads))
	}
	if cfg.Ads[0].Name != "My Ad" {
		t.Errorf("ad name = %q, want %q", cfg.Ads[0].Name, "My Ad")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want mention of not found", err)
	}
}

func TestLoadConfigEmptyFile(t *testing.T) {
	path := writeTemp(t, "campaign.yaml", "")
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error for empty config")
	} else if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error = %q, want 'empty'", err)
	}
}

func TestLoadConfigResolvesRelativeImagePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.yaml")
	content := `
ads:
  - name: "Ad"
    image: ./images/ad.png
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "images", "ad.png")
	if cfg.Ads[0].Image != want {
		t.Errorf("image = %q, want %q", cfg.Ads[0].Image, want)
	}
}

func TestLoadConfigKeepsAbsoluteImagePath(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "images", "ad.png")
	path := writeTemp(t, "campaign.yaml", `
ads:
  - name: "Ad"
    image: `+abs+"\n")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ads[0].Image != abs {
		t.Errorf("image = %q, want %q", cfg.Ads[0].Image, abs)
	}
}

func validConfig(t *testing.T) *Config {
	return &Config{
		Campaign: &CampaignConfig{
			Name:      "My Campaign",
			Objective: "OUTCOME_TRAFFIC",
			Status:    "PAUSED",
		},
		AdSet: &AdSetConfig{
			Name:             "My Ad Set",
			DailyBudget:      1000,
			OptimizationGoal: "LINK_CLICKS",
			Targeting: TargetingConfig{
				Countries: []string{"US"},
			},
		},
		Ads: []AdConfig{
			{
				Name:        "My Ad",
				Image:       writeTempFound(t),
				PrimaryText: "Copy",
				Headline:    "Headline",
				Link:        "https://example.com",
				CTA:         "LEARN_MORE",
			},
		},
	}
}

func writeTempFound(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ad.png")
	if err := os.WriteFile(path, []byte("png"), 0o644); err != nil {
		t.Fatalf("write image file: %v", err)
	}
	return path
}

func TestValidateConfigValid(t *testing.T) {
	if err := ValidateConfig(validConfig(t)); err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
}

func TestValidateConfigMissingCampaign(t *testing.T) {
	cfg := validConfig(t)
	cfg.Campaign = &CampaignConfig{}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "campaign.name is required") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigMissingCampaignSection(t *testing.T) {
	cfg := Config{Ads: validConfig(t).Ads, AdSet: validConfig(t).AdSet}
	err := ValidateConfig(&cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Missing 'campaign' section") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigInvalidObjective(t *testing.T) {
	cfg := validConfig(t)
	cfg.Campaign.Objective = "OUTCOME_NOPE"
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "'OUTCOME_NOPE' is not valid") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigInvalidStatus(t *testing.T) {
	cfg := validConfig(t)
	cfg.Campaign.Status = "INVALID"
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "PAUSED or ACTIVE") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigMissingAdSet(t *testing.T) {
	cfg := Config{Campaign: validConfig(t).Campaign, Ads: validConfig(t).Ads}
	err := ValidateConfig(&cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Missing 'ad_set' section") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigMissingAdSetFields(t *testing.T) {
	cfg := validConfig(t)
	cfg.AdSet = &AdSetConfig{Name: "", DailyBudget: 0}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ad_set.name is required") {
		t.Errorf("error = %q", err)
	}
	if !strings.Contains(err.Error(), "daily_budget is required") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigInvalidOptimizationGoal(t *testing.T) {
	cfg := validConfig(t)
	cfg.AdSet.OptimizationGoal = "NOPE"
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "is not valid") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigMissingTargetingCountries(t *testing.T) {
	cfg := validConfig(t)
	cfg.AdSet.Targeting.Countries = nil
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "targeting.countries is required") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigMissingAds(t *testing.T) {
	cfg := Config{Campaign: validConfig(t).Campaign, AdSet: validConfig(t).AdSet}
	err := ValidateConfig(&cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Missing 'ads' section") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigMissingAdFields(t *testing.T) {
	cfg := validConfig(t)
	cfg.Ads = []AdConfig{
		{Name: "", Image: writeTempFound(t), PrimaryText: "", Headline: "", Link: ""},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"ads[0].name is required", "ads[0].primary_text is required", "ads[0].headline is required", "ads[0].link is required"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in: %v", want, err)
		}
	}
}

func TestValidateConfigMissingImageFile(t *testing.T) {
	cfg := validConfig(t)
	cfg.Ads[0].Image = "/nonexistent/ad.png"
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigMissingImageOrVideo(t *testing.T) {
	cfg := validConfig(t)
	cfg.Ads[0].Image = ""
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ads[0].image or ads[0].video is required") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigBothImageAndVideo(t *testing.T) {
	cfg := validConfig(t)
	cfg.Ads[0].Video = writeTempFound(t)
	cfg.Ads[0].Thumbnail = writeTempFound(t)
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cannot set both image and video") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigVideoRequiresThumbnail(t *testing.T) {
	cfg := validConfig(t)
	cfg.Ads[0].Image = ""
	cfg.Ads[0].Video = writeTempFound(t)
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ads[0].thumbnail is required for video ads") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigVideoAdValid(t *testing.T) {
	cfg := validConfig(t)
	cfg.Ads[0].Image = ""
	cfg.Ads[0].Video = writeTempFound(t)
	cfg.Ads[0].Thumbnail = writeTempFound(t)
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
	if !cfg.Ads[0].IsVideo() {
		t.Error("IsVideo() = false, want true")
	}
}

func TestLoadConfigResolvesRelativeVideoAndThumbnailPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.yaml")
	content := `
ads:
  - name: "Ad"
    video: ./videos/reel.mp4
    thumbnail: ./images/cover.png
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "videos", "reel.mp4"); cfg.Ads[0].Video != want {
		t.Errorf("video = %q, want %q", cfg.Ads[0].Video, want)
	}
	if want := filepath.Join(dir, "images", "cover.png"); cfg.Ads[0].Thumbnail != want {
		t.Errorf("thumbnail = %q, want %q", cfg.Ads[0].Thumbnail, want)
	}
}

func TestValidateConfigInvalidCTA(t *testing.T) {
	cfg := validConfig(t)
	cfg.Ads[0].CTA = "NOPE"
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "is not valid") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateConfigDefaults(t *testing.T) {
	cfg := &Config{
		Campaign: &CampaignConfig{Name: "C"},
		AdSet: &AdSetConfig{
			Name:        "A",
			DailyBudget: 1000,
			Targeting:   TargetingConfig{Countries: []string{"US"}},
		},
		Ads: []AdConfig{{Name: "ad", Image: writeTempFound(t), PrimaryText: "x", Headline: "h", Link: "https://x"}},
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig with defaults: %v", err)
	}
	if cfg.Campaign.Objective != "OUTCOME_TRAFFIC" {
		t.Errorf("default objective = %q", cfg.Campaign.Objective)
	}
	if cfg.Ads[0].CTA != "LEARN_MORE" {
		t.Errorf("default cta = %q", cfg.Ads[0].CTA)
	}
}

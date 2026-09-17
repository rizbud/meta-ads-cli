package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	validObjectives = []string{
		"OUTCOME_TRAFFIC",
		"OUTCOME_AWARENESS",
		"OUTCOME_ENGAGEMENT",
		"OUTCOME_LEADS",
		"OUTCOME_SALES",
		"OUTCOME_APP_PROMOTION",
	}

	validOptimizationGoals = []string{
		"LINK_CLICKS",
		"IMPRESSIONS",
		"REACH",
		"LANDING_PAGE_VIEWS",
		"APP_INSTALLS",
		"OFFSITE_CONVERSIONS",
		"LEAD_GENERATION",
	}

	validCTAs = []string{
		"LEARN_MORE",
		"SIGN_UP",
		"DOWNLOAD",
		"SHOP_NOW",
		"BOOK_NOW",
		"GET_OFFER",
		"SUBSCRIBE",
		"CONTACT_US",
		"APPLY_NOW",
		"WATCH_MORE",
		"INSTALL_MOBILE_APP",
	}

	validStatuses = []string{"PAUSED", "ACTIVE"}
)

// Config is the parsed campaign YAML config.
type Config struct {
	Campaign *CampaignConfig `yaml:"campaign"`
	AdSet    *AdSetConfig    `yaml:"ad_set"`
	Ads      []AdConfig      `yaml:"ads"`
}

// CampaignConfig holds the campaign section of a config file.
type CampaignConfig struct {
	Name                string   `yaml:"name"`
	Objective           string   `yaml:"objective"`
	Status              string   `yaml:"status"`
	SpecialAdCategories []string `yaml:"special_ad_categories"`
}

// AdSetConfig holds the ad_set section of a config file.
type AdSetConfig struct {
	Name             string          `yaml:"name"`
	DailyBudget      int             `yaml:"daily_budget"`
	OptimizationGoal string          `yaml:"optimization_goal"`
	BillingEvent     string          `yaml:"billing_event"`
	BidStrategy      string          `yaml:"bid_strategy"`
	Status           string          `yaml:"status"`
	Targeting        TargetingConfig `yaml:"targeting"`
}

// TargetingConfig holds the targeting spec of an ad set.
type TargetingConfig struct {
	AgeMin             int              `yaml:"age_min"`
	AgeMax             int              `yaml:"age_max"`
	Genders            []int            `yaml:"genders"`
	Countries          []string         `yaml:"countries"`
	Interests          []InterestConfig `yaml:"interests"`
	Platforms          []string         `yaml:"platforms"`
	FacebookPositions  []string         `yaml:"facebook_positions"`
	InstagramPositions []string         `yaml:"instagram_positions"`
}

// InterestConfig is a targeting interest.
type InterestConfig struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

// AdConfig holds an entry in the ads section of a config file.
type AdConfig struct {
	Name        string `yaml:"name"`
	Image       string `yaml:"image"`
	PrimaryText string `yaml:"primary_text"`
	Headline    string `yaml:"headline"`
	Description string `yaml:"description"`
	CTA         string `yaml:"cta"`
	Link        string `yaml:"link"`
}

// LoadConfig reads and parses a campaign YAML config file. Image paths are
// resolved relative to the YAML file's directory.
func LoadConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("config file not found: %s", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, fmt.Errorf("config file is empty")
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	dir := filepath.Dir(path)
	for i := range cfg.Ads {
		if cfg.Ads[i].Image != "" && !filepath.IsAbs(cfg.Ads[i].Image) {
			cfg.Ads[i].Image = filepath.Join(dir, cfg.Ads[i].Image)
		}
	}
	return cfg, nil
}

// ValidateConfig verifies a campaign config, returning an error with all
// problems found. It applies defaults for optional fields in place.
func ValidateConfig(c *Config) error {
	var errs []string

	if c.Campaign == nil {
		errs = append(errs, "Missing 'campaign' section")
	} else {
		if c.Campaign.Name == "" {
			errs = append(errs, "campaign.name is required")
		}
		if c.Campaign.Objective == "" {
			c.Campaign.Objective = "OUTCOME_TRAFFIC"
		}
		if !contains(validObjectives, c.Campaign.Objective) {
			errs = append(errs, fmt.Sprintf("campaign.objective '%s' is not valid. Options: %s", c.Campaign.Objective, strings.Join(validObjectives, ", ")))
		}
		if c.Campaign.Status == "" {
			c.Campaign.Status = "PAUSED"
		}
		if !contains(validStatuses, c.Campaign.Status) {
			errs = append(errs, "campaign.status must be PAUSED or ACTIVE")
		}
	}

	if c.AdSet == nil {
		errs = append(errs, "Missing 'ad_set' section")
	} else {
		if c.AdSet.Name == "" {
			errs = append(errs, "ad_set.name is required")
		}
		if c.AdSet.DailyBudget == 0 {
			errs = append(errs, "ad_set.daily_budget is required (in cents, e.g. 1000 = $10/day)")
		}
		if c.AdSet.OptimizationGoal == "" {
			c.AdSet.OptimizationGoal = "LINK_CLICKS"
		}
		if !contains(validOptimizationGoals, c.AdSet.OptimizationGoal) {
			errs = append(errs, fmt.Sprintf("ad_set.optimization_goal '%s' is not valid. Options: %s", c.AdSet.OptimizationGoal, strings.Join(validOptimizationGoals, ", ")))
		}
		if len(c.AdSet.Targeting.Countries) == 0 {
			errs = append(errs, "ad_set.targeting.countries is required (e.g. ['US', 'CA'])")
		}
	}

	if len(c.Ads) == 0 {
		errs = append(errs, "Missing 'ads' section (need at least one ad)")
	}
	for i, ad := range c.Ads {
		prefix := fmt.Sprintf("ads[%d]", i)
		if ad.Name == "" {
			errs = append(errs, prefix+".name is required")
		}
		if ad.Image == "" {
			errs = append(errs, prefix+".image is required")
		} else if _, err := os.Stat(ad.Image); err != nil {
			errs = append(errs, fmt.Sprintf("%s.image not found: %s", prefix, ad.Image))
		}
		if ad.PrimaryText == "" {
			errs = append(errs, prefix+".primary_text is required")
		}
		if ad.Headline == "" {
			errs = append(errs, prefix+".headline is required")
		}
		if ad.Link == "" {
			errs = append(errs, prefix+".link is required")
		}
		cta := ad.CTA
		if cta == "" {
			cta = "LEARN_MORE"
			c.Ads[i].CTA = cta
		}
		if !contains(validCTAs, cta) {
			errs = append(errs, fmt.Sprintf("%s.cta '%s' is not valid. Options: %s", prefix, cta, strings.Join(validCTAs, ", ")))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

// IsEmpty reports whether no sections were parsed from YAML.
func (c *Config) IsEmpty() bool {
	return c.Campaign == nil && c.AdSet == nil && len(c.Ads) == 0
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

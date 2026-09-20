package campaign

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rizbud/meta-ads-cli/api"
	"github.com/rizbud/meta-ads-cli/config"
	"github.com/rizbud/meta-ads-cli/money"
)

// Creator is the subset of the Meta API client the campaign orchestrator needs.
type Creator interface {
	UploadImage(p api.UploadImageParams) (string, error)
	UploadVideo(p api.UploadVideoParams) (string, error)
	CreateCampaign(p api.CreateCampaignParams) (string, error)
	CreateAdSet(p api.CreateAdSetParams) (string, error)
	CreateAdCreative(p api.CreateAdCreativeParams) (string, error)
	CreateAd(p api.CreateAdParams) (string, error)
}

// Result holds the object IDs created for a full campaign.
type Result struct {
	CampaignID string
	AdSetID    string
	Creatives  []string
	Ads        []string
}

// CreateFullCampaign creates a complete campaign from config: campaign,
// ad set, creatives, and ads. On failure it returns the partial result so
// callers can record what was already created.
func CreateFullCampaign(client Creator, cfg *config.Config) (Result, error) {
	var res Result

	status := cfg.Campaign.Status
	if status == "" {
		status = "PAUSED"
	}

	imageHashes := map[string]string{}
	videoIDs := map[string]string{}
	for _, ad := range cfg.Ads {
		imageHash, videoID, err := uploadAdMedia(client, ad)
		if err != nil {
			return res, err
		}
		imageHashes[ad.Name] = imageHash
		videoIDs[ad.Name] = videoID
	}

	campaignID, err := client.CreateCampaign(api.CreateCampaignParams{
		Name:                cfg.Campaign.Name,
		Objective:           cfg.Campaign.Objective,
		Status:              status,
		SpecialAdCategories: cfg.Campaign.SpecialAdCategories,
	})
	if err != nil {
		return res, err
	}
	res.CampaignID = campaignID

	billingEvent := cfg.AdSet.BillingEvent
	if billingEvent == "" {
		billingEvent = "IMPRESSIONS"
	}
	bidStrategy := cfg.AdSet.BidStrategy
	if bidStrategy == "" {
		bidStrategy = "LOWEST_COST_WITHOUT_CAP"
	}
	adSetID, err := client.CreateAdSet(api.CreateAdSetParams{
		Name:             cfg.AdSet.Name,
		CampaignID:       campaignID,
		DailyBudget:      cfg.AdSet.DailyBudget,
		OptimizationGoal: cfg.AdSet.OptimizationGoal,
		BillingEvent:     billingEvent,
		BidStrategy:      bidStrategy,
		Status:           status,
		Targeting: api.Targeting{
			AgeMin:             cfg.AdSet.Targeting.AgeMin,
			AgeMax:             cfg.AdSet.Targeting.AgeMax,
			Genders:            cfg.AdSet.Targeting.Genders,
			Countries:          cfg.AdSet.Targeting.Countries,
			Interests:          mapInterests(cfg.AdSet.Targeting.Interests),
			Platforms:          cfg.AdSet.Targeting.Platforms,
			FacebookPositions:  cfg.AdSet.Targeting.FacebookPositions,
			InstagramPositions: cfg.AdSet.Targeting.InstagramPositions,
		},
	})
	if err != nil {
		return res, err
	}
	res.AdSetID = adSetID

	for _, ad := range cfg.Ads {
		creativeID, adID, err := createAdCreativeAndAd(client, ad, adSetID, imageHashes[ad.Name], videoIDs[ad.Name], status)
		if err != nil {
			return res, err
		}
		res.Creatives = append(res.Creatives, creativeID)
		res.Ads = append(res.Ads, adID)
	}

	return res, nil
}

// AddAdResult holds the object IDs created for a single ad.
type AddAdResult struct {
	CreativeID string
	AdID       string
}

// AddAdToAdSet uploads an ad's media (image or video) and attaches it as a
// new ad to an already-existing ad set. Status defaults to PAUSED when
// empty.
func AddAdToAdSet(client Creator, ad config.AdConfig, adSetID, status string) (AddAdResult, error) {
	if status == "" {
		status = "PAUSED"
	}
	imageHash, videoID, err := uploadAdMedia(client, ad)
	if err != nil {
		return AddAdResult{}, err
	}
	creativeID, adID, err := createAdCreativeAndAd(client, ad, adSetID, imageHash, videoID, status)
	if err != nil {
		return AddAdResult{}, err
	}
	return AddAdResult{CreativeID: creativeID, AdID: adID}, nil
}

// uploadAdMedia uploads an ad's image or video (with its cover thumbnail)
// and returns the resulting image hash and/or video ID.
func uploadAdMedia(client Creator, ad config.AdConfig) (imageHash, videoID string, err error) {
	if ad.IsVideo() {
		videoData, err := os.ReadFile(ad.Video)
		if err != nil {
			return "", "", err
		}
		videoID, err = client.UploadVideo(api.UploadVideoParams{
			FilePath: ad.Video,
			FileName: filepath.Base(ad.Video),
			Data:     videoData,
		})
		if err != nil {
			return "", "", err
		}

		thumbData, err := os.ReadFile(ad.Thumbnail)
		if err != nil {
			return "", "", err
		}
		imageHash, err = client.UploadImage(api.UploadImageParams{
			FilePath:    ad.Thumbnail,
			FileName:    filepath.Base(ad.Thumbnail),
			ContentType: "image/png",
			Data:        thumbData,
		})
		if err != nil {
			return "", "", err
		}
		return imageHash, videoID, nil
	}

	data, err := os.ReadFile(ad.Image)
	if err != nil {
		return "", "", err
	}
	imageHash, err = client.UploadImage(api.UploadImageParams{
		FilePath:    ad.Image,
		FileName:    filepath.Base(ad.Image),
		ContentType: "image/png",
		Data:        data,
	})
	if err != nil {
		return "", "", err
	}
	return imageHash, "", nil
}

func createAdCreativeAndAd(client Creator, ad config.AdConfig, adSetID, imageHash, videoID, status string) (creativeID, adID string, err error) {
	cta := ad.CTA
	if cta == "" {
		cta = "LEARN_MORE"
	}
	creativeID, err = client.CreateAdCreative(api.CreateAdCreativeParams{
		Name:        ad.Name + " - Creative",
		ImageHash:   imageHash,
		VideoID:     videoID,
		PrimaryText: strings.TrimSpace(ad.PrimaryText),
		Headline:    ad.Headline,
		Description: ad.Description,
		Link:        ad.Link,
		CTA:         cta,
	})
	if err != nil {
		return "", "", err
	}

	adID, err = client.CreateAd(api.CreateAdParams{
		Name:       ad.Name,
		AdSetID:    adSetID,
		CreativeID: creativeID,
		Status:     status,
	})
	if err != nil {
		return creativeID, "", err
	}
	return creativeID, adID, nil
}

func mapInterests(list []config.InterestConfig) []api.Interest {
	out := make([]api.Interest, 0, len(list))
	for _, in := range list {
		out = append(out, api.Interest{ID: in.ID, Name: in.Name})
	}
	return out
}

// StatusAPI is the subset of the Meta API client the status printer needs.
type StatusAPI interface {
	GetCampaign(campaignID, fields string) (map[string]string, error)
	GetAdSets(campaignID, fields string) ([]map[string]string, error)
	GetAds(campaignID, fields string) ([]map[string]string, error)
}

// PrintCampaignStatus fetches and prints a campaign summary with ad sets and
// ads to w.
func PrintCampaignStatus(w io.Writer, client StatusAPI, campaignID, currency string) error {
	campaign, err := client.GetCampaign(campaignID, "name,status,objective,daily_budget")
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "\nCampaign: %s\n", campaign["name"])
	fmt.Fprintf(w, "  ID: %s\n", campaign["id"])
	fmt.Fprintf(w, "  Status: %s\n", campaign["status"])
	if objective := campaign["objective"]; objective != "" {
		fmt.Fprintf(w, "  Objective: %s\n", objective)
	}

	adSets, err := client.GetAdSets(campaignID, "name,status,daily_budget")
	if err != nil {
		return err
	}
	if len(adSets) > 0 {
		fmt.Fprintln(w, "\n  Ad Sets:")
		for _, adSet := range adSets {
			budget := 0
			fmt.Sscanf(adSet["daily_budget"], "%d", &budget)
			fmt.Fprintf(w, "    %s: %s (%s/day)\n", adSet["name"], adSet["status"], money.Format(int64(budget), currency))
		}
	}

	ads, err := client.GetAds(campaignID, "name,status,effective_status")
	if err != nil {
		return err
	}
	if len(ads) > 0 {
		fmt.Fprintln(w, "\n  Ads:")
		for _, ad := range ads {
			effective := ad["effective_status"]
			if effective == "" {
				effective = ad["status"]
			}
			fmt.Fprintf(w, "    %s: %s\n", ad["name"], effective)
		}
	}
	return nil
}

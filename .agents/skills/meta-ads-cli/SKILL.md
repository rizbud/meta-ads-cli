---
name: meta-ads-cli
description: Use when the user wants to use meta-ads-cli to manage Meta (Facebook/Instagram) ad campaigns — triggers like "meta-ads", "campaign.yaml", "launch an ad", "dry-run", "budget", "insights", or "META_ACCESS_TOKEN". Covers the binary's commands, config schema, environment variables, and safety rails. Use ONLY for this tool; do not use for other ad tooling.
---

# meta-ads-cli usage

Run `meta-ads` (installed globally) or `./meta-ads` (repo root). It deploys a campaign defined in one YAML file (campaign → ad set → creatives → ads) and manages existing ads: listings, insights, budgets, status changes, and a local audit log. All output goes to stdout; exit codes are 0 on success, non-zero on failure.

## Credentials

Read from the environment or `.env` in the current directory. **All three are required** before any Meta call, even `--dry-run`:

- `META_ACCESS_TOKEN` (required)
- `META_AD_ACCOUNT_ID` — numbers only, no `act_` prefix (required)
- `META_PAGE_ID` (required)
- `META_API_VERSION` — default `v21.0`
- `META_CURRENCY` — currency code for offline budget display (`validate`, `--dry-run`); live commands read it from your account
- `META_ADS_MAX_DAILY_BUDGET_CENTS` — budget change guardrail
- `META_ADS_AUDIT_LOG_PATH` — default `~/.meta-ads-cli/audit.jsonl`

Setup: `cp .env.example .env`, then edit.

## Currency & daily_budget units

**CRITICAL**: How Meta interprets `daily_budget` in YAML and `budget <id> <amount>` depends on whether the currency is zero-decimal:

- **Zero-decimal currencies** (`IDR`, `JPY`, `KRW`, `VND`, `BIF`, `CLP`, `DJF`, `GNF`, `ISK`, `KMF`, `PYG`, `RWF`, `UGX`, `VUV`, `XAF`, `XOF`, `XPF`):
  - Amount is specified in **WHOLE CURRENCY UNITS** (1 unit = 1 Rupiah / Yen / Won).
  - **DO NOT multiply by 100!** For IDR 18,000/day, write `daily_budget: 18000` (or `meta-ads budget <id> 18000`).
  - *Warning*: Writing `1800000` for IDR sets the daily budget to **IDR 1,800,000 (1.8 juta)**!
- **Standard decimal currencies** (`USD`, `EUR`, `GBP`, `AUD`, `CAD`, `SGD`, etc.):
  - Amount is specified in **cents / minor units** (`1000 = $10.00/day`).

## Standard workflow

1. `validate` — no API calls:
   `meta-ads validate --config campaign.yaml`
2. `create --dry-run` — previews every `[DRY RUN] POST act_<id>/...` call, no network traffic (still writes an audit event):
   `meta-ads create --config campaign.yaml --dry-run`
3. `create --yes` — deploys for real (skips the confirmation prompt):
   `meta-ads create --config campaign.yaml --yes`

Everything is created as `PAUSED` by default so you can review before spending. `budget`, `upload-image`, `upload-video`, `add-ad`, and `bulk-status` also default to dry run — live calls need `--live` (plus `--yes`). `activate` and `delete` always confirm unless `--yes`.

## Config schema (campaign.yaml)

```yaml
campaign:
  name: "My Campaign"                # required
  objective: OUTCOME_TRAFFIC         # default OUTCOME_TRAFFIC
  status: PAUSED                     # PAUSED or ACTIVE
  special_ad_categories: []          # optional
ad_set:
  name: "My Ad Set"                  # required
  daily_budget: 1000                 # required; in cents for USD (1000 = $10/day), or whole units for zero-decimal currencies (18000 = IDR 18.000/day)
  optimization_goal: LINK_CLICKS     # default LINK_CLICKS
  targeting:
    countries: ["US"]                # required
    age_min: 18                      # default 18
    age_max: 65                      # default 65
    genders: [0]                     # default [0] (0 = all)
    interests: [{id, name}]          # optional
    platforms: ["facebook","instagram"]
    facebook_positions: ["feed"]
    instagram_positions: ["stream","story","reels"]
ads:
  # Image ad example
  - name: "My Image Ad"              # required
    image: ./images/ad.png           # required for image ads; relative paths resolve against YAML dir
    primary_text: "copy"             # required
    headline: "Headline"             # required
    link: "https://example.com"      # required
    description: ""                  # optional
    cta: LEARN_MORE                  # default LEARN_MORE

  # Video / Reels ad example (mutually exclusive with image)
  - name: "My Video Ad"              # required
    video: ./videos/reel.mp4         # required for video ads
    thumbnail: ./images/cover.png    # required for video ads (still cover image)
    primary_text: "copy"             # required
    headline: "Headline"             # required
    link: "https://example.com"      # required
    description: ""                  # optional
    cta: LEARN_MORE                  # default LEARN_MORE
```

Open objectives: `OUTCOME_TRAFFIC`, `OUTCOME_AWARENESS`, `OUTCOME_ENGAGEMENT`, `OUTCOME_LEADS`, `OUTCOME_SALES`, `OUTCOME_APP_PROMOTION`. CTAs: `LEARN_MORE`, `SIGN_UP`, `DOWNLOAD`, `SHOP_NOW`, `BOOK_NOW`, `GET_OFFER`, `SUBSCRIBE`, `CONTACT_US`, `APPLY_NOW`, `WATCH_MORE`. `validate` reports any invalid objective/goal/CTA/status, missing required fields, and missing media files; defaults are applied in place.

## Command reference

| Command | Usage |
|---------|-------|
| `create --config <file> [--dry-run \| --yes]` | Deploy a full campaign from YAML |
| `validate --config <file>` | Validate config, no API |
| `status <campaign-id>` | Campaign + ad sets + ads status |
| `account [--json-output]` | Ad account summary |
| `campaigns \| adsets \| ads [--limit N] [--json-output]` | List entities (limit cap: 100) |
| `insights [object-id] [--level ...] [--date-preset ...] [--json-output]` | Insights; default target is the ad account |
| `budget <object-id> <amount> [--live] [--yes]` | Update daily budget (cents for USD, whole units for IDR); respects `META_ADS_MAX_DAILY_BUDGET_CENTS` |
| `upload-image <path> [--live] [--yes]` | Upload an image, print its hash |
| `upload-video <path> [--live] [--yes]` | Upload a video, print its video ID |
| `add-ad <ad-set-id> [--image <p> \| --video <p> --thumbnail <p>] --headline "..." --primary-text "..." --link "..." [--live] [--yes]` | Attach a single ad to existing ad set |
| `bulk-status <PAUSED\|ACTIVE\|DELETED> <id...> [--live] [--yes]` | Bulk status updates |
| `pause \| activate \| delete <campaign-id> [--yes]` | Single-campaign status operations |
| `setup --client <claude\|cursor\|codex\|chatgpt>` | Print MCP setup snippets |

## Audit log

Mutating commands append JSONL events `{"timestamp","action","ad_account_id","request","result"}`. Credentials are never written. On API failure the command records the failure (with partial progress for `create`) and exits non-zero.
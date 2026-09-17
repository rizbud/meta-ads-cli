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
- `META_ADS_MAX_DAILY_BUDGET_CENTS` — budget change guardrail
- `META_ADS_AUDIT_LOG_PATH` — default `~/.meta-ads-cli/audit.jsonl`

Setup: `cp .env.example .env`, then edit.

## Standard workflow

1. `validate` — no API calls:
   `meta-ads validate --config campaign.yaml`
2. `create --dry-run` — previews every `[DRY RUN] POST act_<id>/...` call, no network traffic (still writes an audit event):
   `meta-ads create --config campaign.yaml --dry-run`
3. `create --yes` — deploys for real (skips the confirmation prompt):
   `meta-ads create --config campaign.yaml --yes`

Everything is created as `PAUSED` by default so you can review before spending. `budget`, `upload-image`, and `bulk-status` also default to dry run — live calls need `--live` (plus `--yes`). `activate` and `delete` always confirm unless `--yes`.

## Config schema (campaign.yaml)

```yaml
campaign:
  name: "My Campaign"                # required
  objective: OUTCOME_TRAFFIC         # default OUTCOME_TRAFFIC
  status: PAUSED                     # PAUSED or ACTIVE
  special_ad_categories: []          # optional
ad_set:
  name: "My Ad Set"                  # required
  daily_budget: 1000                 # required, in cents (1000 = $10/day)
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
  - name: "My Ad"                    # required
    image: ./images/ad.png           # required; relative paths resolve against the YAML file's dir
    primary_text: "copy"             # required
    headline: "Headline"             # required
    link: "https://example.com"      # required
    description: ""                  # optional
    cta: LEARN_MORE                  # default LEARN_MORE
```

Open objectives: `OUTCOME_TRAFFIC`, `OUTCOME_AWARENESS`, `OUTCOME_ENGAGEMENT`, `OUTCOME_LEADS`, `OUTCOME_SALES`, `OUTCOME_APP_PROMOTION`. CTAs: `LEARN_MORE`, `SIGN_UP`, `DOWNLOAD`, `SHOP_NOW`, `BOOK_NOW`, `GET_OFFER`, `SUBSCRIBE`, `CONTACT_US`, `APPLY_NOW`, `WATCH_MORE`. `validate` reports any invalid objective/goal/CTA/status, missing required fields, and missing image files; defaults are applied in place.

## Command reference

| Command | Usage |
|---------|-------|
| `create --config <file> [--dry-run \| --yes]` | Deploy a full campaign from YAML |
| `validate --config <file>` | Validate config, no API |
| `status <campaign-id>` | Campaign + ad sets + ads status |
| `account [--json-output]` | Ad account summary |
| `campaigns \| adsets \| ads [--limit N] [--json-output]` | List entities (limit cap: 100) |
| `insights [object-id] [--level ...] [--date-preset ...] [--json-output]` | Insights; default target is the ad account |
| `budget <object-id> <cents> [--live] [--yes]` | Update daily budget; respects `META_ADS_MAX_DAILY_BUDGET_CENTS` |
| `upload-image <path> [--live] [--yes]` | Upload an image, print its hash |
| `bulk-status <PAUSED\|ACTIVE\|DELETED> <id...> [--live] [--yes]` | Bulk status updates |
| `pause \| activate \| delete <campaign-id> [--yes]` | Single-campaign status operations |
| `setup --client <claude\|cursor\|codex\|chatgpt>` | Print MCP setup snippets |

## Audit log

Mutating commands append JSONL events `{"timestamp","action","ad_account_id","request","result"}`. Credentials are never written. On API failure the command records the failure (with partial progress for `create`) and exits non-zero.
---
name: meta-ads-cli
description: Use when the user wants to use or work with the meta-ads-cli binary in this repo to create and manage Meta (Facebook/Instagram) ad campaigns. Triggers include "meta-ads", "campaign.yaml", "Meta Ads campaign", "launch an ad", "ad budget", "META_ACCESS_TOKEN", "dry-run", "audit log", or questions about the create/validate/budget/insights/status commands. Covers how to configure and run the prebuilt binary at ./meta-ads, the YAML config schema, environment variables, commands, and safety controls. Use ONLY for this repo's meta-ads-cli; do not use for other ad tooling.
---

# Using the meta-ads-cli

`meta-ads` is a prebuilt binary at `./meta-ads` (already compiled — no build step needed) that manages Meta (Facebook/Instagram) ad campaigns: deploy a campaign from a YAML file (campaign → ad set → creatives → ads), list accounts/campaigns/adsets/ads/insights, adjust budgets, bulk-toggle statuses, and log every mutation to a local JSONL audit file.

Run it from the repo root: `./meta-ads --help`.

All commands print messages to stdout and exit non-zero on failure.

## Credentials

The CLI reads these from the environment or a `.env` file in the current directory, and **requires all three** before talking to Meta (even for dry run):

- `META_ACCESS_TOKEN` (required)
- `META_AD_ACCOUNT_ID` — numbers only, no `act_` prefix (required)
- `META_PAGE_ID` (required)
- `META_API_VERSION` — default `v21.0`
- `META_ADS_MAX_DAILY_BUDGET_CENTS` — budget change guardrail
- `META_ADS_AUDIT_LOG_PATH` — default `~/.meta-ads-cli/audit.jsonl`

Setup: `cp .env.example .env`, then edit.

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
    image: ./images/ad.png           # required; relative paths resolve against the YAML file's directory
    primary_text: "copy"             # required; leading/trailing whitespace is stripped
    headline: "Headline"             # required
    link: "https://example.com"      # required
    description: ""                  # optional
    cta: LEARN_MORE                  # default LEARN_MORE
```

`validate` catches bad sections/fields: allowed objectives, optimization goals, CTAs, statuses, missing required fields, and image files that do not exist. Optional fields get defaults applied in place.

## Standard workflow

1. `./meta-ads validate --config campaign.yaml` — no API calls.
2. `./meta-ads create --config campaign.yaml --dry-run` — prints the exact API calls as `[DRY RUN] POST act_<id>/campaigns` previews; makes no network requests; still writes an audit event.
3. `./meta-ads create --config campaign.yaml --yes` — live deploy. Asks to confirm unless `--yes`.

Commands default to **dry run** (no network): `budget`, `upload-image`, `bulk-status`. Live requires `--live` (plus `--yes` to skip the confirmation). `activate` and `delete` always confirm unless `--yes`. `create` also creates everything as `PAUSED` by default.

## Command reference

| Command | Purpose |
|---------|---------|
| `create --config <file> [--dry-run \| --yes]` | Deploy a full campaign from YAML |
| `validate --config <file>` | Validate config, no API |
| `status <campaign-id>` | Campaign + ad sets + ads status |
| `account [--json-output]` | Ad account summary |
| `campaigns \| adsets \| ads [--limit N] [--json-output]` | List entities (limit capped at 100) |
| `insights [object-id] [--level ...] [--date-preset ...] [--json-output]` | Insights; default target is the ad account |
| `budget <object-id> <cents> [--live] [--yes]` | Update daily budget; respects `META_ADS_MAX_DAILY_BUDGET_CENTS` |
| `upload-image <path> [--live] [--yes]` | Upload an image, print its hash |
| `bulk-status <PAUSED\|ACTIVE\|DELETED> <id...> [--live] [--yes]` | Bulk status updates |
| `pause \| activate \| delete <campaign-id> [--yes]` | Single-campaign status operations |
| `setup --client <claude\|cursor\|codex\|chatgpt>` | Print MCP setup snippets |

## Audit log

Mutating commands append a JSONL event `{"timestamp","action","ad_account_id","request","result"}`. Credentials are never written. On API failure, commands record the failure (with partial progress for `create`) and exit non-zero.
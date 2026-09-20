# meta-ads-cli

Create and manage Meta (Facebook/Instagram) ad campaigns from your terminal.

Define a campaign in one YAML file and deploy it with one command:

```
meta-ads create --config campaign.yaml
```

One campaign. One ad set. Multiple ads. You also get account lookup, campaign listings, insights, budget updates, bulk status operations, spend limits, local audit logs, and safety rails (everything mutable defaults to dry run).

> Works as a tiny [Meta Marketing API](https://developers.facebook.com/docs/marketing-apis/) wrapper — no heavy SDKs — reimplemented in Go from [Attainment Labs' meta-ads-cli](https://github.com/attainmentlabs/meta-ads-cli). Improvements: **no Python required**, a single static binary, one-line `curl` install with prebuilt binaries, and fast startup — same YAML config and commands, so it's a drop-in replacement.

## Table of Contents

- [Install & Quick Start](#install--quick-start)
- [Getting a Meta Access Token](#getting-a-meta-access-token)
- [Environment Variables](#environment-variables)
- [Campaign Config (YAML)](#campaign-config-yaml)
- [Commands](#commands)
  - [create](#create)
  - [status <campaign-id>](#status-campaign-id)
  - [account](#account)
  - [campaigns / adsets / ads](#campaigns--adsets--ads)
  - [insights](#insights)
  - [budget <object-id> <daily-budget-cents>](#budget-object-id-daily-budget-cents)
  - [upload-image <image-path>](#upload-image-image-path)
  - [upload-video <video-path>](#upload-video-video-path)
  - [add-ad <ad-set-id>](#add-ad-ad-set-id)
  - [bulk-status <status> <campaign-id...>](#bulk-status-status-campaign-id)
  - [pause / activate / delete](#pause--activate--delete)
  - [validate](#validate)
  - [setup](#setup)
- [Safety Controls](#safety-controls)
- [Automation](#automation)
- [Finding Interest IDs](#finding-interest-ids)
- [Examples](#examples)
- [How It Works](#how-it-works)
- [How to Contribute](#how-to-contribute)

## Install & Quick Start

### Install

One command, no clone needed. `install.sh` fetches a prebuilt binary for your OS and architecture (e.g. `meta-ads-linux-amd64`) from the [releases page](https://github.com/rizbud/meta-ads-cli/releases) and installs it so you can run `meta-ads` from anywhere. No Go required:

```bash
curl -fsSL https://raw.githubusercontent.com/rizbud/meta-ads-cli/main/install.sh | bash
meta-ads --version
```

Want your agents to know the CLI too? Add `--skill` to also install the usage docs to `~/.agents/skills/meta-ads-cli/`:

```bash
curl -fsSL https://raw.githubusercontent.com/rizbud/meta-ads-cli/main/install.sh | bash -s -- --skill
```

From a checkout of the repo, `install.sh` reuses the prebuilt `./meta-ads` (no Go needed):

```bash
./install.sh
./install.sh --skill
```

The script installs to, in order of preference: a directory you pass explicitly (`./install.sh /custom/bin`), `INSTALL_DIR`, then the first existing bin directory on your `PATH` (`~/go/bin`, `~/.local/bin`, `/usr/local/bin`), falling back to `~/.local/bin`. If the target is not on your `PATH`, it prints the line to add (e.g. `export PATH="/custom/bin:$PATH"`, persisted in `~/.bashrc`).

### Quick Start

**1. Set up credentials**

```bash
cp .env.example .env
# Edit .env with your Meta access token, ad account ID, and page ID
```

The CLI loads `.env` from the current directory. You can also export the variables in your shell.

**2. Create your campaign config**

```bash
cp campaign.example.yaml campaign.yaml
# Edit campaign.yaml with your ad copy, images, targeting, and budget
```

**3. Validate your config**

```bash
meta-ads validate --config campaign.yaml
```

**4. Preview with dry run**

```bash
meta-ads create --config campaign.yaml --dry-run
```

**5. Deploy**

```bash
meta-ads create --config campaign.yaml --yes
```

Your campaign is created as `PAUSED` by default. Review it in Ads Manager, then activate it when ready.

## Getting a Meta Access Token

This is the part most people get stuck on. Here is the short version:

1. Go to [Meta for Developers](https://developers.facebook.com/) and create an app (type: Business)
2. Open the [Graph API Explorer](https://developers.facebook.com/tools/explorer/)
3. Select your app, then request these permissions: `ads_management`, `pages_read_engagement`, `pages_show_list`
4. Click "Generate Access Token" and authorize
5. Copy the token to your `.env` file

**Important:** Graph API Explorer tokens expire after about 2 hours. For production use, exchange it for a long-lived token:

```bash
curl "https://graph.facebook.com/v21.0/oauth/access_token?\
grant_type=fb_exchange_token&\
client_id=YOUR_APP_ID&\
client_secret=YOUR_APP_SECRET&\
fb_exchange_token=YOUR_SHORT_LIVED_TOKEN"
```

Long-lived tokens last about 60 days.

## Environment Variables

| Variable                           | Required | Description                                        |
|------------------------------------|----------|----------------------------------------------------|
| `META_ACCESS_TOKEN`                | Yes      | Your Meta API access token                          |
| `META_AD_ACCOUNT_ID`               | Yes      | Your ad account ID (numbers only, no `act_` prefix) |
| `META_PAGE_ID`                     | Yes      | Your Facebook Page ID                               |
| `META_API_VERSION`                 | No       | API version (default: `v21.0`)                      |
| `META_CURRENCY`                    | No       | Currency code for budget amounts in offline commands (`validate`, `--dry-run`). Live commands read it from your account. |
| `META_ADS_MAX_DAILY_BUDGET_CENTS`  | No       | Optional daily budget guardrail                     |
| `META_ADS_AUDIT_LOG_PATH`          | No       | Optional JSONL audit log path (default: `~/.meta-ads-cli/audit.jsonl`) |

## Campaign Config (YAML)

```yaml
campaign:
  name: "My Campaign"
  objective: OUTCOME_TRAFFIC        # OUTCOME_TRAFFIC | OUTCOME_AWARENESS | OUTCOME_ENGAGEMENT | OUTCOME_LEADS | OUTCOME_SALES | OUTCOME_APP_PROMOTION
  status: PAUSED                    # PAUSED or ACTIVE
  special_ad_categories: []         # Leave empty unless required

ad_set:
  name: "My Ad Set"
  daily_budget: 1000                # In cents. 1000 = $10/day
  optimization_goal: LINK_CLICKS    # LINK_CLICKS | IMPRESSIONS | REACH | LANDING_PAGE_VIEWS | APP_INSTALLS | OFFSITE_CONVERSIONS | LEAD_GENERATION
  targeting:
    age_min: 18                     # Optional (defaults: 18, 65, genders [0])
    age_max: 65
    genders: [0]                    # 0 = all, 1 = male, 2 = female
    countries: ["US"]
    interests:                      # Optional
      - id: "6003139266461"
        name: "Fitness and wellness"
    platforms: ["facebook", "instagram"]
    facebook_positions: ["feed"]
    instagram_positions: ["stream", "story", "reels"]

ads:
  - name: "My Ad"
    image: ./images/ad.png          # Path relative to YAML file
    primary_text: "Your ad copy."
    headline: "Your Headline"
    description: "Short description"
    cta: LEARN_MORE                 # LEARN_MORE | SIGN_UP | DOWNLOAD | SHOP_NOW | BOOK_NOW | GET_OFFER | SUBSCRIBE | CONTACT_US | APPLY_NOW | WATCH_MORE
    link: "https://example.com"

  - name: "My Reel Ad"               # Video ad: use `video` + `thumbnail` instead of `image`
    video: ./videos/reel.mp4
    thumbnail: ./images/reel-cover.png  # Still image Meta shows as the video's cover
    primary_text: "Your ad copy."
    headline: "Your Headline"
    cta: SHOP_NOW
    link: "https://example.com"
```

**CTA Options:** `LEARN_MORE`, `SIGN_UP`, `DOWNLOAD`, `SHOP_NOW`, `BOOK_NOW`, `GET_OFFER`, `SUBSCRIBE`, `CONTACT_US`, `APPLY_NOW`, `WATCH_MORE`

Each ad is either an image ad (`image`) or a video ad (`video` + `thumbnail`) — set exactly one. Video ads are uploaded via the Marketing API's `/advideos` endpoint and processed asynchronously by Meta; the ad creative is created immediately after upload, which is normally fine, but if Meta rejects it with a "video not ready" style error, wait a minute and re-run.

## Commands

### `create`

Create a full campaign from your YAML config.

```bash
meta-ads create --dry-run                      # Preview
meta-ads create                                # Deploy (asks for confirmation)
meta-ads create --yes                          # Deploy without confirmation
meta-ads create --config path/to/campaign.yaml # Custom config file
```

### `status <campaign-id>`

Check the status of a campaign and all its ads.

```bash
meta-ads status 120243616427570285
```

### `account`

```bash
meta-ads account
meta-ads account --json-output
```

### `campaigns` / `adsets` / `ads`

```bash
meta-ads campaigns --limit 25          # max 100
meta-ads adsets --limit 25
meta-ads ads --limit 25
meta-ads campaigns --json-output
```

### `insights`

```bash
meta-ads insights --level campaign --date-preset last_7d
meta-ads insights 120243616427570285 --date-preset last_30d --json-output
```

### `budget <object-id> <daily-budget-cents>`

Defaults to dry run. Use `--live` to actually change it, `--yes` to skip the prompt.

```bash
meta-ads budget 120243616427570285 3000
meta-ads budget 120243616427570285 3000 --live
meta-ads budget 120243616427570285 3000 --live --yes
```

### `upload-image <image-path>`

Defaults to dry run.

```bash
meta-ads upload-image ./images/ad.png
meta-ads upload-image ./images/ad.png --live --yes
```

### `upload-video <video-path>`

Defaults to dry run. Meta processes uploaded video asynchronously; the returned video ID is usable in an ad creative right away, but may briefly return a "not ready" error until processing finishes.

```bash
meta-ads upload-video ./videos/reel.mp4
meta-ads upload-video ./videos/reel.mp4 --live --yes
```

### `add-ad <ad-set-id>`

Attach a single new ad — image or video — to an ad set that already exists, without redeploying the whole campaign. Useful for adding one more creative variant (e.g. a Reels/Stories video ad) to a running ad set. Defaults to dry run.

```bash
meta-ads add-ad 120251423501650356 \
  --name "Reel Ad V1" \
  --video ./videos/reel.mp4 \
  --thumbnail ./images/reel-cover.png \
  --primary-text "Your ad copy." \
  --headline "Your Headline" \
  --link "https://example.com" \
  --cta SHOP_NOW \
  --live --yes
```

Use `--image` instead of `--video`/`--thumbnail` for an image ad. `--status` defaults to `PAUSED`.

### `bulk-status <status> <campaign-id...>`

Bulk pause, activate, or delete campaigns. Defaults to dry run.

```bash
meta-ads bulk-status PAUSED 111 222 333
meta-ads bulk-status ACTIVE 111 222 333 --live
meta-ads bulk-status DELETED 111 222 333 --live --yes
```

### `pause` / `activate` / `delete`

```bash
meta-ads pause 120243616427570285
meta-ads activate 120243616427570285 --yes       # Starts spending. Asks to confirm unless --yes.
meta-ads delete 120243616427570285 --yes         # Cannot be undone. Asks to confirm unless --yes.
```

### `validate`

Validate your YAML config without making any API calls.

```bash
meta-ads validate --config campaign.yaml
```

### `setup`

Print local MCP setup snippets for Claude, Cursor, Codex, or ChatGPT.

```bash
meta-ads setup --client claude
meta-ads setup --client cursor
meta-ads setup --client codex
meta-ads setup --client chatgpt
```

## Safety Controls

- Campaigns are created as `PAUSED` by default.
- `budget`, `upload-image`, `upload-video`, `add-ad`, and `bulk-status` default to dry run. Live operations require `--live`, with an interactive confirmation unless `--yes` is passed.
- `META_ADS_MAX_DAILY_BUDGET_CENTS` blocks budget changes above your chosen cap.
- `activate` and `delete` ask for confirmation unless `--yes` is passed.
- Mutating commands write JSONL audit events. Default path: `~/.meta-ads-cli/audit.jsonl`. Credentials are never written.

## Automation

Built for scripting. Mutating commands take `--yes` to skip prompts, read commands take `--json-output` for machine-readable output, and every command exits non-zero on failure.

```bash
for cfg in campaigns/*.yaml; do
  meta-ads create --config "$cfg" --yes
done
```

```bash
meta-ads campaigns --json-output | jq '.[] | {id, name, status}'
meta-ads insights --level campaign --date-preset last_7d --json-output > insights.json
```

## Finding Interest IDs

Interest targeting requires Meta's internal IDs. Search for them using the API:

```bash
curl "https://graph.facebook.com/v21.0/search?\
type=adinterest&\
q=fitness&\
access_token=YOUR_TOKEN"
```

This returns interest names and IDs you can use in your campaign YAML.

## Examples

See the [`examples/`](examples/) directory for ready-to-customize campaign configs: `ecommerce.yaml` (product launch) and `app-install.yaml` (mobile app install).

## How It Works

The tool wraps the Meta Marketing API directly. The full `create` chain:

1. Uploads your ad images and videos to your ad account
2. Creates a campaign with your objective
3. Creates an ad set with your budget and targeting
4. Creates ad creatives linking your images/videos and copy
5. Creates ads linking creatives to the ad set

Everything is created as `PAUSED` by default so you can review before spending.

## How to Contribute

PRs welcome. The project is intentionally lightweight.

**Requirements:** Go 1.23+

**Build and run locally:**

```bash
git clone https://github.com/rizbud/meta-ads-cli.git
cd meta-ads-cli
go build -o meta-ads .
go install .        # or install into your GOBIN
./meta-ads --help
```

**Run tests:**

```bash
go test ./...
```

**Project layout:**

- `cmd/` — CLI commands (cobra), confirmation prompts, dry-run/live flags
- `config/` — YAML config loading and validation
- `api/` — Meta Graph API client with dry-run support
- `campaign/` — full campaign orchestration and status printing
- `audit/` — JSONL audit events
- `.agents/skills/meta-ads-cli/` — agent skill (usage docs; install with `install.sh --skill`)
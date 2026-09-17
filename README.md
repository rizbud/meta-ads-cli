# meta-ads-cli

Create and manage Meta (Facebook/Instagram) ad campaigns from your terminal.

> Go port of the open-source [Attainment Labs meta-ads-cli](https://github.com/attainmentlabs/meta-ads-cli). No heavy SDKs — a thin wrapper over the [Meta Marketing API](https://developers.facebook.com/docs/marketing-apis/).

Define a campaign in one YAML file and deploy it with one command:

```
meta-ads create --config campaign.yaml
```

One campaign. One ad set. Multiple ads. You also get account lookup, campaign listings, insights, budget updates, bulk status operations, setup snippets, spend limits, local audit logs, and safety rails (everything mutable defaults to dry run).

## Requirements

- Go 1.23+

## Build

```bash
git clone https://github.com/rizbud/meta-ads-cli.git
cd meta-ads-cli
go build -o meta-ads .
```

Or install into your `GOBIN`:

```bash
go install .
```

## Install (run from anywhere)

One command, no clone needed. `install.sh` fetches a prebuilt binary from the [releases page](https://github.com/rizbud/meta-ads-cli/releases) and installs it so you can run `meta-ads` without a `./` prefix. No Go required:

```bash
curl -fsSL https://raw.githubusercontent.com/rizbud/meta-ads-cli/main/install.sh | bash
meta-ads --version   # now a global command
```

The script picks the binary matching your OS and architecture (e.g. `meta-ads-linux-amd64`, `meta-ads-darwin-arm64`) from the latest release. If no matching release asset exists yet, it falls back to building from source — which needs Go 1.23+ on your path.

From a checkout of the repo, `install.sh` instead reuses the prebuilt `./meta-ads` so no Go is needed:

```bash
./install.sh
```

The script installs to, in order of preference:

1. A directory you pass explicitly: `./install.sh /custom/bin`
2. `INSTALL_DIR`: `INSTALL_DIR=/custom/bin ./install.sh`
3. The first existing bin directory already on your `PATH` (`~/go/bin`, `~/.local/bin`, `/usr/local/bin`), falling back to `~/.local/bin`

If the target directory is not on your `PATH`, the script prints the line to add:

```bash
export PATH="/custom/bin:$PATH"    # append to ~/.bashrc to persist
```

### Publishing a release

Releases are built by a [GitHub Actions workflow](.github/workflows/release.yml), no local build needed. Cross-compiles `dist/meta-ads-<os>-<arch>` for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64` — the exact asset names `install.sh` looks up — and uploads them to the release.

Just tag and push:

```bash
git tag v0.2.1
git push origin v0.2.1
```

The `on: push: tags: ["v*"]` trigger runs the job; it needs `actions: write` permission, which the workflow declares (first-time setup may require approving the workflow on your repo settings).

## Quick Start

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
./meta-ads validate --config campaign.yaml
```

**4. Preview with dry run**

```bash
./meta-ads create --config campaign.yaml --dry-run
```

**5. Deploy**

```bash
./meta-ads create --config campaign.yaml --yes
```

Your campaign is created as `PAUSED` by default. Review it in Ads Manager, then activate it when ready.

## Environment Variables

| Variable                           | Required | Description                                        |
|------------------------------------|----------|----------------------------------------------------|
| `META_ACCESS_TOKEN`                | Yes      | Your Meta API access token                          |
| `META_AD_ACCOUNT_ID`               | Yes      | Your ad account ID (numbers only, no `act_` prefix) |
| `META_PAGE_ID`                     | Yes      | Your Facebook Page ID                               |
| `META_API_VERSION`                 | No       | API version (default: `v21.0`)                      |
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
    age_min: 18
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
```

**CTA Options:** `LEARN_MORE`, `SIGN_UP`, `DOWNLOAD`, `SHOP_NOW`, `BOOK_NOW`, `GET_OFFER`, `SUBSCRIBE`, `CONTACT_US`, `APPLY_NOW`, `WATCH_MORE`

## Commands

### `create`

Create a full campaign from your YAML config.

```bash
./meta-ads create --dry-run                      # Preview
./meta-ads create                                # Deploy (asks for confirmation)
./meta-ads create --yes                          # Deploy without confirmation
./meta-ads create --config path/to/campaign.yaml # Custom config file
```

### `status <campaign-id>`

Check the status of a campaign and all its ads.

```bash
./meta-ads status 120243616427570285
```

### `account`

```bash
./meta-ads account
./meta-ads account --json-output
```

### `campaigns` / `adsets` / `ads`

```bash
./meta-ads campaigns --limit 25          # max 100
./meta-ads adsets --limit 25
./meta-ads ads --limit 25
./meta-ads campaigns --json-output
```

### `insights`

```bash
./meta-ads insights --level campaign --date-preset last_7d
./meta-ads insights 120243616427570285 --date-preset last_30d --json-output
```

### `budget <object-id> <daily-budget-cents>`

Defaults to dry run. Use `--live` to actually change it, `--yes` to skip the prompt.

```bash
./meta-ads budget 120243616427570285 3000
./meta-ads budget 120243616427570285 3000 --live
./meta-ads budget 120243616427570285 3000 --live --yes
```

### `upload-image <image-path>`

Defaults to dry run.

```bash
./meta-ads upload-image ./images/ad.png
./meta-ads upload-image ./images/ad.png --live --yes
```

### `bulk-status <status> <campaign-id...>`

Bulk pause, activate, or delete campaigns. Defaults to dry run.

```bash
./meta-ads bulk-status PAUSED 111 222 333
./meta-ads bulk-status ACTIVE 111 222 333 --live
./meta-ads bulk-status DELETED 111 222 333 --live --yes
```

### `pause` / `activate` / `delete`

```bash
./meta-ads pause 120243616427570285
./meta-ads activate 120243616427570285 --yes       # Starts spending. Asks to confirm unless --yes.
./meta-ads delete 120243616427570285 --yes         # Cannot be undone. Asks to confirm unless --yes.
```

### `validate`

Validate your YAML config without making any API calls.

```bash
./meta-ads validate --config campaign.yaml
```

### `setup`

Print local MCP setup snippets for Claude, Cursor, Codex, or ChatGPT.

```bash
./meta-ads setup --client claude
./meta-ads setup --client cursor
./meta-ads setup --client codex
./meta-ads setup --client chatgpt
```

## Safety Controls

- Campaigns are created as `PAUSED` by default.
- `budget`, `upload-image`, and `bulk-status` default to dry run. Live operations require `--live`, with an interactive confirmation unless `--yes` is passed.
- `META_ADS_MAX_DAILY_BUDGET_CENTS` blocks budget changes above your chosen cap.
- `activate` and `delete` ask for confirmation unless `--yes` is passed.
- Mutating commands write JSONL audit events. Default path: `~/.meta-ads-cli/audit.jsonl`. Credentials are never written.

## Automation

Built for scripting. Mutating commands take `--yes` to skip prompts, read commands take `--json-output` for machine-readable output, and every command exits non-zero on failure.

```bash
for cfg in campaigns/*.yaml; do
  ./meta-ads create --config "$cfg" --yes
done
```

```bash
./meta-ads campaigns --json-output | jq '.[] | {id, name, status}'
./meta-ads insights --level campaign --date-preset last_7d --json-output > insights.json
```

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

1. Uploads your ad images to your ad account
2. Creates a campaign with your objective
3. Creates an ad set with your budget and targeting
4. Creates ad creatives linking your images and copy
5. Creates ads linking creatives to the ad set

Everything is created as `PAUSED` by default so you can review before spending.

## Running Tests

```bash
go test ./...
```

## Project Layout

- `cmd/` — CLI commands (cobra), confirmation prompts, dry-run/live flags
- `config/` — YAML config loading and validation
- `api/` — Meta Graph API client with dry-run support
- `campaign/` — full campaign orchestration and status printing
- `audit/` — JSONL audit events


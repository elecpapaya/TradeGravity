# TradeGravity

TradeGravity collects the latest trade values between each reporter country and the USA/CHN, then publishes a static treemap viewer.

Exports/imports are reported from each reporter's perspective, with USA/CHN as the partner side.

## Features
- WITS SDMX ingestion with automatic "latest year" selection.
- UN Comtrade ingestion (API key via secrets).
- SQLite persistence for repeatable runs.
- Static JSON output for a lightweight web viewer.
- Treemap with linked hover highlights and flag overlays.
- Optional growth coloring (YoY) when historical data is available.
- Optional country snapshot (World Bank indicators + GDELT headlines).
- Allowlist filter for targeted reporter coverage.

## Architecture
- `collector` (Go CLI): fetches WITS data and stores observations.
- `store` (SQLite): persists trade observations.
- `publisher` (Go CLI): builds `site/data/latest.json` and `site/data/meta.json`.
- `site` (static): HTML/CSS/JS treemap viewer.

## Requirements
- Go 1.22+
- Internet access to WITS and public CDNs.

## Quick Start
```bash
go run ./cmd/collector run
go run ./cmd/publisher build --out site/data
cd site
python -m http.server 8080
```
Then open `http://localhost:8080`.

## Configuration
Collector options (example):
```bash
go run ./cmd/collector run -partners USA,CHN -flows export,import -allowlist configs/allowlist.csv -history-years 1
```

WITS provider env vars (optional):
- `WITS_BASE_URL` (default `https://wits.worldbank.org/API/V1/`)
- `WITS_API_KEY` (optional; WITS supports access without a key)
- `WITS_TRADE_PATH`
- `WITS_RATE_LIMIT_PER_SEC`

Comtrade provider env vars (required for API access):
- `COMTRADE_PRIMARY_KEY`
- `COMTRADE_SECONDARY_KEY` (optional fallback)
- `COMTRADE_BASE_URL` (default `https://comtradeapi.un.org/`)
- `COMTRADE_DATA_PATH` (default `data/v1/get/{type}/{freq}/{cl}`)
- `COMTRADE_RATE_LIMIT_PER_SEC` (default `2`)
- `COMTRADE_RATE_LIMIT_BURST` (default `2`)
- `COMTRADE_MAX_RETRIES` (default `3`)
- `COMTRADE_REPORTERS_URL` (default `https://comtradeapi.un.org/files/v1/app/reference/Reporters.json`)
- `COMTRADE_PARTNERS_URL` (default `https://comtradeapi.un.org/files/v1/app/reference/partnerAreas.json`)

Local setup for `COMTRADE_PRIMARY_KEY` (this repo reads OS env vars; no `.env` loader):
```powershell
# PowerShell: current session
$env:COMTRADE_PRIMARY_KEY = "YOUR_KEY"

# PowerShell: persist for new terminals
setx COMTRADE_PRIMARY_KEY "YOUR_KEY"
```
`setx` writes a user-level persistent env var and takes effect in new terminals or after reboot.
```bash
# bash/zsh: current session
export COMTRADE_PRIMARY_KEY="YOUR_KEY"

# bash/zsh: persist
echo 'export COMTRADE_PRIMARY_KEY="YOUR_KEY"' >> ~/.bashrc
```

Comtrade usage:
```bash
go run ./cmd/collector run -provider comtrade -partners USA,CHN -flows export,import -allowlist configs/allowlist.csv -history-years 1
```

GitHub Actions secrets:
- `COMTRADE_PRIMARY_KEY`
- `COMTRADE_SECONDARY_KEY`

Collector flags (common):
- `-history-years` to fetch previous years for YoY growth (default `1`).

Generated files:
- SQLite DB: `tradegravity.db`
- Output JSON: `site/data/*.json` (generated, not committed)

## Deployment
This repo includes a GitHub Actions workflow that runs the collector, builds JSON, and publishes `site/` to GitHub Pages.

## License
See `LICENSE`.

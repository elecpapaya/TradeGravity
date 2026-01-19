# TradeGravity

TradeGravity collects the latest trade values between each reporter country and the USA/CHN, then publishes a static treemap viewer.

Exports/imports are reported from each reporter's perspective, with USA/CHN as the partner side.

## Features
- WITS SDMX ingestion with automatic "latest year" selection.
- SQLite persistence for repeatable runs.
- Static JSON output for a lightweight web viewer.
- Treemap with linked hover highlights and flag overlays.
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
go run ./cmd/collector run -partners USA,CHN -flows export,import -allowlist configs/allowlist.csv
```

WITS provider env vars (optional):
- `WITS_BASE_URL` (default `https://wits.worldbank.org/API/V1/`)
- `WITS_API_KEY` (optional; WITS supports access without a key)
- `WITS_TRADE_PATH`
- `WITS_RATE_LIMIT_PER_SEC`

Generated files:
- SQLite DB: `tradegravity.db`
- Output JSON: `site/data/*.json` (generated, not committed)

## Deployment
This repo includes a GitHub Actions workflow that runs the collector, builds JSON, and publishes `site/` to GitHub Pages.

## License
See `LICENSE`.

# TradeGravity

[![Quality checks](https://github.com/elecpapaya/TradeGravity/actions/workflows/quality.yml/badge.svg)](https://github.com/elecpapaya/TradeGravity/actions/workflows/quality.yml)
[![Daily data update](https://github.com/elecpapaya/TradeGravity/actions/workflows/update-tradegravity.yml/badge.svg)](https://github.com/elecpapaya/TradeGravity/actions/workflows/update-tradegravity.yml)
[![CodeQL](https://github.com/elecpapaya/TradeGravity/actions/workflows/codeql.yml/badge.svg)](https://github.com/elecpapaya/TradeGravity/actions/workflows/codeql.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![User study recruiting](https://img.shields.io/badge/user%20study-recruiting-008672.svg)](https://github.com/elecpapaya/TradeGravity/issues/3)

TradeGravity is an open-source pipeline and tabbed static intelligence dashboard for exploring global trade and supply-chain exposure. It combines same-period US/China headline comparisons, reported multi-partner networks, 5–10 year trends, HS2 chapters, curated strategic HS6 products, WITS/TRAINS tariffs, country context, transparent scenario sensitivity, and explicit quality signals in a deployment that needs no application server.

- **Live demo:** https://elecpapaya.github.io/TradeGravity/
- **System design:** [DESIGN.md](DESIGN.md)
- **Published data schema:** [docs/DATA_SCHEMA.md](docs/DATA_SCHEMA.md)
- **Reuse examples:** [docs/USAGE.md](docs/USAGE.md)
- **Data rights and attribution:** [docs/DATA_RIGHTS.md](docs/DATA_RIGHTS.md)
- **Project roadmap:** [ROADMAP.md](ROADMAP.md)
- **How to cite:** [CITATION.cff](CITATION.cff)

## Why this project exists

Public trade data is valuable but often difficult to compare quickly across many countries. Source APIs use different response shapes, countries can have different latest reporting periods, and a raw table makes the relative scale of US- and China-linked trade hard to see.

TradeGravity provides a reproducible path from public source data to a lightweight visualization. The code, transformation rules, deployment workflow, and generated-data schema are public so that researchers, students, and developers can inspect or adapt the process.

## Project status

TradeGravity is an early-stage project under active maintenance. A scheduled GitHub Actions workflow currently refreshes and deploys the public dataset every day. The default allowlist publishes 51 reporter countries; coverage can be changed through configuration.

**Help test v0.1.0:** we are recruiting three students, researchers, or developers for a 15-minute, task-based evaluation. No setup or trade-data expertise is required. Start with [the public study tracker](https://github.com/elecpapaya/TradeGravity/issues/3), then submit only nonidentifying feedback through the linked form. Participation is voluntary, and public feedback is recorded only with consent.

The viewer is intended for exploration and education, not financial, legal, or policy advice. Its default comparison mode includes only reporters whose USA and China values use the same observation period. Users can opt into all available values, where mixed or stale periods remain visibly flagged.

The pipeline refresh timestamp indicates when TradeGravity generated the site; it does not imply that every source observation is from that date or year. The viewer and `meta.json` expose provider, coverage, and observation-period counts explicitly.

## Features

- WITS SDMX ingestion with automatic latest-year selection.
- Ten-year WITS history collection and published country time series.
- UN Comtrade HS2 product chapters, with public-preview and authenticated modes.
- Curated strategic HS6 trade partitions and revision-aware WITS/TRAINS MFN tariff schedules.
- Reported UN Comtrade multi-partner `TOTAL` matrices, with export/import availability kept separate.
- SQLite persistence for repeatable collection and publishing runs.
- Static JSON output for a low-cost, serverless web viewer.
- Linked US/China treemaps with hover highlighting and flag overlays.
- Same-period comparison by default, explicit stale/missing warnings, and a data-quality dashboard.
- Region, income, ASEAN/EU, per-capita, and GDP-share filters.
- Shareable view URLs plus spreadsheet-safe CSV and filtered JSON export.
- Searchable accessible data table and selected-country 5–10 year trend.
- HS2 product mix for the selected reporter, kept separate from WITS headline totals.
- Shareable Overview, Intelligence, Products, Data & Quality, and Scenario Lab tabs with one synchronized filter and country state.
- Two-anchor concentration, balance, growth-divergence, and ranking views whose USA/China-only scope is visible in the UI, plus a selected-country multi-partner network when its matrix exists.
- An illustrative HS6 tariff sensitivity lab that can load a published MFN rate and product import baseline while exposing elasticity, pass-through, fallback, and source assumptions.
- A machine-readable `catalog.json` that separates ready, partial, and planned resources. Strategic HS6, tariffs, and bilateral matrices are published; reconciliation, value-added, and versioned scenario outputs remain planned.
- Build-time evidence-grounded explanations with citation validation and deterministic fallback.
- Year-over-year growth coloring when prior-period data is available.
- Optional World Bank indicator and GDELT headline panels.
- Reporter allowlist for controlled coverage.
- Daily collection and GitHub Pages deployment through GitHub Actions.

## How it works

```text
WITS totals/history --------\
WITS/TRAINS tariffs ---------\
UN Comtrade HS2/HS6/matrix ---> SQLite ---> publisher ---> versioned static JSON
World Bank context ----------/                  |                    |
                                             v                    v
                                  grounded explainer     static web explorer
```

- `cmd/collector` fetches and normalizes observations.
- `internal/store/sqlite` persists observations using an idempotent key.
- `cmd/context` publishes region, income, population, and GDP context.
- `cmd/publisher` calculates latest values, same-period flags, time series, products, quality, totals, shares, and growth.
- `cmd/explainer` builds citation-checked explanations; it never sends an API key to the browser.
- `site` renders the generated JSON as a static interactive viewer.

## Data sources and interpretation

| Source | Use |
| --- | --- |
| [World Integrated Trade Solution (WITS)](https://wits.worldbank.org/) | Default bilateral trade observations |
| [WITS/TRAINS](https://wits.worldbank.org/WITS/WITS/Support%20Materials/Training/GTAP_UNCTAD_Tariff_Data.asp) | Product-level tariff schedules |
| [UN Comtrade](https://comtradeplus.un.org/) | HS2/strategic HS6 trade and reported multi-partner totals |
| [World Bank Open Data](https://data.worldbank.org/) | Optional country indicators in the viewer |
| [GDELT](https://www.gdeltproject.org/) | Optional country-related headline panel |

Exports and imports are reported from each reporter country's perspective, with `USA` or `CHN` on the partner side. Trade is calculated as exports plus imports. The publisher retains the period and period type used for each partner block so users can see data freshness.

Source availability, revisions, and classification choices can affect results. TradeGravity does not modify or guarantee the accuracy of upstream data.

## Reusing and citing the data

The public deployment exposes stable machine-readable endpoints:

- `https://elecpapaya.github.io/TradeGravity/data/meta.json`
- `https://elecpapaya.github.io/TradeGravity/data/latest.json`
- `https://elecpapaya.github.io/TradeGravity/data/series.json`
- `https://elecpapaya.github.io/TradeGravity/data/products/index.json`
- `https://elecpapaya.github.io/TradeGravity/data/strategic-hs6/index.json`
- `https://elecpapaya.github.io/TradeGravity/data/tariffs/index.json`
- `https://elecpapaya.github.io/TradeGravity/data/bilateral-matrix/index.json`
- `https://elecpapaya.github.io/TradeGravity/data/quality.json`
- `https://elecpapaya.github.io/TradeGravity/data/catalog.json`
- `https://elecpapaya.github.io/TradeGravity/data/explanations/index.json`

`latest.json` is the canonical published dataset. The viewer's **Download CSV** button creates a spreadsheet-safe convenience export of the currently filtered reporters, including schema version, provider, pipeline timestamp, observation periods, flows, growth values, totals, and China share. See [docs/DATA_SCHEMA.md](docs/DATA_SCHEMA.md) before comparing reporters with different periods.

When citing a result, record the repository URL, commit or release when applicable, provider, `generated_at` timestamp, and the observation period shown for each value. GitHub can generate citation formats from [CITATION.cff](CITATION.cff).

Apache-2.0 covers the project code and original documentation, not rights in upstream observations or linked news. Review [docs/DATA_RIGHTS.md](docs/DATA_RIGHTS.md) and the selected provider's current terms before redistributing generated data.

## Requirements

- Go 1.25.12+ (includes standard-library security fixes required by CI)
- Internet access to the selected data provider and public front-end CDNs
- Python 3, or another static file server, to preview the viewer locally

## Quick start

```bash
go run ./cmd/context
go run ./cmd/collector run -provider wits -history-years 9
go run ./cmd/collector products -provider comtrade -primary-provider wits -year auto
go run ./cmd/collector strategic -provider comtrade -primary-provider wits -year auto
go run ./cmd/collector matrix -provider comtrade -primary-provider wits -year auto
go run ./cmd/collector tariffs -provider trains -year auto -data-type aveestimated
go run ./cmd/publisher build -out site/data -series-years 10
go run ./cmd/explainer -dir site/data
go run ./cmd/validator -dir site/data -min-reporters 40
cd site
python -m http.server 8080
```

Open `http://localhost:8080`.

### Offline sample preview

The production collector requires network access and can take several minutes. For UI or contribution work, copy the validated synthetic sample into the ignored output directory:

```powershell
New-Item -ItemType Directory -Force site/data
Copy-Item examples/sample-data/* site/data/ -Recurse -Force
```

```bash
mkdir -p site/data
cp -R examples/sample-data/. site/data/
```

Then serve `site/` as shown above. The three sample reporters and values are synthetic and are not evidence about real trade.

To run the automated checks:

```bash
go test ./...
go vet ./...
node --test site/security.test.cjs site/data-tools.test.cjs site/explorer-tools.test.cjs site/intelligence-tools.test.cjs site/structure.test.cjs
```

## Collector configuration

Example with explicit partners, flows, ten published years, and bounded concurrency:

```bash
go run ./cmd/collector run \
  -partners USA,CHN \
  -flows export,import \
  -allowlist configs/allowlist.csv \
  -history-years 9 \
  -concurrency 6
```

Common flags:

| Flag | Purpose | Default |
| --- | --- | --- |
| `-provider` | `wits` or `comtrade` | `wits` |
| `-partners` | Comma-separated partner ISO3 codes | `USA,CHN` |
| `-flows` | Comma-separated flows | `export,import` |
| `-allowlist` | Reporter allowlist CSV; empty disables filtering | `configs/allowlist.csv` |
| `-history-years` | Prior years to fetch for growth calculation | `1` |
| `-concurrency` | Maximum reporter jobs in flight | `6` |
| `-db` | SQLite output path; empty disables persistence | `tradegravity.db` |

### WITS environment variables

- `WITS_BASE_URL` (default `https://wits.worldbank.org/API/V1/`)
- `WITS_API_KEY` (optional)
- `WITS_TRADE_PATH`
- `WITS_RATE_LIMIT_PER_SEC`

### UN Comtrade environment variables

- `COMTRADE_PRIMARY_KEY` (optional; without it, public preview endpoints are used)
- `COMTRADE_SECONDARY_KEY` (optional fallback)
- `COMTRADE_BASE_URL` (default `https://comtradeapi.un.org/`)
- `COMTRADE_DATA_PATH` (default `data/v1/get/{type}/{freq}/{cl}`)
- `COMTRADE_RATE_LIMIT_PER_SEC` (default `2`)
- `COMTRADE_RATE_LIMIT_BURST` (default `2`)
- `COMTRADE_MAX_RETRIES` (default `3`)
- `COMTRADE_REPORTERS_URL`
- `COMTRADE_PARTNERS_URL`

The `matrix` collector omits `partnerCode` to request the partner breakdown. It never treats `partnerCode=0` (World) as a country row. Public preview responses may provide only numeric partner codes; TradeGravity resolves them through the official partner reference and excludes non-alphabetic special aggregates.

### WITS/TRAINS tariff environment variables

- `TRAINS_BASE_URL` (default `https://wits.worldbank.org/API/V1/SDMX/V21/rest/data/DF_WITS_Tariff_TRAINS/`)
- `TRAINS_AVAILABILITY_URL`
- `TRAINS_TIMEOUT_SECONDS` (default `30`)
- `TRAINS_RETRIES` (default `3`)
- `TRAINS_BACKOFF_MILLISECONDS` (default `750`)

The tariff collector resolves WITS numeric reporter and partner codes, selects the latest available tariff year and source nomenclature, and falls back from unavailable AVE-estimated rows to reported rows without relabeling their `data_type`.

Set an optional primary key for the current shell without committing it:

```powershell
$env:COMTRADE_PRIMARY_KEY = "YOUR_KEY"
```

```bash
export COMTRADE_PRIMARY_KEY="YOUR_KEY"
```

This repository reads operating-system environment variables and does not load a `.env` file. For GitHub Actions, store keys as repository secrets named `COMTRADE_PRIMARY_KEY` and, if used, `COMTRADE_SECONDARY_KEY`. Provider transport errors redact request URLs and credentials, but keys should still be rotated immediately if they appear in any external log.

## Generated files and deployment

- Local SQLite database: `tradegravity.db`
- Published JSON: `meta.json`, `catalog.json`, `latest.json`, `series.json`, `quality.json`, `context.json`, `products/`, `strategic-hs6/`, `tariffs/`, `bilateral-matrix/`, and `explanations/` under `site/data/`

Generated data and the local database are intentionally not committed to the default branch. The daily workflow runs the collector and publisher, then deploys `site/` to the `gh-pages` branch.

Before deployment, `cmd/validator` checks provenance across every artifact, reporter uniqueness, periods, non-negative finite values, totals and shares, matrix availability/count identities, tariff rate identities, product keys, context coverage, and explanation evidence references.

## Maintenance and contributing

TradeGravity is created and primarily maintained by [@elecpapaya](https://github.com/elecpapaya). Maintenance includes monitoring scheduled collection runs, reviewing source/API changes, keeping dependencies current, improving tests and documentation, and planning releases.

Issues and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow and [ROADMAP.md](ROADMAP.md) for planned work. The [user-testing protocol](docs/USER_TESTING.md) explains how nonidentifying feedback is collected and turned into follow-up issues. Please do not include API keys or other secrets in issues, logs, or commits.

Support routes are documented in [SUPPORT.md](SUPPORT.md), and the release procedure is documented in [docs/RELEASING.md](docs/RELEASING.md).

Security vulnerabilities should be reported privately according to [SECURITY.md](SECURITY.md). Notable changes are recorded in [CHANGELOG.md](CHANGELOG.md).

## License

Licensed under the [Apache License 2.0](LICENSE).

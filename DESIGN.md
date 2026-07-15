# TradeGravity design

TradeGravity is a reproducible public-data pipeline and static explorer for comparing reporters' trade with the United States and China. The design favors visible provenance, same-period comparison, and deployability without an application server.

## Supported user flow

The reference task is to compare ASEAN reporters for an exact period, select a country, inspect its 5–10 year trend and HS2 product mix, then share and export the evidence. The implementation treats that as one continuous workflow rather than independent demo widgets.

## Architecture

```text
WITS SDMX totals/history ─┐
UN Comtrade HS2 ──────────┼─> collector ─> SQLite ─> publisher ─> static JSON
World Bank country data ──┘       │                       │             │
                                  └─ ingest_runs          └─ explainer ┘
                                                                        │
                                                        HTML/CSS/SVG/JS explorer
```

- `cmd/collector` normalizes WITS totals/history and Comtrade HS2 chapters. Reporter-level concurrency is bounded and provider rate limits remain global.
- `internal/store/sqlite` uses schema-aware idempotent keys and migrates version 1 total-only databases.
- `cmd/context` publishes country labels, region, income, project groups, population, and GDP.
- `cmd/publisher` emits schema 2 totals, time series, product files, quality signals, context-enriched latest rows, and a resource catalog for chunk discovery.
- `cmd/explainer` generates build-time explanations whose statements cite evidence IDs. OpenAI use is optional; deterministic fallback covers every reporter.
- `cmd/validator` rejects internally inconsistent or incompletely grounded artifact sets before deployment.
- `site/` is a static tabbed client. It never receives provider or OpenAI credentials.

## Dashboard sections

The client separates workflows without duplicating data state:

- **Overview** preserves the two-anchor treemap, country snapshot, trend, and evidence-grounded explanation.
- **Intelligence** derives scoped concentration, balance, growth-divergence signals, a two-anchor network, and an accessible ranking from the active filters.
- **Products** loads reporter-partitioned HS2 files and exposes the planned HS6/tariff boundary.
- **Data & Quality** combines the resource catalog, quality report, accessible table, and exports.
- **Scenario Lab** provides an explicitly illustrative constant-elasticity tariff sensitivity calculation. It is not presented as SMART, a causal forecast, or a GDP/welfare model.

The selected reporter and all filters are shared across tabs. The enumerated `tab` query parameter makes a workflow directly shareable and is restored by browser history.

## Growth path for high-volume data

`catalog.json` decouples resource discovery from individual artifact schemas. Every resource identifies grain, readiness, and partitioning. Current product data already uses an index plus reporter chunks; strategic HS6, tariff, bilateral-matrix, reconciliation, value-added, and scenario resources have planned contracts but no fabricated observations or URLs. New high-volume publishers should emit small manifests and bounded chunks rather than append millions of rows to `latest.json`.

## Observation model

An observation is identified by:

```text
(provider, classification, product_code,
 reporter_iso3, partner_iso3, flow, period_type, period)
```

Headline totals use `product_code=TOTAL` and `product_level=0`. HS2 rows have a two-digit product code and level 2. Classification and product identity are part of the database key, so totals and chapters cannot overwrite each other.

Every source value preserves reporter perspective, partner, export/import flow, period type, period, and provider. Trade, combined totals, China share, comparison flags, and normalization are derived values.

## Comparison rules

The default explorer mode requires both partner blocks to exist and match on period type and period. This is recorded as `same_period` and `comparison_period`. The opt-in all-data mode displays source periods and quality warnings; it never imputes or silently aligns values.

The dominant product year is calculated from the latest period of each reporter/partner/flow series. Historical row density is deliberately excluded so a widely available old year cannot become the default.

## Time series

The collector requests the latest point, then a provider-supported year range. Existing keys are filtered before upsert. The publisher keeps up to ten annual years per reporter and marks a point comparable only when USA and China blocks are both available.

Values are nominal current US dollars. Per-capita and GDP-share display modes use the published World Bank denominator and are not constant-price transformations.

## Product provenance

HS2 chapters come from UN Comtrade. The product provider, classification, and level are repeated in metadata, the index, and each reporter file. WITS remains the default headline provider. A provider comparison is published only for matching reporter, partner, flow, period type, and period; a difference is a quality signal, not an automatic correction.

## Data quality

Each collection creates an `ingest_runs` record with request, success, failure, skip, and stored counts plus bounded error messages. Partial runs retain successful observations. Published quality signals include missing partner blocks, mixed periods, stale blocks, run status, and same-period provider deltas.

Transport errors remove request URLs and credentials before logging or persistence. Credentials remain environment variables or GitHub secrets and are never written to static artifacts.

## Evidence-grounded explanations

The explainer constructs a reporter-specific evidence bundle from published totals, period quality, available trend endpoints, and the top product chapter. Optional OpenAI generation uses the Responses API with a strict JSON schema at build time. The output is accepted only when:

- every statement cites one or more known evidence IDs;
- statement count and lengths are bounded;
- numeric tokens occur in the evidence bundle; and
- structured output parses exactly.

Any failure produces deterministic rules output. The viewer shows generator metadata and the evidence table.

## Static explorer state

The query string preserves tab, metric, color, Top N, period, comparison mode, region, income, project group, normalization, reporter search, and selected country. URL parsing accepts only enumerated values and bounded strings. Popstate restores the view.

Filtering affects treemaps, the accessible table, and downloads. CSV is a raw flattened convenience export; filtered JSON records the view state. Machine consumers should prefer the canonical schema 2 artifacts documented in `docs/DATA_SCHEMA.md`.

## Deployment sequence

The scheduled workflow runs tests and vet, builds country context, collects ten-year WITS history, collects HS2 product chapters, publishes JSON, builds explanations, validates every artifact, and deploys `site/` to GitHub Pages.

The first release uses schema 2.0. Breaking field or meaning changes require a schema-version change and release note. Additive fields may ship within 2.x.

## Known limitations

- Upstream APIs can lag, revise data, throttle requests, or omit reporters.
- Public Comtrade preview and authenticated endpoints can have different limits.
- Current values are not inflation-adjusted.
- Product collection currently publishes one selected year per scheduled run.
- Explanations summarize observed evidence and do not infer causes.
- Intelligence concentration covers only the published USA/China partner universe and is not a whole-world concentration measure.
- The scenario lab uses user-supplied elasticity, tariff, and pass-through assumptions; production trade-diversion analysis still requires tariff schedules, substitution parameters, model versioning, and backtests.
- External usability findings must be gathered from real, consented participants; the protocol is in `docs/USER_TESTING.md`.

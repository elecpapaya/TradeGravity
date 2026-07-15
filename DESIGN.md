# TradeGravity design

TradeGravity is a reproducible public-data pipeline and static explorer with an explicit United States–China analytical perspective. Source observations remain neutral and unadjusted; the application organizes them around position, movement, contributors, and evidence limits between the two anchors. The design favors free/public inputs, visible provenance, same-period comparison, and deployability without an application server.

## Supported user flow

The reference task is to compare ASEAN reporters for an exact period, select a country, determine its current USA/China position and direction, inspect the products and evidence that contribute to it, then share and export the result. The implementation treats that as one continuous workflow rather than independent demo widgets.

## Architecture

```text
WITS SDMX totals/history ─────────┐
UN Comtrade annual/monthly/matrix ┼─> collector ─> SQLite ─> publisher ─> static JSON
World Bank country data ──────────┘       │                       │             │
public policy/reference registry ────────────────────────────────┘             │
                                  └─ ingest_runs          └─ explainer ┘
                                                                        │
                                                        HTML/CSS/SVG/JS explorer
```

- `cmd/collector` normalizes WITS totals/history and annual/monthly Comtrade product and partner observations. Reporter-level concurrency is bounded and provider rate limits remain global.
- `internal/store/sqlite` uses schema-aware idempotent keys and migrates version 1 total-only databases.
- `cmd/context` publishes country labels, region, income, project groups, population, and GDP.
- `cmd/publisher` emits schema 2 totals, time series, annual and monthly product files, bilateral matrices, unadjusted mirror diagnostics, quality signals, context-enriched latest rows, and a resource catalog for chunk discovery.
- `cmd/explainer` generates build-time explanations whose statements cite evidence IDs. OpenAI use is optional; deterministic fallback covers every reporter.
- `cmd/validator` rejects internally inconsistent or incompletely grounded artifact sets before deployment.
- `site/` is a static tabbed client. It never receives provider or OpenAI credentials.

## Dashboard sections

The client separates workflows without duplicating data state:

- **Overview** preserves the two-anchor treemap, country snapshot, trend, and evidence-grounded explanation.
- **US–China Lens** derives anchor shares, exposure balance, position shift, dual exposure, growth divergence, a two-anchor network, country ranking, and mirror-reporting checks from the active filters.
- **Chip Lens** applies the same position and direction grammar to stage-mapped annual and focused monthly HS6 observations, while keeping qualitative roles, dated policies, project signals, and transparent stage-shock sensitivity separate. A coverage gate prevents a narrow sample from being described as a global measurement.
- **Products** loads reporter-partitioned HS2 files and exposes the planned HS6/tariff boundary.
- **Data & Quality** combines the resource catalog, quality report, accessible table, and exports.
- **Scenario Lab** provides an explicitly illustrative constant-elasticity tariff sensitivity calculation. It is not presented as SMART, a causal forecast, or a GDP/welfare model.

The selected reporter and all filters are shared across tabs. The enumerated `tab` query parameter makes a workflow directly shareable and is restored by browser history.

## Growth path for high-volume data

`catalog.json` decouples resource discovery from individual artifact schemas. Every resource identifies grain, readiness, and partitioning. Products, strategic HS6, tariffs, bilateral matrices, focused monthly semiconductor observations, and unadjusted mirror diagnostics use small indexes plus bounded chunks. Computed value-added and versioned scenario resources remain planned and have no fabricated observations or URLs. New high-volume publishers should follow the same pattern rather than append millions of rows to `latest.json`.

## Analytical lens

TradeGravity does not claim to be perspective-free. It computes a shared vocabulary for a reporter, product, or semiconductor stage:

```text
USA share        = USA value / (USA value + China value)
China share      = China value / (USA value + China value)
exposure balance = USA share − China share
dual exposure    = 2 × min(USA share, China share)
position shift   = current exposure balance − previous comparable exposure balance
```

Positive balance or shift means toward the USA and negative means toward China. Threshold labels summarize these values but never modify the underlying observations. The metrics are scoped to the two anchors and must not be described as whole-world dependency, political alignment, causal pressure, or a physical supply-chain route.

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

The query string preserves tab, metric, color, Top N, period, comparison mode, region, income, project group, normalization, reporter search, selected country, and Chip Lens stage/country context. URL parsing accepts only enumerated values and bounded strings. Popstate restores the view.

Filtering affects treemaps, the accessible table, and downloads. CSV is a raw flattened convenience export; filtered JSON records the view state. Machine consumers should prefer the canonical schema 2 artifacts documented in `docs/DATA_SCHEMA.md`.

## Deployment sequence

The scheduled workflow runs tests and vet, builds country context, collects ten-year WITS history, annual HS2/strategic HS6 data, a focused 12-month semiconductor panel, tariffs and bilateral matrices, publishes JSON, builds explanations, validates every artifact, and deploys `site/` to GitHub Pages. A main-branch code push reuses the last validated dataset instead of calling upstream APIs.

The first release uses schema 2.0. Breaking field or meaning changes require a schema-version change and release note. Additive fields may ship within 2.x.

## Known limitations

- Upstream APIs can lag, revise data, throttle requests, or omit reporters.
- Public Comtrade preview and authenticated endpoints can have different limits.
- Current values are not inflation-adjusted.
- HS2 collection publishes one selected year per scheduled run; the strategic HS6 collector publishes the selected year plus four prior years for semiconductor trend coverage.
- Focused monthly semiconductor collection covers a bounded 30-code/connector allowlist and at most 36 months; it is a turning-point signal, not a complete market.
- Explanations summarize observed evidence and do not infer causes.
- Intelligence concentration covers only the published USA/China partner universe and is not a whole-world concentration measure.
- Mirror differences can reflect CIF/FOB valuation, timing, classification, partner attribution, re-exports, and revisions. Neither report is treated as truth, and no adjusted estimate is produced.
- The scenario lab uses user-supplied elasticity, tariff, and pass-through assumptions; production trade-diversion analysis still requires tariff schedules, substitution parameters, model versioning, and backtests.
- External usability findings must be gathered from real, consented participants; the protocol is in `docs/USER_TESTING.md`.

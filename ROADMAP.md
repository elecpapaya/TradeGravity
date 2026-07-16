# TradeGravity roadmap

TradeGravity is maintained as a small, inspectable public-data pipeline with an explicit US–China analytical lens. The roadmap prioritizes data trust, repeatability, free/public inputs, and accessibility before expanding the number of features.

## Current maintenance commitments

- Run the staggered collection and GitHub Pages deployment workflows weekly, with manual refreshes available for urgent source updates.
- Investigate failed scheduled runs and upstream schema changes.
- Review dependencies, documentation, and security alerts at least monthly.
- Keep source attribution, reporting periods, and transformation rules visible.
- Triage reproducible bug reports and contributor pull requests as capacity allows.

## Completed foundations

- Add unit coverage for provider parsing, SQLite upserts, and publisher calculations.
- Publish coverage, source, pipeline-refresh, and observation-period indicators.
- Document the static JSON schema and compatibility expectations.
- Validate generated datasets before deployment.
- Add keyboard-operable treemap tiles, visible focus states, and safe rendering helpers.
- Add a searchable accessible table, spreadsheet-safe CSV export, citation metadata, and release procedure.
- Provide a CI-validated synthetic sample dataset for network-free development.
- Default to same-period USA/China comparisons and expose mixed, missing, and stale observations.
- Publish up to ten annual periods per reporter and render selected-country trends.
- Collect and publish HS2 product chapters from UN Comtrade without mixing their provenance into WITS totals.
- Add World Bank region, income, population, and GDP context with regional/group filters and normalization.
- Preserve period, metric, Top N, filters, normalization, query, and selected reporter in shareable URLs.
- Publish a quality dashboard with collection runs and same-period provider comparisons.
- Generate evidence-grounded explanations at build time with citation and numeric-claim validation.
- Add filtered JSON export and a deterministic sample-data generator.
- Split the viewer into synchronized Overview, US–China Lens, Chip Lens, Products, Data & Quality, and Scenario Lab tabs.
- Publish curated strategic HS6 trade partitions with source-revision metadata.
- Collect and publish WITS/TRAINS strategic-HS6 tariffs with AVE/reported identity and fallback behavior.
- Publish reported UN Comtrade multi-partner matrices and selected-country partner networks without route claims.
- Connect published MFN tariffs and HS6 import baselines to the transparent scenario sensitivity lab.
- Publish the reviewed [`v0.1.0` release](https://github.com/elecpapaya/TradeGravity/releases/tag/v0.1.0) using the documented checklist.
- Add a first-visit guide, in-context definitions and limits, PNG/CSV/summary exports, explicit data-health and multi-clock notices, small-screen layout improvements, and restorable scenario URLs.
- Add a specialist US–China Chip Supply Chain Lens with an eight-stage taxonomy, 30 mapped HS6 codes, five-year strategic collection, country-role matrix, policy and capacity/project evidence registers, observed-distribution metrics, explicit coverage gates, stage CSV export, and a transparent disruption/substitution sensitivity.
- Publish USA/China exposure balance, position shift, and dual-exposure metrics with explicit formulas and non-alignment caveats across the Intelligence and Chip Lens views.
- Collect and publish a focused 12-month, 30-code semiconductor turning-point panel for selected connector economies.
- Publish a validated semiconductor Pulse with latest-month movement, previous-publication coverage/value changes, a bounded machine-readable change feed, and evidence endpoints in the Markdown report.
- Publish unadjusted mirror-reporting diagnostics against USA and China counterpart reports without selecting a ground truth or claiming fraud, rerouting, or reconciliation.
- Register only free/public semiconductor evidence layers, including OECD ICIO as lagged industry context, and validate that paid/proprietary sources cannot become required metric inputs.

## Near term: adoption evidence and release operations

- [Add browser end-to-end coverage for the documented ASEAN task](https://github.com/elecpapaya/TradeGravity/issues/8), including narrow and desktop viewports.
- Run the documented task with at least three students, researchers, or developers and publish consented findings through [the public study tracker](https://github.com/elecpapaya/TradeGravity/issues/3).
- Track task completion, interpretation errors, and time-to-answer in [`docs/USER_TESTING.md`](docs/USER_TESTING.md); never substitute synthetic sessions for real participants.
- Publish and maintain the reproducible ASEAN example notebook.

## Next: usability and analytical depth

- Expand the semiconductor coverage gate from the current publication sample to at least 15 observed reporters and five annual periods, then monitor annual and monthly coverage drift in scheduled runs.
- Evaluate only free, public, and reproducible fab/project evidence before publishing any facility layer; keep announcements, awards, construction, expected operation, and operating output separate.
- Add computed OECD ICIO value-added/propagation context only after its industry aggregation, release lag, mapping, uncertainty, and validation cases are documented. Never manufacture an HS6-level value-added result from industry-level tables.
- Add constant-price or deflator-aware normalization; current values are nominal.
- [Add product-share changes across multiple years](https://github.com/elecpapaya/TradeGravity/issues/12) when provider quotas permit reliable collection.
- [Improve chart keyboard navigation and screen-reader summaries](https://github.com/elecpapaya/TradeGravity/issues/9); continue testing small-screen layouts.
- [Add automated magnitude and cross-file schema anomaly checks](https://github.com/elecpapaya/TradeGravity/issues/10).
- Extend the published mirror diagnostics with optional revision-aware time series and documented CIF/FOB sensitivity; retain both source reports and never silently replace them with an adjusted truth.
- Prototype ESI/ECI/ICI/SPDI/RPI only after formulas, benchmark datasets, uncertainty, and validation cases are documented.
- Add tariff-change decomposition and Marimekko/waterfall views when the required multi-year product coverage is reliable.
- [Add versioned scenario manifests and reproducible result artifacts](https://github.com/elecpapaya/TradeGravity/issues/13) before introducing SMART-like substitution or welfare outputs.

## Later: provider resilience and interpretation

- Add configurable provider fallback without mixing provenance silently.
- Extend the implemented semiconductor publication-change feed to headline, tariff, and matrix artifacts after each layer has a stable comparison key and revision policy.
- Expand grounded explanations after external evaluation confirms they reduce rather than increase interpretation errors.

Priorities may change when upstream APIs change or users report higher-impact needs. Roadmap discussion should happen in a GitHub issue so decisions remain public and reviewable.

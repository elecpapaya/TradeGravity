# TradeGravity roadmap

TradeGravity is maintained as a small, inspectable public-data pipeline. The roadmap prioritizes data trust, repeatability, and accessibility before expanding the number of features.

## Current maintenance commitments

- Run the collection and GitHub Pages deployment workflow daily.
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

## Near term: adoption evidence and release operations

- Expand integration coverage for collection and end-to-end publishing failures.
- Publish the first tagged release using the documented checklist.
- Run the documented task with at least three students, researchers, or developers and publish consented findings as GitHub issues.
- Track task completion, interpretation errors, and time-to-answer in `docs/USER_TESTING.md`.
- Publish and maintain the reproducible ASEAN example notebook.

## Next: usability and analytical depth

- Add constant-price or deflator-aware normalization; current values are nominal.
- Add product-share changes across multiple years when provider quotas permit reliable collection.
- Improve chart keyboard navigation, screen-reader summaries, and small-screen layouts.
- Add automated magnitude and schema anomaly checks.

## Later: provider resilience and interpretation

- Add configurable provider fallback without mixing provenance silently.
- Track data lag and coverage changes across scheduled runs, not only within the latest artifact.
- Expand grounded explanations after external evaluation confirms they reduce rather than increase interpretation errors.

Priorities may change when upstream APIs change or users report higher-impact needs. Roadmap discussion should happen in a GitHub issue so decisions remain public and reviewable.

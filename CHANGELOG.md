# Changelog

All notable changes to TradeGravity will be documented in this file. The project follows [Semantic Versioning](https://semver.org/) once tagged releases begin.

## Unreleased

### Added

- A first-visit 30-second onboarding guide with a ready-made Viet Nam view.
- Global PNG snapshot, CSV, and Markdown summary-report exports for the active analysis view.
- Always-visible metric, observation-period, scope, and limitation context plus a definitions dialog.
- Current, partial, and degraded publication-health states with recovery guidance.

### Changed

- Recent headline panels now state their rolling window beside the selected historical trade periods and explicitly warn that the clocks are not aligned.
- Share URLs now restore scenario partner, HS6 product, tariff, elasticity, and pass-through assumptions in addition to the existing filters and country selection.
- Narrow layouts use compact context, stacked cards, bounded tooltips, and touch-friendly export and dialog controls.

## [0.1.1] - 2026-07-16

### Added

- A real-dashboard README screenshot and a preconfigured 30-second ASEAN/Viet Nam trial path.
- A reusable, privacy-preserving external user-study recruitment kit.
- Public `v0.2.0` roadmap issues with scoped acceptance criteria.
- Deterministic browser tests for trade-focused news relevance, recency, deduplication, and safe links.

### Changed

- The optional GDELT panel now requests trade and supply-chain topics within a 14-day window, rejects irrelevant or stale titles, removes duplicate headlines, and displays its experimental scope and limitations.
- Main-branch Pages deployment reuses the last published dataset instead of calling upstream providers on every code push.
- Dashboard controls, tabs, treemaps, labels, and focus styling adapt more reliably to desktop and narrow layouts.
- Updated `modernc.org/sqlite` from 1.29.10 to 1.53.0.

## [0.1.0] - 2026-07-15

### Added

- Dataset metadata with provider, schema version, coverage, and observation-period counts.
- A validator that blocks deployment of malformed or internally inconsistent datasets.
- Unit and integration tests for publishing, provider parsing, persistence, and browser security helpers.
- Quality, CodeQL, and dependency-update automation.
- Go 1.25.12 as the minimum toolchain to include required standard-library security fixes.
- Contributor guidance, roadmap, security policy, and structured issue templates.
- Keyboard-accessible treemap tiles and visible data freshness information.
- Searchable accessible data table and spreadsheet-safe filtered CSV export.
- Citation metadata, support routes, and a repeatable release checklist.
- CI-validated synthetic sample data for network-free viewer development.
- Reuse examples and provider-specific data-rights and attribution guidance.
- Same-period comparison mode, exact-period selection, and up to ten annual periods per reporter.
- HS2 product collection/publication, region and income context, ASEAN/EU grouping, and per-capita/GDP-share views.
- Shareable explorer URLs, filtered JSON export, and selected-country trend and product panels.
- Collection-run and provider-delta quality reporting.
- Evidence-grounded build-time explanations with strict structured output validation and deterministic fallback.
- A reproducible ASEAN notebook, deterministic schema 2 sample generator, and external user-testing protocol.

### Security

- Escaped third-party content before HTML rendering and restricted article links to HTTPS.
- Added a restrictive Content Security Policy.
- Pinned D3 to an exact version with Subresource Integrity verification.
- Redacted credential-bearing request URLs and provider keys from transport and response errors.

### Changed

- Publisher output is deterministic by reporter ISO3.
- Daily deployment now runs tests, vet, and dataset validation before publishing.
- The daily deployment now collects ten-year WITS history, throttled HS2 chapters, country context, and explanations before validation.
- Published artifacts use schema version 2.0 and keep headline and product providers explicitly separate.

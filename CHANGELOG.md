# Changelog

All notable changes to TradeGravity will be documented in this file. The project follows [Semantic Versioning](https://semver.org/) once tagged releases begin.

## Unreleased

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

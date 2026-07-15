# Changelog

All notable changes to TradeGravity will be documented in this file. The project follows [Semantic Versioning](https://semver.org/) once tagged releases begin.

## Unreleased

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

### Security

- Escaped third-party content before HTML rendering and restricted article links to HTTPS.
- Added a restrictive Content Security Policy.
- Pinned D3 to an exact version with Subresource Integrity verification.

### Changed

- Publisher output is deterministic by reporter ISO3.
- Daily deployment now runs tests, vet, and dataset validation before publishing.

# Changelog

All notable changes to TradeGravity will be documented in this file. The project follows [Semantic Versioning](https://semver.org/) once tagged releases begin.

## Unreleased

### Added

- A specialist US–China Chip Supply Chain Lens covering design/IP, materials, equipment, logic/foundry, memory/HBM, discrete/power, packaging/test, and downstream demand.
- A validated 30-code semiconductor stage registry, country-role matrix, current trend and official policy timeline, bounded capacity/project evidence register, and explicit research-coverage gate.
- Observed-sample HHI and top-three shares, stage/country URL state, stage CSV export, and a disruption/substitution sensitivity that never claims capacity or causal effects.
- Five-year strategic HS6 collection support and a published `semiconductors/reference.json` artifact with cross-registry and source validation.
- Explicit USA/China shares, exposure balance, position shift, dual exposure, and growth-divergence signals across the general and semiconductor lenses.
- Focused 12-month semiconductor HS6 collection/publication for selected connector economies, with missing coverage left unestimated.
- A validated `changes.json` feed and Chip Lens Pulse that distinguish latest month-to-month movement from publish-to-publish coverage, row, and value revisions.
- Semiconductor-aware Markdown summary reports with annual position, latest monthly growth, publication deltas, and direct evidence endpoints.
- Unadjusted USA/China mirror-reporting diagnostics that preserve both reports, symmetric gaps, and interpretation caveats.
- A validated free/public-only semiconductor data policy and open-dataset register, including OECD ICIO as industry-level future context rather than fabricated HS6 evidence.
- A first-visit 30-second onboarding guide with a ready-made Viet Nam view.
- Global PNG snapshot, CSV, and Markdown summary-report exports for the active analysis view.
- Always-visible metric, observation-period, scope, and limitation context plus a definitions dialog.
- Current, partial, and degraded publication-health states with recovery guidance.
- A validated `briefing.json` contract that derives three cited semiconductor observations and exposes review-gated email Markdown and 4:5 social-carousel copy without collecting subscribers or publishing automatically.
- An offline distribution-kit CLI and manual read-only Actions workflow that render one-primary-CTA email HTML with an unsubscribe placeholder, six matched 1080×1350 SVG originals and PNG upload assets, alt text, approval gates, and deterministic file hashes without sending or posting.
- A second original `editorial-light` native carousel theme behind the same validated renderer interface; theme choice is recorded in the manifest and therefore bound to editorial approval, with no browser runtime, remote font, image fetch, or arbitrary HTML input.
- Automated palette contrast checks across every gradient stop, including normal text and large bold role labels; the light theme's muted and accent colors were tightened to retain the documented floor.
- A review-pending Instagram `caption.md` derived from the same three validated signals, with comparison period, evidence link, conservative scope note, restrained tags, an editorial length ceiling, and manifest/approval tamper protection.
- An Instagram manual-publish preflight CLI that requires channel approval, verifies every PNG plus caption and alt-text contracts, emits content-free aggregate evidence outside the kit, and explicitly carries no credentials or publish authorization.
- A content-release approval CLI that rejects changed, missing, or untracked kit files and binds the verified manifest to a reviewer, audience label, time, and email/Instagram channel set without claiming delivery readiness.
- A local email preflight CLI that validates double opt-in, audience identity, suppression precedence, timestamps, duplicate addresses, unique opaque HTTPS unsubscribe URLs, and a pilot ceiling while emitting an aggregate plan with no recipient addresses or tokens and no delivery authorization.
- A private SQLite subscription-registry CLI and subscription HTTP service with a default-off double-opt-in signup form, short-lived purpose-separated HMAC confirmation links, read-only scanner-safe GET plus explicit confirmation POST, stable Resend confirmation idempotency, form-encoded one-click unsubscribe, signed raw-body provider feedback, durable global suppression, security headers, and private preflight exports.
- A short-lived email launch-approval contract and Resend pilot CLI that replay consent/suppression checks at send time, render recipient-specific visible and RFC one-click unsubscribe links, use one recipient and one provider idempotency key per request, and prevent automatic duplicate or uncertain retries with a private HMAC-keyed SQLite delivery ledger. A separate reconciliation CLI records provider-confirmed acceptance or non-acceptance without storing recipient PII; only the latter plus a different launch-authorization digest permits retry.

### Changed

- The provider refresh runs weekly and is split into a core workflow and a quota-spaced semiconductor workflow; annual strategic HS6 collection is bounded to the declared connector allowlist instead of querying every headline reporter.
- WITS/TRAINS requests now declare annual frequency and split large strategic HS6 registries into server-safe 20-code batches.
- The scheduled strategic HS6 collection now requests the selected year plus four prior annual periods and a focused 12-month panel; main-branch fast deploys preserve the latest observed coverage block while refreshing the versioned semiconductor reference.
- The semiconductor publisher now compares against the previous `gh-pages` monthly dataset; the first comparable release is marked as a baseline rather than “unchanged.”
- Intelligence and semiconductor navigation now state the explicit US–China perspective while keeping reported observations, context, and illustrative estimates separate.
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

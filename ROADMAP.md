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

## Near term: reliability and transparency

- Expand integration coverage for collection and end-to-end publishing failures.
- Surface partial collection failures without discarding successful observations.
- Publish the first tagged release using the documented checklist.

## Next: usability and reuse

- Improve keyboard navigation, focus states, and small-screen layouts.
- Add a downloadable filtered JSON view.
- Add regional filtering and optional grouping.
- Provide small sample fixtures for offline development and documentation.

## Later: provider resilience

- Compare WITS and UN Comtrade coverage and document known differences.
- Add configurable provider fallback without mixing provenance silently.
- Track data lag and coverage changes over time.
- Add automated anomaly checks for unexpected schema or magnitude changes.

Priorities may change when upstream APIs change or users report higher-impact needs. Roadmap discussion should happen in a GitHub issue so decisions remain public and reviewable.

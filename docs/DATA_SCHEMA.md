# Published data schema 2.0

TradeGravity publishes one versioned artifact set under `site/data/`. Headline, product, strategic HS6, semiconductor, publish-to-publish change, distribution briefing, tariff, bilateral-matrix, mirror-diagnostic, quality, and explanation artifacts share the trade publication timestamp. The semiconductor reference has its own additive schema and records both its editorial update date and publisher `generated_at`. `catalog.json`, `changes.json`, and `briefing.json` have independent additive schemas and the same publisher timestamp. `context.json` has its own refresh time because it is built before the trade publisher.

## Time and comparison semantics

`generated_at` is the UTC pipeline time, not an observation date. Every trade observation retains `period_type` (`Y`, `Q`, or `M`) and `period`.

`same_period` is true only when both USA and China partner blocks exist and use the same period type and value. `comparison_period` is populated only for such rows. The viewer defaults to these rows. Opting into all available values never changes or fills the source periods.

## Artifact map

| Artifact | Purpose | Primary provenance |
| --- | --- | --- |
| `meta.json` | Counts, dominant period, coverage, feature availability | Publisher |
| `latest.json` | Latest reporter/partner totals, context, growth, comparability | WITS by default |
| `series.json` | Up to ten years per reporter | Same provider as `latest.json` |
| `context.json` | Region, income, groups, population, GDP | World Bank + project groups |
| `products/index.json` | Product-file discovery and classification | UN Comtrade |
| `products/{ISO3}.json` | HS2 chapters by reporter and period | UN Comtrade |
| `strategic-hs6/index.json` | Curated code registry and partition discovery | UN Comtrade + project registry |
| `strategic-hs6/{ISO3}/{YEAR}.json` | USA/China strategic HS6 flows | UN Comtrade |
| `semiconductors/reference.json` | Stage taxonomy, roles, trends, policy/project signals, sources, coverage gate | Project registry + cited official/intergovernmental sources |
| `semiconductors/monthly/index.json` | Focused monthly reporter/period discovery | UN Comtrade + semiconductor registry |
| `semiconductors/monthly/{ISO3}.json` | Selected HS6 monthly USA/China flows | UN Comtrade |
| `changes.json` | Previous-publication coverage, row, and value deltas for the focused monthly semiconductor layer | Publisher comparison of consecutive publications |
| `briefing.json` | Review-gated email and 4:5 carousel drafts derived from three cited monthly semiconductor signals | Deterministic publisher calculations over the focused monthly layer |
| `tariffs/index.json` | Importer/year tariff partition discovery | WITS/TRAINS |
| `tariffs/{ISO3}/{YEAR}.json` | Revision-aware strategic HS6 tariff rows | WITS/TRAINS |
| `bilateral-matrix/index.json` | Multi-partner `TOTAL` partition discovery | UN Comtrade |
| `bilateral-matrix/{ISO3}/{YEAR}.json` | Reported partner exports/imports and availability | UN Comtrade |
| `mirror/index.json` | Unadjusted mirror-diagnostic partition discovery | Derived from bilateral matrices |
| `mirror/{ISO3}/{YEAR}.json` | Reporter/USA/China counterpart gaps | Derived from both reporters' UN Comtrade totals |
| `quality.json` | Missing/stale data, collection runs, provider comparisons | Pipeline calculations |
| `catalog.json` | Resource discovery, grain, partitioning, and readiness | Publisher |
| `explanations/index.json` | Explanation coverage and generator counts | Explainer |
| `explanations/{ISO3}.json` | Claims with exact evidence IDs | Published JSON evidence |

## `catalog.json`

The catalog is the stable discovery layer for a dashboard that may grow beyond a few single-file datasets. Each resource declares an `id`, display title, `status`, analytical `grain`, `partitioning`, and an `href` only when an artifact is published. Current statuses are `ready`, `partial`, and `planned`.

`strategic_hs6`, `semiconductor_atlas`, `semiconductor_monthly`, `publication_changes`, `distribution_briefing`, `tariff_schedules`, `bilateral_matrix`, and `mirror_reconciliation` are published resources. The last ID is retained for catalog compatibility, but its title and artifact explicitly describe **unadjusted mirror-reporting diagnostics**, not a reconciled truth. Computed value-added networks and versioned scenario results remain planned contracts and do not claim that those observations exist. Published resources use relative same-origin paths; the validator rejects duplicate IDs, invalid statuses, unsafe paths, and metadata that conflicts with `meta.json`.

The current product resource demonstrates the intended scaling pattern: a small discovery index plus one reporter file. Higher-volume resources should use period, reporter, importer, industry, or sector chunks named in the catalog rather than expanding `latest.json`.

## `meta.json`

Important fields include. The values below illustrate the schema shape; consumers must read the active publication rather than treat these counts or timestamps as the latest dataset:

```json
{
  "schema_version": "2.0",
  "generated_at": "2026-07-15T12:00:00Z",
  "provider": "wits",
  "partners": ["USA", "CHN"],
  "reporter_count": 51,
  "dominant_period": "Y:2023",
  "comparable_reporters": 48,
  "incomparable_reporters": 3,
  "stale_partner_blocks": 6,
  "series_reporter_count": 51,
  "series_point_count": 500,
  "product_provider": "comtrade",
  "product_classification": "HS",
  "product_level": 2,
  "product_reporter_count": 49,
  "context_status": "success",
  "strategic_partition_count": 49,
  "tariff_importer_count": 5,
  "matrix_reporter_count": 49,
  "matrix_partner_row_count": 10586,
  "mirror_provider": "comtrade",
  "mirror_reporter_count": 47,
  "mirror_partition_count": 47,
  "mirror_comparison_count": 188,
  "semiconductor_status": "limited",
  "semiconductor_code_count": 30,
  "semiconductor_reporter_count": 1,
  "semiconductor_period_count": 1,
  "semiconductor_monthly_provider": "comtrade",
  "semiconductor_monthly_reporter_count": 12,
  "semiconductor_monthly_period_count": 12,
  "semiconductor_monthly_observation_count": 17280
}
```

Coverage fields retain their version 1 meanings. `dominant_period` is the most common period among latest partner blocks, not the most common period in historical storage.

## `latest.json`

Each reporter row contains source values plus context:

```json
{
  "iso3": "KOR",
  "iso2": "KR",
  "name": "Korea, Rep.",
  "region": "East Asia & Pacific",
  "income_group": "High income",
  "groups": [],
  "population": {"value": 51700000, "year": "2023"},
  "gdp": {"value": 1.71e12, "year": "2023"},
  "usa": {
    "period": "2023",
    "period_type": "Y",
    "prev_period": "2022",
    "export": 123,
    "import": 456,
    "trade": 579,
    "growth": {"export": 0.12, "import": -0.04, "trade": 0.05},
    "growth_basis": "yoy"
  },
  "chn": {
    "period": "2023",
    "period_type": "Y",
    "export": 200,
    "import": 300,
    "trade": 500
  },
  "total": 1079,
  "share_cn": 0.4634,
  "same_period": true,
  "comparison_period": "2023"
}
```

Calculations are `trade = export + import`, `total = usa.trade + chn.trade`, and `share_cn = chn.trade / total` when total is positive. Growth is `(current - previous) / previous` and is omitted when the prior comparable value is unavailable or zero.

## `series.json`

`rows` contains `{iso3, points}`. A point includes `period_type`, `period`, USA and China blocks with an `available` flag, `total`, `share_cn`, and `comparable`. Points are chronological and limited to the configured annual window (ten years by default). Missing partner values remain zero with `available: false` and must not be imputed.

## Country context and normalization

`context.json` records a status of `success` or `partial`, upstream errors, and country records. Population and GDP are `{value, year}` pairs. The viewer's per-capita and GDP-share modes divide nominal trade values by these published denominators. They do not produce constant-price series; the UI states that limitation.

Project groups such as `ASEAN` and `EU` come from `configs/countries.csv`; region and income labels come from the World Bank.

## HS2 products

The product index identifies provider, classification, level, periods, and available reporters. Each reporter file contains product rows keyed by period and two-digit code. USA/China blocks have export, import, trade, and availability fields.

Product data is never substituted for the WITS headline series. `quality.json` may compare the summed HS2 value with a WITS total only when reporter, partner, flow, period type, and period are identical.

## Strategic HS6 and tariffs

`strategic-hs6/index.json` publishes the curated code descriptors, sectors, available reporters/periods, and reporter/year partition links. Each row retains the source classification because a six-digit code must not be assumed equivalent across every HS revision.

`tariffs/index.json` discovers importer/year partitions and enumerates data types and rate types. A tariff row includes source classification and nomenclature, exporter/regime, `reported` or `ave_estimated`, rate identity, average/minimum/maximum rates when supplied, line counts, exclusions, and the source update timestamp. The UI prefers World MFN AVE rows only as an explicit display rule; it does not merge them with preferential rates.

## Semiconductor reference and US–China metrics

`semiconductors/reference.json` uses schema `1.0` and contains `perspective`, `data_policy`, `open_datasets`, `stages`, `country_roles`, `trends`, `policy_events`, `capacity_signals`, `sources`, `scenario_defaults`, and a publisher-added `publication` block. The perspective must be `us_china` with anchors `USA` and `CHN`; the data policy must be `free_public_only`. Dataset and dated-context entries declare access/reuse notes, and the validator rejects paid/proprietary access types. Stage codes must exist in the strategic registry. Trend, policy, and capacity records must resolve to a declared HTTPS source; policy and capacity stages must resolve to a declared stage.

The publication block records `status`, registered-code count, observed reporters/periods/rows, minimum coverage targets, measurement scope, and the concrete observed dimension values. `research_ready` requires at least 30 mapped codes, 15 observed reporters, and five annual periods. This is a descriptive coverage gate, not a claim of production-capacity or physical-route coverage. See [SEMICONDUCTOR_ATLAS.md](SEMICONDUCTOR_ATLAS.md) for formulas and interpretation boundaries.

The browser derives the explicit two-anchor metrics without changing published values:

```text
exposure_balance = usa_share − china_share
dual_exposure = 2 × min(usa_share, china_share)
position_shift = current_exposure_balance − previous_exposure_balance
```

Positive balance/shift means toward USA and negative means toward China. These client-derived fields are not whole-world shares or political-alignment scores.

## Focused monthly semiconductor observations

`semiconductors/monthly/index.json` declares provider, level 6, anchors, sorted reporters/periods, partition metadata, raw source-observation count, and scope. Each reporter file contains one aggregated row per `period × classification × code`:

```json
{
  "period": "2026-05",
  "classification": "H6",
  "code": "854232",
  "label": "Memories",
  "usa": {"available": true, "export": 10, "import": 5, "trade": 15},
  "chn": {"available": true, "export": 12, "import": 8, "trade": 20},
  "total": 35,
  "share_cn": 0.5714285714
}
```

Only mapped HS6 codes, monthly periods, and USA/CHN partners are admitted. The index observation count is the number of accepted source flow rows, while partition `row_count` is the number of aggregated product-month rows. Missing data remain unavailable and are never interpolated.

## Publish-to-publish semiconductor changes

`changes.json` uses schema `1.0` and compares the current focused monthly publication with the previously deployed `semiconductors/monthly/` dataset. This is deliberately separate from economic month-to-month movement. The comparison key is `reporter × period × classification × HS6`; matching keys are revisions when either anchor series block, total, share, or label changed.

```json
{
  "schema_version": "1.0",
  "generated_at": "2026-07-16T02:30:00Z",
  "previous_generated_at": "2026-07-09T02:30:00Z",
  "status": "changed",
  "summary": {
    "current_observation_count": 1200,
    "previous_observation_count": 1100,
    "observation_delta": 100,
    "added_rows": 25,
    "removed_rows": 0,
    "revised_rows": 3
  },
  "new_periods": ["2026-06"],
  "new_reporters": [],
  "top_revisions": []
}
```

`status` is `baseline` when no earlier comparable publication exists, `unchanged` when a previous publication exists but no admitted change is found, and `changed` otherwise. A baseline never masquerades as “no change.” The revision list is capped at 20 rows and ordered by the sum of absolute USA and China trade-value changes; global counts remain complete even when the list is truncated. Added and removed rows are counted but not represented as value revisions. The validator checks current-index identity, timestamps, dimensions, counts, finite values, revision arithmetic, and descending magnitude.

## Reviewed distribution briefing

`briefing.json` uses schema `1.0` and converts the focused monthly semiconductor observations into one inspectable distribution contract. It contains exactly three deterministic signals when two comparable months exist: the largest absolute reporter total change, the largest absolute two-anchor China-share shift, and the largest absolute HS6 product change. Each signal retains current and previous USA/China values, arithmetic deltas, periods, interpretation limits, and relative evidence paths.

The same signals feed two non-publishing draft formats:

- `email` contains subject, preview, Markdown body, evidence CTA, `send_policy: "manual_review_required"`, and a primary goal;
- `social_carousel` contains six cited `4:5` copy slides with `review_policy: "manual_review_required"` in cover, scale, anchor-balance, product, method, and CTA order.

The top-level `review_required` must remain `true`. A ready artifact is rejected if either channel permits automatic publication. If two comparable months are unavailable, the artifact fails closed with `status: "unavailable"`, no signals, and no sendable copy. The static application can download drafts and copy the evidence link; it does not maintain a subscriber database, send mail, or call a social publishing API. The offline distributor can render matched SVG and 1080×1350 PNG drafts from the validated slide model, but its manifest keeps both send and social authorization false.

An optional local `approval.json` schema `1.0` is created only after the distributor verifies the exact manifest file set, byte counts, and SHA-256 digests. The manifest's carousel record includes the selected native `theme` (`intelligence-dark` or `editorial-light`) and fixed `caption_path: "carousel/caption.md"`. The caption is derived from the validated signals with period, evidence entry point, scope note, restrained tags, and a 1,800-rune project ceiling. A theme, caption, or image change alters hashes and invalidates an earlier approval. The approval records `scope: "content_release"`, the edition and manifest digest, reviewer, non-sensitive audience label, canonical UTC approval time, sorted channels, and fixed review attestations; automatic publishing remains false.

An optional external `instagram-preflight.json` schema `1.0` binds an Instagram-approved manifest and approval digest to the theme, six-slide count, 1080×1350 dimensions, caption rune/hashtag counts, alt-text section count, and boolean integrity checks. It contains no caption text, hashtags, evidence URL, credentials, or account identity. `manual_upload_required` is true and `automatic_publish_authorized` is false. It must remain outside the approved kit; see [INSTAGRAM_PREFLIGHT.md](INSTAGRAM_PREFLIGHT.md).

An optional local `delivery-preflight.json` schema `1.0` can then bind an email-approved edition to the SHA-256 digests of private subscriber and suppression CSVs. It records only aggregate consented, suppressed, suppression-row, and eligible counts; the approved audience label; the template, manifest, and approval digests; an explicit pilot ceiling; required unsubscribe headers and DKIM coverage; and whether individual opaque HTTPS unsubscribe URLs passed validation. It contains no recipient addresses, unsubscribe URLs, tokens, or local file paths. `provider_configured` and `delivery_authorized` remain false. The private CSV schemas and operational limits are documented in [EMAIL_DELIVERY_PREFLIGHT.md](EMAIL_DELIVERY_PREFLIGHT.md); neither the CSVs nor this local plan belongs under `site/data/`.

The optional private subscription registry is deliberately outside the published schema. Its SQLite tables keep random subscription and pending-confirmation IDs, normalized email, audience, active/suppressed state, double-opt-in evidence, confirmation expiry/dispatch state, privacy-notice version, suppression reason/time, provider-feedback event IDs, and global bounced/complaint/provider-suppression state. Purpose-separated HMAC token payloads contain only a version and random ID. Confirmation GET is read-only and explicit POST records consent; globally suppressed addresses cannot receive confirmation or become active. Database files, WAL/SHM files, source/export CSVs, tokens, and secrets must never be published. See [UNSUBSCRIBE_SERVICE.md](UNSUBSCRIBE_SERVICE.md).

An optional local `email-launch-authorization.json` schema `1.0` binds one Resend pilot to the exact preflight digest, edition, audience, sender, content/input digests, eligible count, operator, attestations, and a validity window of no more than one hour. It contains no recipient address or unsubscribe token. The accompanying private delivery-ledger SQLite schema stores only HMAC-derived recipient and delivery keys, aggregate labels, content/idempotency and authorization identifiers, provider message IDs, statuses, timestamps, and bounded non-sensitive reconciliation labels. Accepted and unresolved-pending rows both prevent an automatic resend. A provider-confirmed `not_accepted` resolution can transition back to a pending attempt only when a different launch-authorization digest is supplied later; the audit row and stable provider idempotency key are retained. Neither artifact is published, and neither changes the public `catalog.json`; see [EMAIL_PROVIDER_PILOT.md](EMAIL_PROVIDER_PILOT.md).

## Multi-partner bilateral matrix

The matrix index has `product_code: "TOTAL"`, `product_level: 0`, sorted reporter/partner/period dimensions, partition counts, partner-row counts, and source-observation counts. Each reporter/year file contains one row per alphabetic ISO3 partner:

```json
{
  "partner_iso3": "USA",
  "export_available": true,
  "import_available": true,
  "export_usd": 100,
  "import_usd": 80,
  "trade_usd": 180,
  "balance_usd": 20
}
```

`trade_usd = export_usd + import_usd` and `balance_usd = export_usd - import_usd`. Availability flags distinguish a missing flow from a reported zero. World (`partnerCode=0`), regional groups, non-alphabetic special codes, and the reporter itself are excluded. These rows are reported bilateral totals, not shipment legs, firm relationships, value-added origin, or proof of rerouting.

## Mirror-reporting diagnostics

`mirror/index.json` declares the fixed anchors `USA` and `CHN`, sorted reporter/year partitions, and the number of available flow-pair comparisons. A row in `mirror/{ISO3}/{YEAR}.json` pairs:

- reporter exports with anchor-reported imports; and
- reporter imports with anchor-reported exports.

For each available pair, `gap_usd = reporter_value − anchor_value` and `symmetric_gap_ratio = gap_usd / ((reporter_value + anchor_value) / 2)`. A zero denominator yields no ratio. Each file carries scope and caveats; neither reporter is ground truth. The artifact is not an adjusted series and cannot support a fraud, evasion, transshipment, or physical-route claim.

## Quality and collection runs

`quality.json` contains summary counts, reporter issue codes, recent collection runs, and same-period provider comparisons. Run status is `success`, `partial`, or `failed`; successful observations remain published even when other requests fail. Provider deltas are ratios `(secondary - primary) / primary`, not corrections.

## Evidence-grounded explanations

Each explanation has generator metadata, a summary, two to six statements, and evidence records. Every statement lists one or more `evidence_ids`. Evidence records include label, display value, period, source, and source JSON path.

When the optional OpenAI build credential is present, structured output is requested at build time. Unknown evidence IDs or unsupported numeric tokens cause deterministic fallback. Without a credential, all reporters receive the same citation-preserving rules output. No browser artifact contains an API key.

## Compatibility and validation

Additive fields may be introduced within schema 2.x. Removing fields, changing types, or changing meanings requires a schema-version change and release note. Consumers should ignore unknown fields and enforce their own missing-data policy.

Run the same validation used before deployment:

```bash
go run ./cmd/validator -dir site/data -min-reporters 40
```

The validator checks cross-file provenance and counts, reporter and period uniqueness, finite numbers, calculated totals/shares/balances, monthly product identities, briefing signal arithmetic and review gates, mirror-pair arithmetic and disclosure, flow-availability identities, strategic registry membership, free/public reference policy, tariff rate identities, catalog contracts, context coverage, collection-run metadata, and every explanation citation.

## CSV and filtered JSON

CSV is a spreadsheet-safe client export of raw headline fields. Filtered JSON includes the current view filters and selected rows. The canonical machine-readable artifacts remain the published JSON files above. Review [DATA_RIGHTS.md](DATA_RIGHTS.md) before redistributing upstream observations.

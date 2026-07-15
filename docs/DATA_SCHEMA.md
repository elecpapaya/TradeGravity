# Published data schema 2.0

TradeGravity publishes one versioned artifact set under `site/data/`. `meta.json`, `latest.json`, `series.json`, `quality.json`, the product index/files, and the explanation index/files share the same `schema_version` and publisher `generated_at`. `context.json` has its own refresh time because it is built before the trade publisher.

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
| `quality.json` | Missing/stale data, collection runs, provider comparisons | Pipeline calculations |
| `explanations/index.json` | Explanation coverage and generator counts | Explainer |
| `explanations/{ISO3}.json` | Claims with exact evidence IDs | Published JSON evidence |

## `meta.json`

Important fields include:

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
  "product_classification": "H6",
  "product_level": 2,
  "product_reporter_count": 49,
  "context_status": "success"
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

The validator checks cross-file provenance and counts, reporter and period uniqueness, finite non-negative numbers, calculated totals/shares, product keys, context coverage, collection-run metadata, and every explanation citation.

## CSV and filtered JSON

CSV is a spreadsheet-safe client export of raw headline fields. Filtered JSON includes the current view filters and selected rows. The canonical machine-readable artifacts remain the published JSON files above. Review [DATA_RIGHTS.md](DATA_RIGHTS.md) before redistributing upstream observations.

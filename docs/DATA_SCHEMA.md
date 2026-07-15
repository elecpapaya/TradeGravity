# Published data schema

TradeGravity publishes static JSON under `site/data/`. The files are generated together and share the same `schema_version`, `generated_at`, provider, and partner configuration.

## Time semantics

`generated_at` is the UTC time when the pipeline produced the files. It is not the observation date.

Each partner block carries its own `period_type` and `period` because upstream reporting can lag and different reporter/partner combinations can have different latest observations.

| Period type | Period example | Meaning |
| --- | --- | --- |
| `Y` | `2023` | Annual observation |
| `Q` | `2024-Q2` | Quarterly observation |
| `M` | `2024-06` | Monthly observation |

## `meta.json`

```json
{
  "schema_version": "1.0",
  "generated_at": "2026-07-15T04:13:00Z",
  "provider": "wits",
  "partners": ["USA", "CHN"],
  "reporter_count": 51,
  "observation_count": 400,
  "expected_partner_blocks": 102,
  "available_partner_blocks": 100,
  "missing_partner_blocks": 2,
  "period_counts": {
    "Y:2023": 96,
    "Y:2021": 2,
    "Y:2015": 2
  }
}
```

`observation_count` counts normalized source rows loaded by the publisher. Partner-block counts describe the latest summarized output. `period_counts` uses `<period_type>:<period>` keys and counts available reporter/partner blocks.

## `latest.json`

```json
{
  "schema_version": "1.0",
  "generated_at": "2026-07-15T04:13:00Z",
  "provider": "wits",
  "partners": ["USA", "CHN"],
  "rows": [
    {
      "iso3": "KOR",
      "usa": {
        "period": "2023",
        "period_type": "Y",
        "prev_period": "2022",
        "export": 123,
        "import": 456,
        "trade": 579,
        "growth": {
          "export": 0.12,
          "import": -0.04,
          "trade": 0.05
        },
        "growth_basis": "yoy"
      },
      "chn": {
        "period": "",
        "period_type": "",
        "export": 0,
        "import": 0,
        "trade": 0
      },
      "total": 579,
      "share_cn": 0
    }
  ]
}
```

### Calculated values

- `trade = export + import`
- `total = usa.trade + chn.trade`
- `share_cn = chn.trade / total` when `total > 0`, otherwise `0`
- growth values are ratios: `(current - previous) / previous`
- growth is omitted when a comparable prior period is unavailable or zero

Values are nominal US dollars as supplied and normalized by the selected provider. TradeGravity does not combine providers silently.

The schema describes structure and meaning, not a blanket license for upstream observations. See [DATA_RIGHTS.md](DATA_RIGHTS.md) for provider terms and attribution references.

## Compatibility

Additive fields may be introduced within a schema version. Removing fields, changing meanings, or changing required types requires a schema-version change and release note. Consumers should ignore unknown fields but should not assume that every reporter has both partner blocks.

## CSV export

The viewer can export the currently filtered reporters as CSV. This is a client-generated convenience representation, not a third canonical endpoint. It repeats schema, provider, and generation metadata on every row and flattens USA and CHN partner blocks into explicit columns.

All CSV cells are quoted, embedded quotes are doubled, and values beginning with spreadsheet formula characters are prefixed with an apostrophe to reduce CSV formula-injection risk. Automated consumers should prefer `latest.json`, which preserves types and nesting.

## Validation

The deployment workflow runs:

```bash
go run ./cmd/validator -dir site/data -min-reporters 40
```

Validation rejects malformed country codes and periods, duplicate reporters, negative or non-finite values, inconsistent totals or shares, mismatched metadata, and unexpected coverage counts.

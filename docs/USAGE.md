# Reusing TradeGravity

TradeGravity is designed for inspection, education, and small programmatic analyses where source periods and provenance remain visible.

## Suitable use cases

- **Research exploration:** compare the scale and direction of reporters' trade with the United States and China before moving to product-level or policy analysis.
- **Teaching:** demonstrate API normalization, reproducible pipelines, data lag, bilateral reporting asymmetry, validation, and accessible visualization.
- **Data tooling:** consume a small static JSON dataset without operating a database or web application server.
- **Monitoring:** detect changes in provider availability, reporting periods, or high-level totals that merit investigation in the upstream source.

## Important non-goals

TradeGravity is not a real-time feed, an exhaustive mirror of either provider, a substitute for country or product metadata, or a basis for financial, legal, or policy decisions. Reporters can have different latest periods. Imports and exports are from the reporter's perspective, and mirror statistics can differ for valuation, timing, classification, and reporting reasons.

## Read metadata first

```bash
curl --fail --silent --show-error \
  https://elecpapaya.github.io/TradeGravity/data/meta.json | jq
```

Check `schema_version`, `provider`, `generated_at`, coverage, and `period_counts` before reading observations. `generated_at` is a pipeline timestamp, not the observation period.

## JavaScript example

The following Node.js 22 or browser example lists the five largest reporters by combined USA/CHN trade while retaining both source periods:

```js
const url = "https://elecpapaya.github.io/TradeGravity/data/latest.json";
const response = await fetch(url);
if (!response.ok) throw new Error(`TradeGravity request failed: ${response.status}`);

const dataset = await response.json();
if (dataset.schema_version !== "1.0") {
  throw new Error(`Unsupported schema: ${dataset.schema_version}`);
}

const top = dataset.rows
  .filter(row => Number.isFinite(row.total) && row.total > 0)
  .sort((a, b) => b.total - a.total)
  .slice(0, 5)
  .map(row => ({
    reporter: row.iso3,
    total_usd: row.total,
    usa_period: `${row.usa.period_type}:${row.usa.period}`,
    chn_period: `${row.chn.period_type}:${row.chn.period}`,
  }));

console.table(top);
```

For production consumers, add timeouts, retries, local caching, schema checks, and an explicit policy for missing partner blocks. Do not silently compare values from different periods.

## CSV and filtered views

The web viewer's reporter filter changes the accessible table and the rows included by **Download CSV**. JSON remains the canonical typed representation; CSV is a flattened convenience export. See [DATA_SCHEMA.md](DATA_SCHEMA.md) and [DATA_RIGHTS.md](DATA_RIGHTS.md) before redistribution.

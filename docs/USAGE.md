# Reusing TradeGravity

TradeGravity is designed for inspection, education, and small programmatic analyses where source periods and provenance remain visible.

## Reference task

The main supported workflow is:

> Compare ASEAN reporters' same-period USA/China trade for 2023, select Viet Nam, inspect its recent trend and leading HS2 chapters, then share the exact view and export the filtered evidence.

In the viewer, choose `Y:2023`, `Same-period only`, group `ASEAN`, and the desired metric. Select `VNM`, then use **Copy view URL**, **Download CSV**, or **Download filtered JSON**. URL parameters preserve metric, color, Top N, period, comparison mode, region, income, group, normalization, reporter query, and selected country.

## Suitable use cases

- **Research exploration:** compare the scale and direction of reporters' trade with the United States and China before moving to product-level or policy analysis.
- **Teaching:** demonstrate API normalization, reproducible pipelines, data lag, bilateral reporting asymmetry, validation, and accessible visualization.
- **Data tooling:** consume a small static JSON dataset without operating a database or web application server.
- **Monitoring:** detect changes in provider availability, reporting periods, or high-level totals that merit investigation in the upstream source.

## Important non-goals

TradeGravity is not a real-time feed, an exhaustive mirror of either provider, or a basis for financial, legal, or policy decisions. Imports and exports are from the reporter's perspective, and mirror statistics can differ for valuation, timing, classification, and reporting reasons.

Per-capita and GDP-share modes use the latest published World Bank denominator shown with each country. They are cross-sectional aids, not constant-price or year-specific normalized time series. HS2 values use UN Comtrade and are not silently combined with WITS headline totals.

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
if (dataset.schema_version !== "2.0") {
  throw new Error(`Unsupported schema: ${dataset.schema_version}`);
}

const top = dataset.rows
  .filter(row => row.same_period && Number.isFinite(row.total) && row.total > 0)
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

For production consumers, add timeouts, retries, local caching, schema checks, and an explicit policy for missing partner blocks. Prefer `same_period: true` or select an exact point from `series.json`; do not silently compare mixed periods.

## Python notebook

[`examples/asean-analysis.ipynb`](../examples/asean-analysis.ipynb) reproduces the reference task from the public JSON endpoints. It asserts schema 2.0, applies the same-period and ASEAN filters, joins a selected reporter's series and product file, and saves a compact CSV. The notebook deliberately reads periods and provenance before calculating rankings.

For a network-free pipeline example, `cmd/sampledata` produces a deterministic SQLite fixture and context file; the sample README shows the publisher, explainer, and validator commands.

## CSV and filtered views

All explorer controls affect the treemaps and accessible table. **Download CSV** exports the filtered headline rows in raw source units; **Download filtered JSON** also records the active view state. JSON remains the canonical typed representation. See [DATA_SCHEMA.md](DATA_SCHEMA.md) and [DATA_RIGHTS.md](DATA_RIGHTS.md) before redistribution.

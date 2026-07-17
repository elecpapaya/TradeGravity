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
- **Partner screening:** select a reporter and inspect its largest reported bilateral partners before performing route-, firm-, or product-specific research.
- **Tariff sensitivity:** use a published strategic-HS6 MFN rate and import baseline for a transparent elasticity sensitivity check, not a forecast.

## Important non-goals

TradeGravity is not a real-time feed, an exhaustive mirror of either provider, or a basis for financial, legal, or policy decisions. Imports and exports are from the reporter's perspective, and mirror statistics can differ for valuation, timing, classification, and reporting reasons.

The multi-partner network is not a physical supply-chain route. The scenario lab is not a WITS SMART, welfare, GDP, substitution, or causal model. ECI/ESI/ICI/SPDI/RPI, mirror reconciliation, tariff-evasion estimates, and value-added routing are not currently published.

## Discovering scalable resources

Read `catalog.json` first, then follow the relevant index. For example, load `bilateral-matrix/index.json`, find the reporter/year partition, and fetch that bounded file. Do the same with `strategic-hs6/index.json` and `tariffs/index.json`; do not guess a partition that the index does not list.

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

## Reviewed email and social drafts

The Chip Lens distribution desk reads `briefing.json` and shows three deterministic, cited observations. **Download email draft** exports Markdown with a link back to the evidence. **Download carousel copy** exports a six-slide, 4:5 JSON copy bundle whose slides retain evidence URLs. **Copy evidence link** returns readers to the Chip Lens rather than to an uncited claim.

Both exports are editorial inputs, not automatic publishing instructions. Review the periods, values, source scope, wording, and rights before use. TradeGravity's static site does not collect email addresses, manage consent or unsubscribes, send mail, or publish to a social account.

For an offline review package with mobile-first email HTML, a cited Instagram caption, two network-free native themes, six matched editable SVG originals and 1080×1350 PNG upload assets, alt text, file hashes, and an approval checklist, run `cmd/distributor` as documented in [DISTRIBUTION.md](DISTRIBUTION.md). Building that package still does not authorize delivery or social publication.

After completing the checklist, use `cmd/distribution-approval` to verify the manifest and record a content release for `email`, `instagram`, or both. The record contains only a non-sensitive audience label and keeps consent, provider delivery, and automatic publishing false; it is not a subscriber list or a send instruction.

For an Instagram-approved kit, run `cmd/instagram-preflight` outside the kit before opening a private platform draft. It rechecks all six PNG dimensions, caption evidence/scope/tags, alt-text completeness, manifest hashes, and channel approval while emitting only aggregate results with `manual_upload_required: true` and `automatic_publish_authorized: false`. See [INSTAGRAM_PREFLIGHT.md](INSTAGRAM_PREFLIGHT.md).

For an email-approved kit, run `cmd/distribution-preflight` locally with private double-opt-in and suppression CSVs as documented in [EMAIL_DELIVERY_PREFLIGHT.md](EMAIL_DELIVERY_PREFLIGHT.md). Each active row must supply its own opaque HTTPS unsubscribe URL. The command applies suppressions, enforces URL uniqueness and the pilot limit, and writes an aggregate-only plan outside the kit without addresses or tokens. It never sends mail and deliberately leaves provider configuration and delivery authorization false.

If the selected provider does not own consent and unsubscribe state, `cmd/subscription-registry` can import existing verified double-opt-in evidence into a separate private SQLite database and export the two preflight CSVs. The default-off signup mode in `cmd/unsubscribe-service` can instead create pending consent, send a short Resend confirmation, and activate only after the reader's explicit POST. The same service handles scanner-safe unsubscribe links and signed suppressions behind an HTTPS reverse proxy. Follow [UNSUBSCRIBE_SERVICE.md](UNSUBSCRIBE_SERVICE.md) before handling real addresses.

For a deliberately bounded Resend pilot, `cmd/email-launch-approval` converts that exact aggregate preflight into a one-hour authorization only after the operator attests sender authentication, feedback suppression, privacy controls, and the final audience. `cmd/email-delivery` then reruns the live preflight, sends one isolated request per eligible recipient, and records only HMAC recipient identities in a private SQLite duplicate-prevention ledger. It remains inert without the matching files, `RESEND_API_KEY`, `TRADEGRAVITY_DELIVERY_SECRET`, and `-send-live`; see [EMAIL_PROVIDER_PILOT.md](EMAIL_PROVIDER_PILOT.md).

If a provider response is uncertain, the ledger blocks every automatic retry. `cmd/email-delivery-reconcile` can record provider-confirmed acceptance or non-acceptance using the private recipient address only to derive the existing HMAC key. Acceptance becomes permanently skippable; confirmed non-acceptance can be retried only with a different short-lived launch authorization. Never delete the ledger to bypass this gate.

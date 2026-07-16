# US–China Chip Supply Chain Lens

The US–China Chip Supply Chain Lens is TradeGravity's specialist semiconductor section. Its purpose is not to reproduce a generic worldwide industry map. It asks where each connector economy and value-chain stage sits between US and Chinese trade exposure, whether that position is moving, what contributes to it, and where the evidence stops.

The principle is **neutral reported observations, explicit US–China lens**. The source values are not altered to fit the perspective, and a US- or China-leaning label is not a political-alignment score.

## Free/public data policy

Every published metric must be reproducible from openly accessible official, intergovernmental, or public-web evidence. Paid market databases, licensed proprietary fab-capacity files, undisclosed vendor estimates, and paywalled evidence required to reproduce a metric are excluded.

The declared layers are UN Comtrade, WITS/TRAINS, World Bank Open Data, the public OECD ICIO release, and dated official policy/project pages. OECD ICIO is a lagged country-by-industry context layer; it is not yet computed into the UI and must never be relabeled as HS6 evidence. Public industry releases may add context, but no proprietary capacity number is promoted to a TradeGravity observation.

## Evidence layers

The atlas never collapses every input into a single opaque risk score. It publishes four independently labeled layers:

1. **Observed customs trade** — annual and focused monthly stage-mapped UN Comtrade HS6 rows reported against the USA and China anchors.
2. **External context** — qualitative country roles, official policy events, and dated intergovernmental or industry context.
3. **Capacity and project signals** — bounded official projects and industry forecasts that retain `planned`, `supported_plan`, or `industry_forecast` status.
4. **Illustrative estimate** — disruption/substitution arithmetic applied to a visible observed baseline.

Pipeline refresh, trade observation, policy-event, source-publication, announcement, forecast, and expected-operation dates remain separate.

## Value-chain model

The canonical stage registry is [`configs/semiconductor_reference.json`](../configs/semiconductor_reference.json). It models:

```text
Design, IP & EDA
  → materials & wafers
  → manufacturing equipment
  → logic & foundry output
  → memory & HBM
  → discrete, power & sensors
  → packaging, substrates & test
  → downstream compute & network demand
```

Each stage declares its HS6 codes, observation type, description, and measurement gap. Some codes deliberately appear in more than one stage because the same customs proxy can inform equipment and packaging, for example. Stage totals therefore are not additive.

The strategic registry contains at least 30 unique stage-mapped codes across direct goods, revision-sensitive categories, and broad proxies. The publisher rejects a stage code that is missing from [`configs/strategic_hs6.csv`](../configs/strategic_hs6.csv).

## Coverage gate

The publication declares `reference_only`, `limited`, or `research_ready`. `research_ready` requires all of:

- at least 30 registered HS6 codes;
- at least 15 reporters with a stage-mapped observation; and
- at least five annual observation periods.

Passing the gate does not make HS6 a capacity database. It means only that the published reporter/period sample is broad enough for the atlas's descriptive comparisons. The UI always keeps the measurement limitations visible.

The staggered semiconductor refresh restores the latest successful core database, waits beyond the public Comtrade quota window, and requests the selected year plus four prior annual periods only for the declared connector allowlist. Reporter/year files remain bounded and are loaded lazily, eight at a time, only when the Chip Lens is opened. The same bounded refresh then requests the latest 12 complete months for the 30 mapped codes and connector allowlist. Monthly values are a turning-point layer; absent reporters, products, or months stay missing rather than being interpolated.

## Calculations

For a selected stage and metric, TradeGravity sums the mapped product rows within each reporter, anchor, and period. It then calculates:

```text
USA share        = USA value / (USA value + China value)
China share      = China value / (USA value + China value)
exposure balance = USA share − China share
dual exposure    = 2 × min(USA share, China share)
position shift   = current exposure balance − previous comparable exposure balance
```

Positive balance or shift means toward the USA; negative means toward China. `dual exposure` is 100% for an even two-anchor split and 0% when only one anchor is observed. The view also retains the observed stage value, reporter count, change from the closest prior published period, anchor growth divergence, and leading reporter rows. The focused monthly view applies the same formulas over its bounded window.

These values describe the published USA/China-anchor reporter sample. They are not global production share, global supplier concentration, whole-world dependency, geopolitical alignment, value-added origin, firm revenue, or installed capacity. Coverage changes can affect the displayed period change.

## Mirror-reporting diagnostics

When both sides publish the matching annual bilateral total, TradeGravity compares country-reported exports with anchor-reported imports and country-reported imports with anchor-reported exports:

```text
gap             = country report − counterpart report
symmetric ratio = gap / ((country report + counterpart report) / 2)
```

Neither reporter is treated as ground truth. Differences can reflect CIF/FOB valuation, timing, classification, partner attribution, re-exports, and revisions. The diagnostic is not an adjusted series and is not evidence of fraud, evasion, transshipment, or a physical route.

The stage sensitivity uses:

```text
disrupted_amount = observed_baseline × disruption_percent
substitution_offset = disrupted_amount × substitution_percent
residual_exposure = disrupted_amount − substitution_offset
```

It does not model prices, inventories, input-output propagation, trade diversion, GDP, welfare, or causality.

## Country roles and capacity signals

Country roles are an unranked, qualitative specialization matrix. A blank cell means only that the bounded reference has not assigned that role; it does not prove the activity is absent. A country can also appear as `observed_only` when a published trade partition exists but no curated role has been registered.

Capacity signals retain the source's stated status and expected-operation text. An announcement, selected support plan, construction forecast, equipment-spending forecast, and operating output are never converted into one capacity number. The current register is intentionally not a complete company or fab database.

## Revision and product limitations

- HS 2022 split several legacy semiconductor-device groups. Comparisons must use the row's source classification and the registry's `revision_note`.
- HS6 `854232` does not distinguish DRAM, NAND, HBM generation, bandwidth, or package architecture.
- Integrated-circuit trade does not identify fabless design origin, foundry ownership, process node, wafer size, or fabrication site.
- Printed circuits and photographic chemicals are broad proxies with non-semiconductor uses.
- EDA, design IP, licensing, cloud compute, and many engineering services are not goods trade.
- Trade links are reported relationships, not ports, firms, value-added paths, transshipment proof, or physical shipment routes.

Process-node capacity, firm ownership, and validated input-output propagation remain separate future datasets. TradeGravity will consider only free/public, reproducible inputs; paid proprietary capacity is out of scope. The catalog must not mark a layer ready without a published artifact, declared grain, interpretation limits, and validation.

## Reproduction

Build the reference publication and coverage metadata with:

```bash
go run ./cmd/collector strategic -provider comtrade -primary-provider wits -year auto -history-years 4 -allowlist configs/chip_connectors.csv
COMTRADE_FREQUENCY=M go run ./cmd/collector chip-monthly -provider comtrade -months 12 -allowlist configs/chip_connectors.csv
go run ./cmd/collector matrix -provider comtrade -primary-provider wits -year auto
go run ./cmd/publisher build -out site/data
go run ./cmd/validator -dir site/data -min-reporters 40
```

The canonical reference endpoint is `data/semiconductors/reference.json`; focused monthly discovery is `data/semiconductors/monthly/index.json`, and mirror diagnostics are discovered from `data/mirror/index.json`. Use the Chip Lens CSV export for the active stage distribution. Retain the reference JSON, relevant index and partition, metric, observation period, and view URL when citing a result.

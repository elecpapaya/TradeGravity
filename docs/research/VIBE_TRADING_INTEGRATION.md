# Vibe-Trading integration assessment

**Decision date:** 2026-07-17

**Upstream reviewed:** HKUDS/Vibe-Trading commit [`86f6012`](https://github.com/HKUDS/Vibe-Trading/tree/86f6012e00120e3fa5c3f0e15be8c94abe732dcf), package version `0.1.11`

**Question:** Should TradeGravity connect its US-China semiconductor supply-chain evidence to company and stock-market information through Vibe-Trading?

## Decision

Proceed only with a **loosely coupled, read-only companion pilot**. Do not embed Vibe-Trading in the public TradeGravity site, do not add broker connectivity, and do not publish market-price data until the selected provider's redistribution terms are explicitly cleared.

The useful seam is:

```text
TradeGravity versioned semiconductor evidence
        │ read-only snapshot + issuer/stage registry
        ▼
Pinned Vibe-Trading MCP process or local container
        │ market/fundamental tools; no order tools
        ▼
Local, dated research bundle with provenance and limitations
```

This preserves TradeGravity's static, inspectable architecture while testing whether market context helps users. A public stock panel should be a later decision based on data rights, market coverage, reliability, and user evidence.

## What is verified

### TradeGravity boundary

TradeGravity is a Go/SQLite publication pipeline that emits versioned static JSON and a static browser client; the browser receives neither provider nor OpenAI credentials. Its weekly semiconductor workflow publishes bounded annual and 12-month HS6 observations after validation, while a `main` push reuses the last validated dataset instead of recollecting it. See [DESIGN.md](../../DESIGN.md), [the data schema](../DATA_SCHEMA.md), and [the semiconductor methodology](../SEMICONDUCTOR_ATLAS.md).

The current semiconductor layer is country/stage/product evidence, not a firm or fab database. It explicitly says that HS6 trade cannot identify fab ownership, design origin, process node, or physical routes, and that the current register is not a complete company database. A ticker-to-stage join therefore requires a new evidence-backed entity layer; it cannot be inferred from existing trade rows. See [SEMICONDUCTOR_ATLAS.md](../SEMICONDUCTOR_ATLAS.md#revision-and-product-limitations).

### Vibe-Trading capabilities

Vibe-Trading is a Python 3.11+ beta package with a CLI, FastAPI server, React frontend, MCP server, LangChain/LangGraph agent runtime, backtesting, and a broad scientific/data dependency set. Its documented public interfaces are the CLI, HTTP application, and MCP tools; the package declares version `0.1.11` and MIT licensing. ([pyproject](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/pyproject.toml#L1-L71), [runtime structure](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/README.md#L1271-L1303))

The MCP surface includes `get_market_data`, `get_stock_profile`, `get_sec_filings`, `get_financial_statements`, screening, news, and backtesting. `get_market_data` accepts symbols, dates, source, interval, and a bounded row count, returning strict JSON with unresolved symbols reported rather than silently omitted. ([MCP interface](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/agent/mcp_server.py#L1045-L1086), [normalization/output code](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/agent/src/market_data.py#L49-L129))

Its US SEC tool is read-only, uses free/no-auth EDGAR JSON, and exposes recent filings plus selected XBRL series. The SEC permits free programmatic access but currently asks clients to declare a user agent and stay at or below 10 requests per second. ([Vibe-Trading SEC tool](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/agent/src/tools/sec_filings_tool.py#L1-L88), [SEC fair-access rules](https://www.sec.gov/search-filings/edgar-search-assistance/accessing-edgar-data))

## Fit and gaps

| Area | Fit | Evidence and consequence |
| --- | --- | --- |
| Read-only price context | Good for a pilot | Normalized OHLCV and fallback routing are already exposed through MCP. Do not use broker/order tools. |
| US company fundamentals | Good | SEC filings/XBRL are a stronger reproducible base than analyst consensus. |
| Public static deployment | Poor | Vibe-Trading is a stateful Python service/process, whereas TradeGravity is static GitHub Pages. A browser-to-Vibe call would require an always-on backend and public authentication. |
| Semiconductor entity mapping | Missing | Neither project supplies an audited company-to-stage, listing, operating-country, or fab-ownership crosswalk. |
| Geographic coverage | Major gap | Documented/auto-routed equity patterns cover US, HK, mainland China and India; unmatched symbols default to Tushare. There is no documented first-class routing for Taiwan, Korea, Japan, the Netherlands, Germany, Malaysia, Singapore, Viet Nam, or Mexico, although these are central TradeGravity connector economies. ([documented sources](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/README.md#L306-L333), [source detection](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/agent/src/market_data.py#L16-L39)) |
| Comparable returns | Gap | The yfinance loader requests `auto_adjust=False` and then retains only OHLCV, not adjusted close. Split/dividend-safe total-return comparison is therefore not established by this interface. ([loader code](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/agent/backtest/loaders/yfinance_loader.py#L95-L109), [normalization](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/agent/backtest/loaders/yfinance_loader.py#L160-L190)) |
| Causal interpretation | Not supported | Monthly/annual customs observations and daily forward-looking equity prices have different clocks. Correlation must not be described as supply-chain impact or an investment signal. |

The geographic gap is the gating issue. A pilot restricted to US, HK, and mainland listings would be technically feasible but would systematically underrepresent the semiconductor chain that TradeGravity is designed to explain. Adding unsupported tickers ad hoc through a raw provider call would defeat Vibe-Trading's normalized routing and TradeGravity's reproducibility standard.

## Integration options

### 1. Embed Vibe-Trading or call its HTTP server from GitHub Pages — reject

Vibe-Trading's Docker configuration binds to loopback by default, persists sessions/configuration, and recommends bearer authentication for non-local clients. Its security documentation also warns that generated backtest code executes locally and remains network-capable, despite a narrowed environment. ([deployment/auth](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/README.md#L540-L596), [security policy](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/SECURITY.md#L23-L27), [container hardening](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/docker-compose.yml#L1-L59))

A static public browser cannot safely hold a shared bearer secret. Hosting the service would introduce a second deployment, persistent state, abuse controls, CORS/authentication, patching, and variable LLM/provider costs. It is disproportionate to the first hypothesis.

### 2. Import Vibe-Trading internals into TradeGravity — reject

This would mix Go and a large Python/agent dependency graph and couple TradeGravity to non-documented internal Python modules. It would also turn a low-cost static pipeline into a more fragile application. The MIT code license does not remove market-data terms or operational risk.

### 3. Read-only MCP/local companion — recommend

Pin a Vibe-Trading release or commit and run `vibe-trading-mcp` locally or in an isolated CI experiment. Provide it a dated TradeGravity evidence snapshot and a small, manually cited issuer registry. Allow only research tools such as market data and SEC filings; do not configure any `trading_*`, shell, swarm, scheduler, broker, or live/advisory capability. Vibe-Trading documents stdio and loopback HTTP MCP transports, so this can remain outside the public site. ([MCP transports and tools](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/README.md#L932-L990))

### 4. Publish a bounded market-context artifact — defer

Only after the pilot should TradeGravity consider `market-context/index.json` plus issuer partitions. That artifact would need its own schema, provider/terms register, observation and retrieval timestamps, currency, exchange/MIC, corporate-action handling, missing-data state, and explicit separation from TradeGravity trade metrics. No LLM prose should become a canonical numeric field.

## Licensing, privacy, and security

- **Code:** Vibe-Trading is MIT-licensed; redistribution of a substantial copy requires retaining its copyright and permission notice. This is normally compatible with TradeGravity's Apache-2.0 codebase, but vendoring is unnecessary for the pilot. ([Vibe-Trading license](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/LICENSE), [TradeGravity license](../../LICENSE))
- **Data:** The code license does not license market data. Vibe-Trading's default US/HK path uses Yahoo/yfinance-related endpoints. The yfinance project says it is for research/education, is not affiliated with Yahoo, and directs users to Yahoo's terms; it specifically describes Yahoo Finance API data as intended for personal use. Public redistribution through TradeGravity is therefore **not cleared** by installing Vibe-Trading. ([yfinance notice](https://github.com/ranaroussi/yfinance#readme))
- **Provider unknowns:** Terms for each fallback source, derived-return redistribution, caching duration, and display attribution have not been established. A fallback could also change the effective provider between runs. Record the effective provider per symbol or fail closed.
- **Secrets:** Keep all provider and LLM credentials outside TradeGravity JSON. Run on an isolated worker with no broker keys. Do not expose Vibe settings/session endpoints publicly.
- **Personal data:** Do not use Shadow Account, uploaded brokerage exports, holdings, or persistent memory for this feature. The integration needs public issuer evidence only.

## Cost and reliability

Vibe-Trading itself has no license fee and its core market tools can run without an LLM key, but "free" endpoints still carry rate-limit, availability, and terms risk. LLM/swarm execution and optional premium providers create variable external cost; they are unnecessary for deterministic market-context calculations. Vibe-Trading's own fallback order is designed around throttling/IP-ban risk, which is evidence that provider instability must be expected rather than hidden. ([fallback design](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/README.md#L306-L340))

Do not run this on every push. If automation is later justified, run after the weekly semiconductor publication, cache by `provider × symbol × interval × date range`, bound retries/concurrency, and publish `current`, `partial`, or `degraded` status without carrying forward stale values as current.

## Staged experiment

### Stage 0 — contract and rights gate

1. Define `issuer_registry.json` with stable issuer ID, legal name, listings (`ticker`, exchange/MIC, currency), domicile, operating countries, TradeGravity stages, evidence URL, effective dates, and a direct/proxy classification.
2. Start with 8–12 issuers across at least four stages. Every stage assignment must cite an issuer filing or official company page; listing country and operating footprint must remain separate.
3. Select one market-data provider whose terms allow the intended local experiment. Do not approve public redistribution yet.

### Stage 1 — local read-only proof

1. Freeze a TradeGravity snapshot by repository commit, `generated_at`, provider, reporter, period, stage, and HS6 evidence IDs.
2. Run the pinned Vibe-Trading MCP process in a container/venv with no broker or LLM credentials.
3. Fetch daily OHLCV and official filings for only the supported pilot listings. Retain unresolved symbols and effective provider names.
4. Produce a local report that juxtaposes, without causal language:
   - TradeGravity monthly stage movement and publication-revision status;
   - issuer/stage evidence;
   - 20/60-trading-day price movement, currency and corporate-action caveat;
   - filing dates and directly reported metrics where available.

### Stage 2 — decision test

Test with three target users. Success requires at least two users to correctly distinguish (a) customs movement, (b) publication revision, and (c) stock-price movement, and no user to interpret the panel as a causal or trading recommendation. Also require deterministic reproduction, zero silently dropped tickers, and an explicit provider/terms record for every value.

Stop or redesign if the connector-market coverage remains below the intended semiconductor scope, corporate actions cannot be handled consistently, provider terms prohibit the intended display, or users conflate the three clocks.

## Open questions before any public feature

1. Which provider contract permits public display or redistribution for each required exchange?
2. Can Vibe-Trading support Taiwan, Korea, Japan, Europe, and Southeast Asia through its normalized loader/MCP contract, not raw one-off scripts?
3. What adjusted-price/total-return field and corporate-action method will be canonical?
4. Who maintains issuer-to-stage evidence through ticker changes, ADRs, spin-offs, acquisitions, and multi-stage firms?
5. Is the user value greater than a simpler TradeGravity export plus a companion notebook?

Until these are answered, describe the experiment as **market context for research**, never as valuation, prediction, causal impact, or investment advice. Vibe-Trading itself gives the same high-level warning: it is research software, not investment advice. ([upstream disclaimer](https://github.com/HKUDS/Vibe-Trading/blob/86f6012e00120e3fa5c3f0e15be8c94abe732dcf/README.md#L1471-L1473))

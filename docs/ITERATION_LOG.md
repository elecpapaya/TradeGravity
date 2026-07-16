# TradeGravity iteration log

This log records why TradeGravity changes, what evidence supported each change, how it was checked, and what decision follows. It is deliberately different from the [changelog](../CHANGELOG.md), which records shipped behavior, and the [roadmap](../ROADMAP.md), which records intended work.

The log is not a diary of every commit. Add or update an entry when a bounded product, data, or maintenance decision produces evidence that another contributor can inspect.

## Improvement loop

1. **Observe:** state the user, data, or maintenance problem without assuming its cause.
2. **Hypothesize:** describe the expected improvement and the interpretation risk it should reduce.
3. **Choose a signal:** define what would justify keeping, revising, or reverting the change.
4. **Change:** link the issue, pull request, commit, files, or deployed artifact.
5. **Verify:** separate automated checks, local inspection, production observation, and external-user evidence.
6. **Decide:** record the result even when it is neutral or negative, then name the next bounded step.

## Evidence rules

- Do not invent participants, quotes, usage, completion rates, or performance results.
- Label automated and maintainer-run checks as such; they are not evidence of user comprehension or adoption.
- Link public or reproducible evidence where possible. Summarize private feedback only with consent and without identifying information.
- Record failed, inconclusive, and reverted experiments as well as successful ones.
- Keep observation dates, source publication dates, pipeline refresh dates, and evaluation dates distinct.
- Append dated corrections instead of silently rewriting a decision after the fact.

## Status vocabulary

- **Proposed:** hypothesis and success signal are defined; implementation has not started.
- **Implemented:** the change exists on a branch or in `main`; required technical checks are recorded.
- **Production-verified:** deployed artifacts and behavior were checked in the public site.
- **Externally evaluated:** at least one real person outside the implementation team completed the stated evaluation.
- **Adopted, revised, or reverted:** a decision was made from the recorded evidence.

## Entry template

Copy this section for a new bounded decision. Use `Not measured yet` instead of filling an evidence gap with an estimate.

```markdown
## TG-YYYY-NNN — Decision title

- Date opened: YYYY-MM-DD
- Status: Proposed
- Scope: Product, data, reliability, security, accessibility, or maintenance
- Owner: GitHub handle or maintainer role

### Observation

What was observed, for whom, and where is the evidence?

### Hypothesis and decision signal

If we change X, we expect Y. Keep, revise, or stop when Z is observed.

### Change

What changed? Link the issue, pull request, commit, files, and artifacts.

### Evidence

| Evidence type | Date | Result | Interpretation |
| --- | --- | --- | --- |
| Automated / local / production / external user | YYYY-MM-DD | Not measured yet | What this does and does not establish |

### Decision and next step

State the current decision and the next bounded evidence-gathering step.
```

## TG-2026-001 — Separate observed chip movement from publication revision

- Date opened: 2026-07-16
- Status: Implemented; local verification complete; production and external evaluation pending
- Scope: Semiconductor data interpretation and publication reliability
- Owner: TradeGravity maintainer

### Observation

The Chip Lens already exposed monthly observations, but a refreshed publication could add coverage or revise an existing source value. Without an explicit comparison to the previous publication, a user could confuse a publish-to-publish revision with an economic month-to-month movement. A first publication also needed to say `baseline` rather than imply that nothing changed.

This was a design-risk observation from the implementation review. It was not a finding from external users.

### Hypothesis and decision signal

If the site presents the latest monthly movement separately from a bounded, machine-readable publication comparison, users and data consumers should be better able to distinguish an observed market movement from coverage drift or a source revision.

Keep the design when all of the following are true:

- the deployed `changes.json` passes validation and identifies `baseline`, `unchanged`, or `changed` without contradicting the current monthly index;
- the Chip Lens and exported report label monthly movement and publication revision as different comparisons; and
- at least two of three external task participants can explain that difference without coaching.

Revise the labels or layout if any participant treats publication status as market growth, or if production comparison counts conflict with the published monthly artifacts.

### Change

Commit [`d31c7ef`](https://github.com/elecpapaya/TradeGravity/commit/d31c7ef850296e8d4928725c88cb37282c3f318e) added:

- a bounded `changes.json` comparison built from the current and previously deployed focused-monthly semiconductor publications;
- validator and catalog checks for comparison status, dimensions, counts, arithmetic, and revision ordering;
- a Chip Lens publication Pulse and Markdown evidence report that remain separate from latest month-to-month growth; and
- workflow restoration of the previous publication so scheduled runs can calculate the comparison.

The schema and interpretation boundary are documented in [DATA_SCHEMA.md](DATA_SCHEMA.md) and [SEMICONDUCTOR_ATLAS.md](SEMICONDUCTOR_ATLAS.md).

### Evidence

| Evidence type | Date | Result | Interpretation |
| --- | --- | --- | --- |
| Automated unit and structure checks | 2026-07-16 | `go test ./...`, `go vet ./...`, and 42 Node tests passed | Establishes tested calculations and expected UI structure; does not establish production data quality or user comprehension |
| Local pipeline check | 2026-07-16 | Publisher → explainer → validator completed against the sample publication | Establishes that the generated artifacts form a validator-accepted bundle; does not establish that the next scheduled provider run succeeds |
| Maintainer browser inspection | 2026-07-16 | Desktop and narrow layouts had no horizontal overflow or console errors in the inspected states | Establishes a bounded local rendering check; does not replace assistive-technology or external usability testing |
| Production artifact | Not measured yet | The public `data/changes.json` has not yet been verified for this change | Required after merge and a manual or scheduled semiconductor refresh |
| External-user comprehension | Not measured yet | No participant result has been recorded | Required through the [user-testing protocol](USER_TESTING.md); do not substitute a simulated session |

### Decision and next step

Retain the implementation as a release candidate. After CI and merge, run or observe the semiconductor refresh, verify the public `changes.json` against its monthly index, and record the production result here. Then add one comprehension prompt to the three-person task: ask participants to describe the difference between the latest monthly movement and the change since the previous publication. Adopt or revise the presentation only after those results are recorded.

## TG-2026-002 — Test Vibe-Trading as a read-only market-research companion

- Date opened: 2026-07-17
- Status: Proposed; rights, coverage, and data-contract gates unresolved
- Scope: Semiconductor market context and optional external integration
- Owner: TradeGravity maintainer

### Observation

TradeGravity explains country-, stage-, and product-level semiconductor trade evidence but does not connect that evidence to listed-company market context. The request to evaluate Vibe-Trading is a maintainer product hypothesis, not evidence that users need a stock panel.

The [integration assessment](research/VIBE_TRADING_INTEGRATION.md) found a functional match for read-only research, but also found that Vibe-Trading's documented automatic equity routing does not cover several central TradeGravity connector markets, its market-data result does not yet meet TradeGravity's publication-provenance contract, and installing MIT-licensed code does not grant redistribution rights for upstream prices.

### Hypothesis and decision signal

If TradeGravity exports a dated semiconductor evidence snapshot and a separately cited issuer-to-stage registry to a pinned, read-only Vibe-Trading process, target users may be able to compare structural trade evidence with market movement without turning TradeGravity into a stateful trading service.

Proceed beyond a local pilot only when all of the following are true:

- every issuer-stage assignment has dated first-party evidence and keeps listing country separate from operating footprint;
- every market value records effective provider, symbol, exchange, currency, retrieval time, adjustment method, and missing state;
- the intended use is permitted by the selected provider's terms and no unsupported symbol is silently dropped; and
- at least two of three external participants correctly distinguish customs movement, publication revision, and stock-price movement, with no participant interpreting the output as causal or investment advice.

Stop or redesign if the required Korean, Taiwanese, Japanese, European, or other connector listings remain unsupported; corporate actions cannot be handled consistently; redistribution rights are unclear; or the public feature would require browser-held secrets, broker access, or an always-on Vibe-Trading backend.

### Change

No integration code or market data was added. The investigation produced [`docs/research/VIBE_TRADING_INTEGRATION.md`](research/VIBE_TRADING_INTEGRATION.md), pinned its upstream review to a commit, compared four integration options, and defined a gated local experiment.

### Evidence

| Evidence type | Date | Result | Interpretation |
| --- | --- | --- | --- |
| Upstream source and documentation review | 2026-07-17 | Read-only MCP research tools are available, but the runtime is a stateful Python/FastAPI application and documented automatic equity routing is concentrated in US, HK, mainland China, and India | Supports a loose-coupled local experiment; does not support describing the integration as a global semiconductor-equity layer |
| Data-contract review | 2026-07-17 | Current OHLCV output lacks the complete provider, currency, exchange, and adjusted-return contract required for canonical TradeGravity artifacts | Blocks public market-data publication through the current interface |
| Licensing and security review | 2026-07-17 | MIT code reuse is possible, but market-data redistribution rights remain provider-specific; public browser-to-service integration would introduce secrets, authentication, state, and generated-code risk | Favors a no-broker, no-LLM, local read-only companion and a separate rights gate |
| Local proof | Not measured yet | No Vibe-Trading process or issuer registry has been created | Required only after a provider and bounded issuer scope are selected |
| External-user value | Not measured yet | No participant has evaluated the proposed workflow | Required before adding a public stock panel |

### Decision and next step

Reject embedding Vibe-Trading, importing its internal Python modules, or using it as the current public site's live backend. Keep a read-only MCP/local companion as a conditional experiment. The next bounded step, if pursued, is Stage 0 only: select a legally usable provider and create a cited 8–12 issuer registry across at least four semiconductor stages before installing or invoking Vibe-Trading.

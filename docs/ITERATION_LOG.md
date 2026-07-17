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

## TG-2026-003 — Establish a deterministic notebook before a finance MCP

- Date opened: 2026-07-17
- Status: Proposed; implementation and provider testing have not started
- Scope: Semiconductor market-research handoff, reproducibility, and data rights
- Owner: TradeGravity maintainer

### Observation

TradeGravity already publishes versioned JSON and includes a small Jupyter example, but the proposed semiconductor-to-market handoff does not yet have a deterministic reference consumer. The request to compare Notebook and finance MCP options is a maintainer hypothesis, not evidence that users need a stock-research workflow.

The [finance handoff assessment](research/FINANCE_HANDOFF_OPTIONS.md) found that notebook and MCP frameworks can both consume financial data, but neither supplies global listing coverage, correct corporate-action treatment, or redistribution rights by itself. A provider-backed MCP also adds an abstraction layer before TradeGravity has a reference result against which to test it.

### Hypothesis and decision signal

If a pinned Quarto or Jupyter analysis consumes a versioned TradeGravity handoff, a cited issuer registry, and synthetic or locally licensed market inputs, the project can test the value of the combined analysis while keeping calculations, provenance, missing states, and license boundaries inspectable.

Advance to an MCP comparison only when all of the following are true:

- the notebook executes from a clean environment without a browser secret or unrecorded network fallback;
- each result preserves provider, symbol, exchange/MIC, currency, listing type, price-adjustment basis, retrieval time, and unresolved status;
- split, dividend, ticker-change, stale-response, missing-day, currency, and ADR cases have deterministic tests; and
- a selected provider confirms the intended storage, display, derived-data, caching, and retention rights in writing.

Stop or redesign if the leading provider cannot return primary listings and corporate actions across the bounded connector-market sample, or if an MCP result differs from the pinned direct-API reference or omits provenance.

### Change

No notebook, MCP server, provider account, API key, or market-data artifact was added. The investigation produced [`docs/research/FINANCE_HANDOFF_OPTIONS.md`](research/FINANCE_HANDOFF_OPTIONS.md), ranked the local and browser notebook surfaces, compared official finance data/MCP options, defined a canonical evidence contract, and specified a staged provider bake-off.

### Evidence

| Evidence type | Date | Result | Interpretation |
| --- | --- | --- | --- |
| Repository review | 2026-07-17 | An existing schema-2.0 Jupyter example reads TradeGravity's public JSON, while CI currently does not execute notebooks | Supports extending a familiar project pattern; does not establish that a market handoff is useful or reproducible yet |
| Official notebook-tool review | 2026-07-17 | Quarto, marimo, Jupyter/JupyterLite, and Observable can produce different combinations of static, local, and browser analysis; browser-WASM variants have package and secret constraints | Supports a pinned local report first; does not prove compatibility with the final package set |
| Official provider and MCP review | 2026-07-17 | Candidate services expose price, identifier, filing, and corporate-action capabilities, but plan coverage and redistribution conditions vary; broker-linked MCPs add out-of-scope write operations | Supports a direct, read-only provider bake-off before adopting MCP; does not select a canonical provider |
| Local deterministic proof | Not measured yet | No handoff schema, issuer registry, fixture, or report has been implemented | Required in Stage 0 |
| External-user value | Not measured yet | No participant has evaluated the combined report | Required before a public feature or MCP integration |

### Decision and next step

Prefer a Quarto `.qmd` plus a small tested Python calculation module as the first reference consumer; a plain Jupyter notebook remains the minimum-friction fallback. Defer finance MCP adoption and reject broker-capable MCPs for this scope. The next bounded step is Stage 0 only: define the handoff, issuer-registry, and market-evidence schemas, then render a no-network report from synthetic corporate-action fixtures and one cited TradeGravity snapshot.

## TG-2026-004 — Derive reviewed distribution drafts from one evidence contract

- Date opened: 2026-07-17
- Status: Implemented; local verification complete; production and external evaluation pending
- Scope: Distribution, editorial safety, privacy, and adoption evidence
- Owner: TradeGravity maintainer

### Observation

The project can generate evidence-grounded analysis, but publishing a newsletter or Instagram carousel separately would risk inconsistent numbers, missing citations, and unsupported automation. TradeGravity is currently a static site with no consent store, sender identity, suppression list, or social-account publishing boundary. The request for email and card-news distribution is a maintainer product hypothesis, not evidence of audience demand.

### Hypothesis and decision signal

If the publisher derives email and social drafts from the same validated semiconductor observations and requires a human review before either leaves the repository, the maintainer can test distribution usefulness without introducing subscriber PII, browser-held secrets, or uncited claims.

Retain the contract when all of the following are true:

- every ready draft has the same three signal kinds, periods, arithmetic, and cited evidence as the published monthly artifacts;
- the deployed UI never claims that delivery is configured and never collects an email address;
- an editor can trace every email section and carousel slide to the cited artifact; and
- at least two of three intended readers say the brief gives them a reason to inspect the underlying Chip Lens.

Do not enable provider sending or direct social publishing until double opt-in, unsubscribe/suppression handling, privacy retention, sender authentication, platform rights, and an explicit approval gate are documented and tested.

### Change

The implementation adds:

- a deterministic `briefing.json` publisher contract with reporter-scale, two-anchor share-shift, and HS6 product signals;
- review-gated email Markdown and a cited six-slide 4:5 carousel-copy bundle;
- validator checks for provenance, periods, totals, shares, deltas, ratios, evidence paths, slide roles, and mandatory human review;
- Chip Lens controls for inspecting signals, downloading both drafts, and copying the evidence entry point; and
- a fail-closed unavailable state when two comparable monthly periods do not exist; and
- an offline review-kit builder that emits one-primary-CTA email HTML/Markdown, a cited Instagram caption, two native themes, six matched 1080×1350 SVG/PNG assets, alt text, an approval checklist, and a deterministic hash manifest, plus a read-only manual Actions workflow.
- a content-release approval contract and CLI that reject modified, missing, or untracked kit files before binding the manifest digest to a reviewer, audience label, time, and approved channels while leaving provider, consent, and automation readiness false.
- a local email preflight that verifies email content approval, exact private double-opt-in and suppression schemas, audience match, timestamps, unique opaque HTTPS unsubscribe URLs, suppression precedence, and a pilot ceiling before emitting an address/token-free aggregate plan with provider and delivery authorization false.
- a separate private SQLite subscription service that imports existing consent or runs a default-off double-opt-in form, sends short-lived confirmation mail with stable idempotency, activates only on explicit POST, exports HMAC links and suppression rows, verifies signed Resend feedback, and prevents globally suppressed addresses from reactivation.
- a fail-closed Resend pilot adapter that requires a one-hour launch approval bound to the exact preflight, sender, audience, content and input digests; reruns all preflight checks at send time; sends one recipient per request with visible/header one-click links; stores only HMAC recipient identities in a private duplicate-prevention ledger; and requires provider-evidence reconciliation plus a different launch approval before retrying confirmed non-acceptance.
- an aggregate Instagram manual-publish preflight that rechecks the approved kit, PNG dimensions, caption evidence/scope/tags, and alt-text completeness without storing content or credentials and without authorizing a post.

No live provider account, API key, real subscriber record, tracking pixel, social credential, or automatic publishing action was added. The Resend adapter is explicit, local, and inert without private inputs, a matching short-lived authorization, two environment secrets, and `-send-live`.

### Evidence

| Evidence type | Date | Result | Interpretation |
| --- | --- | --- | --- |
| Go unit and static checks | 2026-07-17 | `go test ./...` and `go vet ./...` passed | Establishes deterministic calculations and validator behavior; does not establish production-provider coverage or editorial quality |
| Frontend contract and structure checks | 2026-07-17 | 45 Node tests and JavaScript syntax checks passed | Establishes browser helper behavior and expected UI wiring; does not establish real-user comprehension |
| Sample publication validation | 2026-07-17 | `cmd/validator` accepted the synthetic three-reporter publication with `briefing.json` | Establishes a network-free end-to-end artifact contract; synthetic values are not real market evidence |
| Distribution-kit build | 2026-07-17 | The sample briefing produced review-pending email plus six matched SVG/PNG cards with deterministic SHA-256 entries and explicit false send/publish authorization | Establishes repeatable production inputs and safety gates; does not establish provider delivery or audience value |
| Distribution-kit render | 2026-07-17 | Six SVGs and six embedded-font PNGs decoded at 1080×1350 with no external assets; representative scale, product, and method PNGs were visually inspected; email HTML rendered on desktop and 390px mobile with one primary CTA plus a utility unsubscribe link, no scripts or images, no horizontal overflow, and no console issues | Establishes bounded visual, dimension, and dependency checks; does not establish inbox-client compatibility, accessibility-tool results, Instagram acceptance, or reader comprehension |
| Content-approval gate | 2026-07-17 | A fixed-time sample approval verified the 20-file manifest, sorted email/Instagram channels, recorded three attestations, refused overwrite, and retained false provider/consent/automatic-publish readiness; tests rejected tampered caption/email content, missing, and untracked files plus manifest changes after approval | Establishes a reproducible content-to-manifest binding; it does not authenticate the reviewer, prove subscriber consent, configure a provider, or authorize a live send/post |
| Email consent preflight | 2026-07-17 | Reserved-domain fixtures verified two double-opt-in rows, unique opaque HTTPS unsubscribe URLs, one matched suppression, one eligible recipient, exact audience binding, address/token-free JSON, output non-overwrite, and a 25-recipient ceiling; negative tests rejected wrong audience, non-double-opt-in, future consent, full suppression, over-limit audiences, insecure/duplicate/address-exposing URLs, and inputs inside the kit | Establishes deterministic local consent/suppression and unsubscribe-input gating without persisting addresses or tokens in artifacts; it does not prove a production consent store, working unsubscribe endpoint, provider event handling, sender authentication, or live-delivery authorization |
| Subscription registry and endpoints | 2026-07-17 | Two reserved-domain consents imported into a mode-0600 SQLite registry; exported tokens decoded without email/audience data; GET retained both active rows; invalid-media POST was rejected; valid and repeated one-click POST produced one stable suppression; signed raw-body bounce feedback suppressed one active address, its replay was idempotent, a forged signature was rejected, and a provider-suppressed address was blocked from later import; registry exports passed the email preflight with no address/token leakage | Establishes the reference storage, token, unsubscribe, signed-feedback, replay, global-suppression, and export behavior locally; it does not prove TLS proxy configuration, production durability, original double opt-in, abuse resistance, privacy operations, provider registration, or public availability |
| Double-opt-in signup | 2026-07-17 | Reserved-domain tests created a short-lived pending request, verified identity-free purpose-separated tokens, kept confirmation GET read-only, activated on explicit POST, made repeats idempotent, retried uncertain confirmation delivery with the same provider key after cooldown, omitted unsubscribe headers before consent, rejected expired tokens, and withheld mail from a globally bounced address | Establishes local consent-state and confirmation-delivery behavior; it does not prove sender-domain authentication, inbox placement, public rate limiting, privacy operations, or production HTTPS durability |
| Native carousel theme seam | 2026-07-17 | Added an original `editorial-light` renderer alongside the default `intelligence-dark`; both produce deterministic matched SVG/PNG files from the same validated model, retain evidence and false publish authorization, bind the selected theme into the manifest, reject unknown themes, and pass the existing Instagram approval verifier. Representative cover, scale, and CTA PNGs were visually inspected at 1080×1350 with no clipping | Improves presentation choice without adopting an unescaped browser renderer or weakening provenance; it does not establish audience preference, Instagram acceptance, or automatic publishing rights |
| Carousel palette contrast | 2026-07-17 | Relative-luminance tests measured header, counter/evidence, headline, body, evidence title, and footer against every gradient stop at a minimum 4.5:1, plus 20px bold role labels against their 16%-accent pills at a minimum 3:1. The light muted color improved from a measured 4.07:1 worst case to above the floor, and the regenerated scale card was visually inspected | Establishes deterministic palette-level contrast; it does not replace zoom, low-vision, screen-reader, feed-size, or platform-client testing |
| Instagram caption draft | 2026-07-17 | The distributor generated a period-labelled caption from all three validated signals, included the evidence entry point and non-causal/non-investment scope note, enforced a project length ceiling, recorded its path and digest in the manifest, and rejected an independently replaced caption at approval | Closes the manual upload-content gap while preserving one-source consistency; it does not prove hashtag performance, link clickability in a client, platform acceptance, or editorial approval |
| Instagram manual preflight | 2026-07-17 | An aggregate-only CLI required an unchanged Instagram approval, decoded six 1080×1350 PNGs, checked caption length/evidence/scope/unique restrained tags and six alt-text evidence sections, rejected email-only approval and tampering, refused output inside the kit/overwrite, and retained false credential/content/publish flags | Establishes a reproducible local handoff to private platform preview; it does not prove account permissions, Instagram rendering, upload success, reach, or audience value |
| Provider pilot adapter | 2026-07-17 | Reserved-domain integration tests bound a short-lived launch approval to an unchanged preflight, sent two isolated requests through a synthetic provider, verified visible and RFC one-click links plus distinct idempotency keys, skipped accepted rows, and stopped an uncertain outcome. Provider-confirmed acceptance then remained skipped; confirmed non-acceptance rejected the same authorization and completed only under a different short-lived authorization while retaining its idempotency key. Repeated/contradictory resolutions and PII-bearing audit labels were tested, and a TLS mock verified the Resend HTTP payload and bounded errors | Establishes local launch, rendering, request, reconciliation, and duplicate-safety behavior without leaking addresses/tokens into authorization or ledger files; it does not prove a verified sender domain, inbox delivery, provider-account settings, public webhook operation, or a live consented audience |
| Maintainer browser inspection | 2026-07-17 | Synthetic publication rendered at 1440×1000 and 390×844 with three ready signals, enabled draft actions, no horizontal overflow, and no console warnings or errors | Establishes a bounded local responsive rendering check; does not replace assistive-technology or external-reader evaluation |
| Production artifact | Not measured yet | Public `data/briefing.json` has not been verified | Required after merge and a semiconductor refresh |
| External-reader value | Not measured yet | No reader has evaluated the email or carousel draft | Required before conducting the provider-backed pilot or automating any social distribution |

### Decision and next step

Keep the provider-neutral draft contract and deterministic PNG renderer as a release candidate. Next, verify the public artifact after deployment, use the exported Markdown with a small consented test audience, and privately preview the generated PNG sequence with reviewed alt text before selecting an email provider or enabling any social integration. Direct publishing remains a separate decision after citation legibility, rights, platform, and audience-value review.

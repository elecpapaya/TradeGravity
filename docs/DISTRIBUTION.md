# Reviewed distribution workflow

TradeGravity uses one evidence contract for its website briefing, email draft, and future social cards. The workflow is deliberately split into **analysis**, **rendering**, **editorial approval**, and **delivery** so that generating an asset never authorizes sending or publishing it.

## Build a local kit

Start from a validator-accepted `briefing.json` and the public URL at which its cited evidence will be available:

```bash
go run ./cmd/distributor \
  -briefing examples/sample-data/briefing.json \
  -out distribution-kit \
  -base-url https://elecpapaya.github.io/TradeGravity/ \
  -theme intelligence-dark
```

`-theme` accepts `intelligence-dark` (the default analytical dark treatment) or `editorial-light` (a restrained light editorial treatment). Both are original, network-free native Go themes over the same validated six-slide model. Theme selection changes the PNG/SVG bytes and is recorded in the manifest, so changing it requires a new kit and approval.

The command refuses unavailable or automatically publishable briefing contracts, insecure public base URLs, and an output directory that already exists. It performs no network request and sends nothing.

The generated directory contains:

```text
distribution-kit/
├── manifest.json
├── REVIEW.md
├── email/
│   ├── subject.txt
│   ├── preview.txt
│   ├── body.md
│   └── body.html
└── carousel/
    ├── index.html
    ├── alt-text.md
    ├── caption.md
    ├── slide-01.svg … slide-06.svg
    └── slide-01.png … slide-06.png
```

After review, the approval command adds `approval.json` beside the manifest. It is intentionally absent from a newly built kit.

`carousel/caption.md` is a review-pending Instagram caption derived only from the three validated signals. It retains the comparison period, evidence entry point, scope warning, and restrained topic tags. The project imposes a 1,800-rune editorial ceiling. Edit the source briefing and regenerate instead of changing the caption independently.

`manifest.json` records the edition, public evidence base, email CTA, selected native theme, caption path, 1080×1350 dimensions, available `png` and `svg` formats, review-pending state, explicit false send/publish authorization, and a byte count and SHA-256 digest for every reviewable file. The manifest is deterministic for the same briefing, base URL, and theme.

## Editorial review

Open `carousel/index.html` locally and preview `email/body.html` on desktop and mobile. Complete `REVIEW.md` before moving any file to an external provider. At minimum, verify:

- all periods, values, directions, and source links against the cited JSON;
- month-to-month movement is not confused with a publish-to-publish revision;
- the email has one primary CTA back to the evidence;
- the six 4:5 cards remain legible at feed size and have reviewed alt text; and
- caveats do not become causal, routing, capacity, alignment, or investment claims.

The SVG files are editable, resolution-independent originals. The matching PNG files are deterministic 1080×1350 raster assets generated from the same validated slide model; `carousel/index.html` deliberately previews those PNGs so the editor reviews the files intended for upload. Both formats and their SHA-256 hashes are created together, but they remain drafts. If copy or citations change, regenerate the whole kit instead of editing a PNG in place. Platform preview, alt-text entry, caption review, and the final Instagram publish action stay manual.

The PNG renderer uses embedded Go fonts and makes no network request or external-font fetch. This keeps the output repeatable in CI and avoids silently changing typography between builds. Review every card at actual feed size because successful decoding and correct dimensions do not prove platform acceptance or reader comprehension.

Both native palettes are tested across all three gradient stops. Normal text colors must retain at least 4.5:1 contrast, and the 20px bold role labels must retain at least 3:1 against their translucent pills. This automated contrast floor complements—rather than replaces—the feed-size visual and assistive-technology review.

## Record content approval

Complete `REVIEW.md`, then bind the unchanged manifest and all 20 tracked files to a named reviewer, non-sensitive audience label, channel list, and explicit UTC time:

```bash
go run ./cmd/distribution-approval \
  -kit distribution-kit \
  -reviewer elecpapaya \
  -audience consented-internal-pilot \
  -channels email,instagram \
  -approved-at 2026-07-17T12:00:00Z \
  -attest-reviewed
```

The command fails if any tracked file was changed or removed, an untracked file was added, the manifest no longer has its review-pending/false-authorization gates, a channel is unsupported, or `approval.json` already exists. Rebuild the entire kit to make a new approval. Never place recipient addresses or provider secrets in the kit or audience label.

`approval.json` has `scope: "content_release"` and binds the manifest's SHA-256 digest, file count, edition, reviewer, audience label, approved channels, time, and fixed attestations. It deliberately keeps `provider_delivery_ready`, `subscriber_consent_ready`, and `automatic_publish_ready` false. A future delivery adapter can call the same verifier and require the intended channel, but must enforce its own consent, suppression, sender, and provider gates.

The approval record proves consistency with the local manifest; it does not authenticate the human reviewer by itself. Preserve it in a protected commit, release artifact, or separately signed record if reviewer authenticity is required.

## Validate the manual Instagram package

After an Instagram-channel approval, run `cmd/instagram-preflight` to recheck the unchanged manifest, six PNG dimensions, caption evidence/scope/tags, and all six alt-text sections. The aggregate output remains outside the kit and cannot publish anything. See [Instagram manual-publish preflight](INSTAGRAM_PREFLIGHT.md).

## Manual GitHub Actions build

The **Build reviewed distribution kit** workflow lets the operator choose either supported native theme, reads the validator-accepted `gh-pages` publication, builds the kit, verifies the theme and false social authorization in the manifest, and uploads it as a 14-day Actions artifact. It is manually dispatched and has read-only repository permissions. It does not have an email token, subscriber list, or social credential.

## Delivery gate

Provider-backed sending remains a separate operator-controlled step. Before enabling it, document and test:

- double opt-in and the exact subscription promise;
- one-click unsubscribe and a durable suppression list;
- sender identity, SPF, DKIM, and DMARC;
- bounce and complaint handling;
- privacy notice, retention, deletion, and data-processing terms;
- secret storage outside the browser and generated artifact; and
- an approval record that binds one reviewed edition and its final hashes to one channel and audience.

The content-release record implements only the editorial part of the last item. Run the provider-neutral [email delivery preflight](EMAIL_DELIVERY_PREFLIGHT.md) locally to validate a private double-opt-in CSV, apply a private suppression CSV, and create an aggregate-only plan. The preflight keeps provider configuration and delivery authorization false.

The [provider-backed email pilot](EMAIL_PROVIDER_PILOT.md) adds a second, short-lived launch approval plus a fail-closed Resend adapter. It replays the exact preflight inputs immediately before sending and records accepted or uncertain attempts in a private HMAC-keyed SQLite ledger. Merely building, approving, or preflighting a kit still cannot send email; the live command additionally requires the matching authorization, provider and ledger secrets, and `-send-live`.

TradeGravity should not use open tracking pixels by default. If aggregate link measurement is later added, disclose it and keep raw subscriber behavior out of the public dataset.

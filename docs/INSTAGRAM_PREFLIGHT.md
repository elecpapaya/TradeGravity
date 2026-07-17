# Instagram manual-publish preflight

TradeGravity keeps Instagram publishing manual. `cmd/instagram-preflight` verifies that one unchanged, Instagram-approved distribution kit is ready for a private platform preview without configuring an account credential or authorizing a post.

## Run after content approval

First build and approve the kit for the `instagram` channel as described in [DISTRIBUTION.md](DISTRIBUTION.md). Then write the aggregate preflight outside the kit:

```bash
go run ./cmd/instagram-preflight \
  -kit distribution-kit \
  -out instagram-preflight.json \
  -generated-at 2026-07-17T14:00:00Z
```

The command fails unless:

- `approval.json` covers the `instagram` channel and matches the exact manifest;
- all six manifest-bound PNGs decode at 1080×1350;
- `caption.md` remains within TradeGravity's 1,800-rune editorial ceiling, contains the public evidence base and scope warning, has 1–8 unique simple hashtags, and has no unresolved placeholder;
- `alt-text.md` has one evidence section for each slide; and
- every kit file still matches its recorded byte count and SHA-256 digest.

The output records only edition/theme identities, hashes, counts, dimensions, and boolean checks. It does not contain the caption, hashtags, evidence URL, credentials, or platform account. Its fixed controls remain:

```json
{
  "contains_caption_text": false,
  "contains_credentials": false,
  "manual_upload_required": true,
  "automatic_publish_authorized": false
}
```

The file is not part of the approved kit and the command refuses to write it inside that directory or overwrite an existing output. It is ignored at the repository root by default.

## Private platform preview

After preflight, use Instagram's own draft or private preview surface manually. Confirm ordering, crop, line wrapping, caption appearance, hashtag wording, alt-text entry, account identity, and rights. Do not treat a passing local preflight as proof that Instagram accepted or published the carousel. Rebuild and reapprove after any content change.

Automatic publishing would require a separate credential-bearing adapter, explicit account and permission checks, platform idempotency, post-result reconciliation, and a new short-lived launch authorization. None of those capabilities are implemented or implied here.

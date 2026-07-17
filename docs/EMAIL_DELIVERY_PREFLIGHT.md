# Email delivery preflight

TradeGravity keeps subscriber addresses outside the repository, static site, distribution kit, approval record, and aggregate preflight artifact. The preflight CLI is a provider-neutral gate between reviewed content and any future network sender. It validates double opt-in evidence, applies a durable suppression list, enforces a small pilot ceiling, and then emits only counts and source-file digests. It sends no email.

## Inputs

Start with a distribution kit whose `approval.json` includes the `email` channel. Keep both CSV files in a private local directory outside the kit.

The subscriber CSV header must be exactly:

```csv
email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,unsubscribe_url
reader@example.invalid,consented-internal-pilot,active,2026-07-10T01:00:00Z,double_opt_in,website-form,v1,https://subscriptions.example.invalid/u/opaque-recipient-token
```

Every subscriber row must:

- match the non-sensitive audience label in `approval.json`;
- have `status` equal to `active` and `consent_method` equal to `double_opt_in`;
- contain an RFC 3339 consent time that is not in the future;
- record a consent source and privacy-notice version; and
- use a unique plain email addr-spec without a display name; and
- provide a unique absolute HTTPS unsubscribe URL containing an opaque recipient token, with no credentials, fragment, or decoded email address.

The suppression CSV header must be exactly:

```csv
email,reason,suppressed_at
former-reader@example.invalid,unsubscribed,2026-07-12T03:00:00Z
```

Allowed reasons are `unsubscribed`, `bounced`, `complaint`, `invalid`, and `manual`. An empty suppression list still needs the header. Suppression always wins over active consent.

## Run the preflight

```bash
go run ./cmd/distribution-preflight \
  -kit distribution-kit \
  -subscribers private/subscribers.csv \
  -suppressions private/suppressions.csv \
  -out delivery-preflight.json \
  -generated-at 2026-07-17T12:30:00Z \
  -max-recipients 25
```

The command refuses:

- a kit without a valid email content approval;
- changed, missing, or untracked kit files;
- subscriber or suppression inputs stored inside the kit;
- malformed, duplicate, future-dated, wrong-audience, or non-double-opt-in subscriber rows;
- malformed or duplicate suppression rows;
- an empty post-suppression audience; and
- an eligible audience above the explicit pilot limit.

`delivery-preflight.json` uses file mode `0600` where the operating system supports it and refuses to overwrite an existing file. It records the edition, manifest and approval digests, audience label, subscriber/suppression file digests, aggregate counts, template digest, required unsubscribe headers, and the pilot limit. It contains no recipient addresses, unsubscribe URLs, tokens, or local source paths.

## What remains intentionally false

A passing plan has:

```json
{
  "consent_validated": true,
  "suppression_applied": true,
  "unsubscribe_urls_validated": true,
  "contains_recipient_addresses": false,
  "provider_configured": false,
  "delivery_authorized": false
}
```

The approved HTML and Markdown templates contain exactly one `{{UNSUBSCRIBE_URL}}` placeholder in addition to the single primary evidence CTA. A future sender must replace that placeholder with a recipient-specific HTTPS unsubscribe URL, set `List-Unsubscribe` to that HTTPS URI, set `List-Unsubscribe-Post` to `List-Unsubscribe=One-Click`, and ensure a valid DKIM signature covers both headers. These are the one-click sender requirements defined by [RFC 8058](https://www.rfc-editor.org/rfc/rfc8058.html); Gmail's current sender guidance also requires a visible body link for subscribed messages and one-click support for high-volume senders. ([Gmail sender guidelines](https://support.google.com/mail/answer/81126))

The reference [provider-backed email pilot](EMAIL_PROVIDER_PILOT.md) re-runs the same in-memory preflight immediately before sending, requires unchanged source digests, and obtains a separate short-lived launch authorization. The operator must still authenticate the sender domain and deploy/test the signed Resend feedback handler so bounce, complaint, and provider-suppression events update the private registry before issuing that authorization.

For a provider-neutral implementation that generates these URLs and exports both private CSVs, see the [subscription registry and unsubscribe service](UNSUBSCRIBE_SERVICE.md). It imports existing double-opt-in evidence; it does not create or prove the original opt-in.

Do not upload the CSV inputs to GitHub Actions artifacts. Do not print the eligible in-memory address list, embed it in logs, or copy it into the aggregate JSON. A passing preflight is never a live-send authorization.

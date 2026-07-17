# Provider-backed email pilot

TradeGravity includes a bounded Resend adapter for a deliberately small, consented email pilot. It is not connected to the static site or GitHub Actions and it does not run on a schedule. A live provider request is possible only when the reviewed kit, private subscriber and suppression inputs, aggregate preflight, short-lived launch authorization, provider API key, delivery-ledger secret, and explicit `-send-live` acknowledgement all agree.

The adapter follows one-email/one-job lifecycle guidance: each semiconductor brief has one primary evidence CTA, one visible unsubscribe link, and no tracking pixel. It sends one provider request per recipient rather than placing multiple readers in `To`, `Cc`, or `Bcc`.

Audience consent may be imported from an existing verified double-opt-in source or collected by the default-off signup/confirmation flow documented in [UNSUBSCRIBE_SERVICE.md](UNSUBSCRIBE_SERVICE.md). Confirmation mail is transactional and does not itself add an address to a briefing audience; only the confirmation-page POST does so.

## Security boundary

Keep these files and values outside the repository, distribution kit, public site, CI logs, and Actions artifacts:

- subscriber and suppression CSVs;
- `delivery-preflight.json` and `email-launch-authorization.json`;
- `delivery-ledger.db` plus its WAL/SHM files and backups;
- `RESEND_API_KEY`; and
- `TRADEGRAVITY_DELIVERY_SECRET`, a stable random value of at least 32 bytes.

The delivery ledger stores edition and audience labels, content digests, provider/idempotency identifiers, times, and HMAC-derived recipient keys. It does not store recipient addresses, unsubscribe URLs, or rendered bodies. File mode is requested as `0600` where supported, but the operator still needs encrypted storage, access control, backups, retention, and deletion procedures.

The Resend endpoint is compiled as `https://api.resend.com/emails`; it cannot be redirected with a CLI flag that might exfiltrate the API key. HTTP timeouts and response-size limits are enforced, and provider error bodies are not printed.

## 1. Produce the aggregate preflight

Follow [Email delivery preflight](EMAIL_DELIVERY_PREFLIGHT.md) and keep the resulting files private:

```bash
go run ./cmd/distribution-preflight \
  -kit distribution-kit \
  -subscribers private/subscribers.csv \
  -suppressions private/suppressions.csv \
  -out private/delivery-preflight.json \
  -generated-at 2026-07-17T12:30:00Z \
  -max-recipients 25
```

## 2. Record a short-lived launch approval

Verify the Resend sender domain, SPF, DKIM, and DMARC; deploy and test both the HTTPS unsubscribe endpoint and the signed feedback endpoint described in [UNSUBSCRIBE_SERVICE.md](UNSUBSCRIBE_SERVICE.md); prove that `email.bounced`, `email.complained`, and `email.suppressed` events become durable suppressions; review privacy controls; and inspect the final eligible pilot list. Then create an authorization that expires within one hour:

```bash
go run ./cmd/email-launch-approval \
  -preflight private/delivery-preflight.json \
  -out private/email-launch-authorization.json \
  -provider resend \
  -from 'TradeGravity <briefs@example.org>' \
  -reply-to maintainer@example.org \
  -authorized-by elecpapaya \
  -authorized-at 2026-07-17T12:35:00Z \
  -expires-at 2026-07-17T13:05:00Z \
  -attest-domain-authenticated \
  -attest-feedback-ready \
  -attest-privacy-reviewed \
  -attest-pilot-recipients
```

The JSON contains aggregate identities and digests but no recipient address or token. It refuses overwrite. Changing the preflight, manifest, approval, email template, subscriber file, suppression file, recipient count, audience, provider, or sender requires a new authorization.

## 3. Send the bounded pilot

Set secrets without writing them into shell history, then run within the authorization window:

```bash
export RESEND_API_KEY='read-from-your-secret-manager'
export TRADEGRAVITY_DELIVERY_SECRET='at-least-32-stable-random-bytes'

go run ./cmd/email-delivery \
  -kit distribution-kit \
  -subscribers private/subscribers.csv \
  -suppressions private/suppressions.csv \
  -preflight private/delivery-preflight.json \
  -authorization private/email-launch-authorization.json \
  -ledger private/delivery-ledger.db \
  -send-at 2026-07-17T12:40:00Z \
  -send-live
```

Immediately before the first provider request, the command reruns content approval, double-opt-in, suppression, audience, URL, and pilot-ceiling checks. It compares all live digests and counts with both the original preflight and launch approval. Each request includes:

- one recipient;
- the approved HTML and Markdown bodies with that recipient's HTTPS link;
- `List-Unsubscribe: <https://…>`;
- `List-Unsubscribe-Post: List-Unsubscribe=One-Click`; and
- a stable provider `Idempotency-Key` derived from the edition and a secret HMAC recipient identity.

Resend documents custom email headers and `Idempotency-Key` on `POST /emails`; its idempotency window is currently 24 hours. The local ledger is therefore the longer-lived duplicate guard. See [Resend Send Email](https://resend.com/docs/api-reference/emails/send-email) and [Resend idempotency keys](https://resend.com/docs/dashboard/emails/idempotency-keys).

## Failure and reconciliation

An accepted ledger row is skipped on every later run. If the process cannot prove provider acceptance, it leaves a `pending` row and stops. A later run refuses to resend that recipient automatically—even if Resend's 24-hour idempotency window has elapsed. This trades possible non-delivery for protection against an accidental duplicate.

Do not delete or edit the ledger to force a retry. First reconcile the pending idempotency key against the provider dashboard and logs. The signed feedback handler records later suppression outcomes, but it cannot prove whether an unresolved send request was accepted; that delivery-ledger reconciliation remains an operator decision. Treat any pending row as a stopped pilot until the provider outcome is known.

If the provider confirms acceptance, record the exact provider message ID. The recipient argument is used only in memory to derive the existing HMAC ledger key and is neither printed nor stored:

```bash
go run ./cmd/email-delivery-reconcile \
  -ledger private/delivery-ledger.db \
  -edition EDITION_ID \
  -audience consented-internal-pilot \
  -recipient 'private-recipient@example.org' \
  -outcome accepted \
  -provider-message-id PROVIDER_MESSAGE_ID \
  -resolved-by elecpapaya \
  -evidence resend-dashboard-message-match \
  -resolved-at 2026-07-17T12:50:00Z \
  -attest-provider-checked
```

If the provider positively confirms that it did **not** accept the request, use `-outcome not_accepted` and omit `-provider-message-id`. That outcome remains in the ledger as an audit record. The same launch authorization still cannot retry it: create a new unexpired authorization file and run delivery at a later time. The provider idempotency key remains stable across the reconciled retry.

`resolved-by`, `evidence`, and provider message IDs are bounded non-sensitive labels; addresses and URLs are rejected in the stored audit fields. A contradictory second outcome is rejected, while an identical reconciliation is idempotent.

## Current limits

- No live TradeGravity send has been performed or implied by these tools.
- Provider account creation, domain verification, billing, data-processing terms, webhook registration, and HTTPS deployment are external operator actions.
- Signed bounce, complaint, and provider-suppression ingestion is implemented, but public HTTPS deployment, provider webhook registration, secret rotation, monitoring, backup/restore, and a real replay drill remain operator responsibilities.
- Link/open tracking is not enabled; provider-account defaults must also be reviewed.
- The first pilot should remain small, manually observed, and limited to readers who explicitly expect this edition.
- Instagram cards remain review-gated manual uploads; email delivery never authorizes social publishing.

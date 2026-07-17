# Private subscription registry and unsubscribe service

TradeGravity includes a private reference service for double-opt-in signup, working unsubscribe URLs, and durable suppressions. The static dashboard still never receives an address or API key. The separately deployed service can send a short transactional confirmation through Resend, activate consent only after an explicit confirmation POST, import existing verified consent, export private delivery CSVs, and verify signed provider feedback.

## Security boundary

Keep all of the following outside the repository, `site/data/`, GitHub Actions, Actions artifacts, and the distribution kit:

- `subscriptions.db`, its WAL/SHM files, and backups;
- the source consent CSV;
- exported subscriber and suppression CSVs; and
- `TRADEGRAVITY_UNSUBSCRIBE_SECRET`; and
- `RESEND_WEBHOOK_SECRET` when provider feedback is enabled.
- `RESEND_API_KEY` when public signup is enabled.

Use a dedicated directory on an encrypted volume with access limited to the operator account. The tools request mode `0600` for the database and exports where supported, but file mode alone is not encryption or a backup policy.

The secret must contain at least 32 bytes and remain stable while issued links are in circulation. Store it in a secret manager or password manager. Rotating or losing it invalidates existing links. The token contains only a version and random 128-bit subscription ID, authenticated with HMAC-SHA-256; it contains no email address or audience label.

## Import verified consent and create private exports

The source CSV must be produced by a real double-opt-in process and have this exact header:

```csv
email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version
reader@example.invalid,consented-internal-pilot,active,2026-07-10T01:00:00Z,double_opt_in,website-form,v1
```

Set the same secret for registry and service processes, then import and export:

```bash
export TRADEGRAVITY_UNSUBSCRIBE_SECRET='at-least-32-random-secret-bytes'

go run ./cmd/subscription-registry \
  -db private/subscriptions.db \
  -base-url https://subscriptions.example.org/tradegravity/ \
  -consents private/verified-consents.csv \
  -audience consented-internal-pilot \
  -out-subscribers private/delivery-subscribers.csv \
  -out-suppressions private/delivery-suppressions.csv \
  -imported-at 2026-07-17T12:00:00Z
```

The command validates active double opt-in, exact audience labels, timestamps, uniqueness, source, and privacy-notice version. It refuses to overwrite exports. Reimporting an active record updates its consent evidence only when it is not older than the stored record. Reimporting a suppressed record never reactivates it.

The subscriber export has the exact preflight schema and a stable, unique HTTPS unsubscribe URL for each active row. The suppression export contains unsubscribed addresses and timestamps. Feed both directly to [`cmd/distribution-preflight`](EMAIL_DELIVERY_PREFLIGHT.md); never copy them into the repository.

## Enable double-opt-in signup

Signup is disabled unless explicitly configured. After publishing the privacy notice and verifying the sender domain, enable it on the same private service:

```bash
export TRADEGRAVITY_UNSUBSCRIBE_SECRET='the-same-stable-secret'
export RESEND_API_KEY='read-from-your-secret-manager'

go run ./cmd/unsubscribe-service \
  -db private/subscriptions.db \
  -base-url https://subscriptions.example.org/tradegravity/ \
  -listen 127.0.0.1:8081 \
  -enable-signup \
  -signup-audience tradegravity-briefing \
  -consent-source public-subscribe-form \
  -privacy-notice-version v1 \
  -privacy-notice-url https://example.org/privacy \
  -confirmation-from 'TradeGravity <confirm@example.org>'
```

The form is `/tradegravity/subscribe` and the confirmation page is `/tradegravity/confirm`. A request creates a pending record for 30 minutes and sends one transactional email with a stable provider idempotency key. The token contains only a random ID. Neither signup nor confirmation responses disclose whether an address was active or globally suppressed. `GET` on the confirmation link is read-only; only a form-encoded `POST` activates the audience membership. An address that previously unsubscribed may re-consent through this flow, while a bounced, complaining, invalid, or provider-suppressed address cannot. Confirmation mail intentionally has no newsletter unsubscribe header because consent has not yet been completed.

The built-in pending-record cap and one-minute resend cooldown are only backstops. Before exposing the form, apply IP/request-rate limits at the reverse proxy, prevent request-body/query logging, monitor aggregate failures, and consider a privacy-preserving challenge. Do not add `RESEND_API_KEY` to the static site or GitHub Actions.

## Run the unsubscribe endpoint

The Go process listens on loopback by default and expects a production TLS reverse proxy:

```bash
export TRADEGRAVITY_UNSUBSCRIBE_SECRET='the-same-stable-secret'

go run ./cmd/unsubscribe-service \
  -db private/subscriptions.db \
  -base-url https://subscriptions.example.org/tradegravity/ \
  -listen 127.0.0.1:8081
```

For that base URL, the public endpoint is `/tradegravity/unsubscribe`; `/healthz` checks database readiness.

The reverse proxy must:

- terminate HTTPS and preserve the full path, query string, request body, and method;
- allow `GET` and form-encoded `POST` on the unsubscribe path;
- avoid logging the `token` query value or forwarding it to analytics;
- preserve `Cache-Control: no-store` and the supplied security headers;
- impose connection and request-rate limits without requiring cookies or login; and
- keep the service bound to a private interface.

`GET` validates the token and displays a confirmation form but never changes subscription state, preventing ordinary link previews and scanners from silently unsubscribing a reader. A form-encoded `POST` with `List-Unsubscribe=One-Click` records `unsubscribed` once; repeated valid requests succeed without changing the original suppression time. No response includes the subscriber address.

This matches the one-click body defined by [RFC 8058](https://www.rfc-editor.org/rfc/rfc8058.html). The pilot email sender puts the same URL in the visible body and `List-Unsubscribe` header and sets `List-Unsubscribe-Post: List-Unsubscribe=One-Click`; the configured provider/domain must ensure DKIM covers both headers.

## Enable signed Resend feedback

Register only the suppression-relevant Resend events—`email.bounced`, `email.complained`, and `email.suppressed`—against:

```text
https://subscriptions.example.org/tradegravity/webhooks/resend
```

Store the endpoint signing secret separately, then enable the route explicitly:

```bash
export TRADEGRAVITY_UNSUBSCRIBE_SECRET='the-same-stable-secret'
export RESEND_WEBHOOK_SECRET='whsec_read-from-your-secret-manager'

go run ./cmd/unsubscribe-service \
  -db private/subscriptions.db \
  -base-url https://subscriptions.example.org/tradegravity/ \
  -listen 127.0.0.1:8081 \
  -enable-resend-webhook
```

The handler reads at most 64 KiB, verifies the signature against the untouched raw body and `svix-id`, `svix-timestamp`, and `svix-signature` headers, and only then parses the event. It expects one recipient because the pilot sender emits one provider request per recipient. Event IDs are stored without message bodies and deduplicated for at-least-once delivery. A verified event suppresses every active audience membership for the address and creates a global address suppression that blocks later consent imports. Replays return success without changing the original suppression.

The reverse proxy must preserve those three signature headers and the raw body exactly, allow only POST on the feedback path, avoid request/body logging, and return the service response unchanged so Resend can retry failures. Signature verification follows [Resend's raw-body guidance](https://resend.com/docs/webhooks/verify-webhooks-requests); replay handling follows its documented `svix-id` at-least-once contract.

## Current limits

This is a single-instance reference service, not a hosted mailing platform. Before public use, provide and test:

- production abuse protection for sign-up, confirmation, and unsubscribe traffic;
- encrypted backups, restore drills, retention, deletion, and operator-access audit;
- monitoring without query-token or address logging;
- a deployment plan for SQLite durability or a migration to a managed transactional store;
- production registration, secret rotation, replay drills, and monitoring for the signed feedback endpoint; and
- authenticated, idempotent launch authorization for delivery.

Do not link the public TradeGravity site to this service until those controls, the privacy notice, verified sender, and public HTTPS deployment are ready.

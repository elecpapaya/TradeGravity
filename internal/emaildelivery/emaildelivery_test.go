package emaildelivery

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tradegravity/internal/deliverypreflight"
	"tradegravity/internal/distributionkit"
)

func TestAuthorizationBindsPreflightWithoutRecipientData(t *testing.T) {
	fixture := newDeliveryFixture(t)
	authorization, raw, err := Authorize(fixture.authorizationRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !authorization.DeliveryAuthorized || authorization.EligibleRecipients != 2 || authorization.Provider != "resend" {
		t.Fatalf("unexpected authorization: %+v", authorization)
	}
	if bytes.Contains(raw, []byte("example.invalid")) || bytes.Contains(raw, []byte("token=")) {
		t.Fatal("launch authorization leaked a recipient address or token")
	}
	path := filepath.Join(fixture.root, "launch.json")
	if err := WriteAuthorization(path, raw); err != nil {
		t.Fatal(err)
	}
	if err := WriteAuthorization(path, raw); err == nil {
		t.Fatal("WriteAuthorization() overwrote an existing authorization")
	}
	preflightRaw, err := os.ReadFile(fixture.preflightPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAuthorization(path, preflightRaw, fixture.sendAt)
	if err != nil || loaded.PreflightSHA256 != authorization.PreflightSHA256 {
		t.Fatalf("LoadAuthorization() = %+v, %v", loaded, err)
	}

	tampered := append([]byte(nil), preflightRaw...)
	tampered = append(tampered, ' ')
	if _, err := LoadAuthorization(path, tampered, fixture.sendAt); err == nil {
		t.Fatal("LoadAuthorization() accepted a changed preflight")
	}
}

func TestAuthorizationRejectsMissingControlsAndLongWindows(t *testing.T) {
	fixture := newDeliveryFixture(t)
	request := fixture.authorizationRequest()
	request.Attestations.BounceComplaintReady = false
	if _, _, err := Authorize(request); err == nil {
		t.Fatal("Authorize() accepted a missing feedback-control attestation")
	}
	request = fixture.authorizationRequest()
	request.ExpiresAt = request.AuthorizedAt.Add(time.Hour + time.Second)
	if _, _, err := Authorize(request); err == nil {
		t.Fatal("Authorize() accepted a window longer than one hour")
	}
	request = fixture.authorizationRequest()
	request.Sender = "bad\r\nBcc: victim@example.invalid"
	if _, _, err := Authorize(request); err == nil {
		t.Fatal("Authorize() accepted sender header injection")
	}
}

func TestDeliverSendsOnceWithOneClickHeadersAndPrivateLedger(t *testing.T) {
	fixture := newDeliveryFixture(t)
	authorizationPath := fixture.writeAuthorization(t)
	provider := &recordingProvider{}
	request := fixture.deliveryRequest(authorizationPath, provider)

	first, err := Deliver(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Eligible != 2 || first.Accepted != 2 || first.Skipped != 0 || first.Pending != 0 || len(provider.messages) != 2 {
		t.Fatalf("unexpected first delivery: result=%+v provider_calls=%d", first, len(provider.messages))
	}
	for _, message := range provider.messages {
		if strings.Contains(message.HTML, unsubscribePlaceholder) || strings.Contains(message.Text, unsubscribePlaceholder) {
			t.Fatal("provider message retained the unsubscribe placeholder")
		}
		if !strings.HasPrefix(message.ListUnsubscribe, "<https://") || !strings.HasSuffix(message.ListUnsubscribe, ">") || message.ListUnsubscribePost != "List-Unsubscribe=One-Click" {
			t.Fatalf("invalid one-click headers: %+v", message)
		}
		if strings.Count(message.HTML, "token=") != 1 || strings.Count(message.Text, "token=") != 1 {
			t.Fatal("visible recipient unsubscribe link was not rendered exactly once per body")
		}
	}
	if provider.idempotencyKeys[0] == provider.idempotencyKeys[1] {
		t.Fatal("different recipients shared an idempotency key")
	}

	second, err := Deliver(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if second.Accepted != 2 || second.Skipped != 2 || second.Pending != 0 || len(provider.messages) != 2 {
		t.Fatalf("accepted deliveries were sent again: result=%+v provider_calls=%d", second, len(provider.messages))
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		raw, err := os.ReadFile(fixture.ledgerPath + suffix)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("example.invalid")) || bytes.Contains(raw, []byte("token=")) {
			t.Fatalf("delivery ledger %s leaked a recipient address or token", suffix)
		}
	}
}

func TestDeliverLeavesUnknownProviderResultPendingAndRefusesRetry(t *testing.T) {
	fixture := newDeliveryFixture(t)
	authorizationPath := fixture.writeAuthorization(t)
	provider := &recordingProvider{fail: true}
	request := fixture.deliveryRequest(authorizationPath, provider)

	result, err := Deliver(context.Background(), request)
	if err == nil || result.Pending != 1 || len(provider.messages) != 1 {
		t.Fatalf("unknown provider result was not left pending: result=%+v err=%v calls=%d", result, err, len(provider.messages))
	}
	provider.fail = false
	result, err = Deliver(context.Background(), request)
	if !errors.Is(err, ErrDeliveryPending) || len(provider.messages) != 1 {
		t.Fatalf("pending delivery was retried: result=%+v err=%v calls=%d", result, err, len(provider.messages))
	}
}

func TestReconcileNotAcceptedRequiresNewAuthorizationBeforeRetry(t *testing.T) {
	fixture := newDeliveryFixture(t)
	authorizationPath := fixture.writeAuthorization(t)
	provider := &recordingProvider{fail: true}
	request := fixture.deliveryRequest(authorizationPath, provider)
	if _, err := Deliver(context.Background(), request); err == nil {
		t.Fatal("initial synthetic provider failure was accepted")
	}

	ledger := fixture.openLedger(t)
	resolvedAt := fixture.sendAt.Add(time.Minute)
	reconciled, err := ledger.Reconcile(context.Background(), ReconciliationRequest{
		EditionID: fixture.editionID(t), Audience: "pilot-audience", Email: "alpha@example.invalid",
		Outcome: "not_accepted", ResolvedBy: "maintainer", Evidence: "resend-dashboard-no-message",
		ResolvedAt: resolvedAt,
	})
	if err != nil || !reconciled.Changed {
		t.Fatalf("Reconcile(not_accepted) = %+v, %v", reconciled, err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}

	provider.fail = false
	request.SendAt = resolvedAt.Add(30 * time.Second)
	if _, err := Deliver(context.Background(), request); !errors.Is(err, ErrNewAuthorizationRequired) {
		t.Fatalf("same authorization retried reconciled delivery: %v", err)
	}
	if len(provider.messages) != 1 {
		t.Fatalf("provider called again under same authorization: %d", len(provider.messages))
	}

	retryAuthorizedAt := fixture.sendAt.Add(2 * time.Minute)
	retryAuthorization := fixture.writeAuthorizationAt(t, "launch-retry.json", retryAuthorizedAt, retryAuthorizedAt.Add(20*time.Minute))
	request.AuthorizationPath = retryAuthorization
	request.SendAt = retryAuthorizedAt.Add(time.Minute)
	result, err := Deliver(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Accepted != 2 || result.Pending != 0 || len(provider.messages) != 3 {
		t.Fatalf("new authorization did not complete retry and remaining recipient: result=%+v calls=%d", result, len(provider.messages))
	}
	if provider.idempotencyKeys[0] != provider.idempotencyKeys[1] {
		t.Fatal("reconciled retry changed the stable provider idempotency key")
	}
}

func TestReconcileAcceptedSkipsProviderRetry(t *testing.T) {
	fixture := newDeliveryFixture(t)
	authorizationPath := fixture.writeAuthorization(t)
	provider := &recordingProvider{fail: true}
	request := fixture.deliveryRequest(authorizationPath, provider)
	if _, err := Deliver(context.Background(), request); err == nil {
		t.Fatal("initial synthetic provider failure was accepted")
	}

	ledger := fixture.openLedger(t)
	reconciled, err := ledger.Reconcile(context.Background(), ReconciliationRequest{
		EditionID: fixture.editionID(t), Audience: "pilot-audience", Email: "alpha@example.invalid",
		Outcome: "accepted", ProviderMessageID: "provider-confirmed-1", ResolvedBy: "maintainer",
		Evidence: "resend-dashboard-message-match", ResolvedAt: fixture.sendAt.Add(time.Minute),
	})
	if err != nil || !reconciled.Changed {
		t.Fatalf("Reconcile(accepted) = %+v, %v", reconciled, err)
	}
	repeated, err := ledger.Reconcile(context.Background(), ReconciliationRequest{
		EditionID: fixture.editionID(t), Audience: "pilot-audience", Email: "alpha@example.invalid",
		Outcome: "accepted", ProviderMessageID: "provider-confirmed-1", ResolvedBy: "maintainer",
		Evidence: "resend-dashboard-message-match", ResolvedAt: fixture.sendAt.Add(time.Minute),
	})
	if err != nil || !repeated.AlreadyResolved {
		t.Fatalf("repeated accepted reconciliation = %+v, %v", repeated, err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}

	provider.fail = false
	request.SendAt = fixture.sendAt.Add(2 * time.Minute)
	result, err := Deliver(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Accepted != 2 || result.Skipped != 1 || len(provider.messages) != 2 {
		t.Fatalf("accepted reconciliation was resent: result=%+v calls=%d", result, len(provider.messages))
	}
}

func TestReconcileRejectsPIIInAuditFieldsAndContradictoryOutcome(t *testing.T) {
	fixture := newDeliveryFixture(t)
	authorizationPath := fixture.writeAuthorization(t)
	provider := &recordingProvider{fail: true}
	request := fixture.deliveryRequest(authorizationPath, provider)
	if _, err := Deliver(context.Background(), request); err == nil {
		t.Fatal("initial synthetic provider failure was accepted")
	}
	ledger := fixture.openLedger(t)
	defer ledger.Close()
	base := ReconciliationRequest{
		EditionID: fixture.editionID(t), Audience: "pilot-audience", Email: "alpha@example.invalid",
		Outcome: "not_accepted", ResolvedBy: "maintainer", Evidence: "dashboard-check",
		ResolvedAt: fixture.sendAt.Add(time.Minute),
	}
	withPII := base
	withPII.Evidence = "reader@example.invalid"
	if _, err := ledger.Reconcile(context.Background(), withPII); err == nil {
		t.Fatal("Reconcile() accepted PII in the stored evidence label")
	}
	contradictory := base
	contradictory.ProviderMessageID = "provider-id"
	if _, err := ledger.Reconcile(context.Background(), contradictory); err == nil {
		t.Fatal("Reconcile() accepted a message ID for a not_accepted outcome")
	}
	acceptedWithoutID := base
	acceptedWithoutID.Outcome = "accepted"
	if _, err := ledger.Reconcile(context.Background(), acceptedWithoutID); err == nil {
		t.Fatal("Reconcile() accepted an accepted outcome without a provider message ID")
	}
}

func TestOpenLedgerAddsReconciliationColumnsToEarlierPilotSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delivery-ledger.db")
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`CREATE TABLE deliveries (
		delivery_key TEXT PRIMARY KEY,
		edition_id TEXT NOT NULL,
		audience TEXT NOT NULL,
		recipient_key TEXT NOT NULL,
		provider TEXT NOT NULL,
		content_sha256 TEXT NOT NULL,
		idempotency_key TEXT NOT NULL,
		status TEXT NOT NULL CHECK (status IN ('pending', 'accepted')),
		attempted_at TEXT NOT NULL,
		accepted_at TEXT,
		provider_message_id TEXT,
		UNIQUE(edition_id, audience, recipient_key)
	)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	ledger, err := OpenLedger(path, bytes.Repeat([]byte("m"), 32))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	rows, err := ledger.db.Query(`PRAGMA table_info(deliveries)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		found[name] = true
	}
	for _, name := range []string{"authorization_sha256", "resolution", "resolved_at", "resolved_by", "resolution_evidence"} {
		if !found[name] {
			t.Fatalf("migrated ledger omitted %s", name)
		}
	}
}

func TestDeliverRequiresExplicitLiveAcknowledgement(t *testing.T) {
	_, err := Deliver(context.Background(), DeliveryRequest{})
	if err == nil || !strings.Contains(err.Error(), "send-live") {
		t.Fatalf("Deliver() without live acknowledgement = %v", err)
	}
}

func TestResendProviderRequestContractAndBoundedErrors(t *testing.T) {
	var received struct {
		Authorization string
		Idempotency   string
		Payload       map[string]any
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		received.Authorization = request.Header.Get("Authorization")
		received.Idempotency = request.Header.Get("Idempotency-Key")
		raw, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(raw, &received.Payload)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"id":"provider-message-1"}`))
	}))
	defer server.Close()
	provider, err := NewResendProvider("secret-test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	provider.endpoint = server.URL
	message := Message{
		From: "TradeGravity <brief@example.org>", To: "reader@example.invalid", Subject: "Pilot",
		HTML: "<p>Body</p>", Text: "Body", ListUnsubscribe: "<https://subscriptions.example.org/u?token=opaque>", ListUnsubscribePost: "List-Unsubscribe=One-Click",
	}
	id, err := provider.Send(context.Background(), message, "tradegravity/edition/key")
	if err != nil || id != "provider-message-1" {
		t.Fatalf("provider.Send() = %q, %v", id, err)
	}
	if received.Authorization != "Bearer secret-test-key" || received.Idempotency != "tradegravity/edition/key" {
		t.Fatalf("provider authentication or idempotency header missing: %+v", received)
	}
	headers, ok := received.Payload["headers"].(map[string]any)
	if !ok || headers["List-Unsubscribe"] != message.ListUnsubscribe || headers["List-Unsubscribe-Post"] != message.ListUnsubscribePost {
		t.Fatalf("provider payload omitted one-click headers: %+v", received.Payload)
	}
	to, ok := received.Payload["to"].([]any)
	if !ok || len(to) != 1 || to[0] != message.To {
		t.Fatalf("provider payload did not isolate the recipient: %+v", received.Payload)
	}
	received.Payload = nil
	confirmation := message
	confirmation.ListUnsubscribe = ""
	confirmation.ListUnsubscribePost = ""
	if _, err := provider.Send(context.Background(), confirmation, "tradegravity-confirm/test"); err != nil {
		t.Fatal(err)
	}
	if _, exists := received.Payload["headers"]; exists {
		t.Fatalf("pre-consent confirmation payload contained unsubscribe headers: %+v", received.Payload)
	}
	confirmation.ListUnsubscribe = "<https://subscriptions.example.org/u?token=opaque>"
	if _, err := provider.Send(context.Background(), confirmation, "tradegravity-confirm/test-2"); err == nil {
		t.Fatal("provider accepted only one of the paired unsubscribe headers")
	}

	errorServer := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadRequest)
		_, _ = response.Write([]byte(`{"message":"private recipient detail"}`))
	}))
	defer errorServer.Close()
	provider.client = errorServer.Client()
	provider.endpoint = errorServer.URL
	_, err = provider.Send(context.Background(), message, "tradegravity/edition/key")
	if err == nil || strings.Contains(err.Error(), "private recipient detail") {
		t.Fatalf("provider error leaked its response body: %v", err)
	}
}

type recordingProvider struct {
	messages        []Message
	idempotencyKeys []string
	fail            bool
}

func (provider *recordingProvider) Name() string { return "resend" }

func (provider *recordingProvider) Send(_ context.Context, message Message, idempotencyKey string) (string, error) {
	provider.messages = append(provider.messages, message)
	provider.idempotencyKeys = append(provider.idempotencyKeys, idempotencyKey)
	if provider.fail {
		return "", errors.New("synthetic unknown result")
	}
	return "provider-id-" + string(rune('a'+len(provider.messages)-1)), nil
}

type deliveryFixture struct {
	root            string
	kitDir          string
	subscriberPath  string
	suppressionPath string
	preflightPath   string
	ledgerPath      string
	preflightAt     time.Time
	authorizedAt    time.Time
	sendAt          time.Time
}

func newDeliveryFixture(t *testing.T) deliveryFixture {
	t.Helper()
	root := t.TempDir()
	briefing, err := os.ReadFile(filepath.Join("..", "..", "examples", "sample-data", "briefing.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := distributionkit.Build(briefing, "https://example.org/TradeGravity/")
	if err != nil {
		t.Fatal(err)
	}
	kitDir := filepath.Join(root, "kit")
	if err := distributionkit.Write(kitDir, bundle); err != nil {
		t.Fatal(err)
	}
	_, approvalRaw, err := distributionkit.Approve(kitDir, distributionkit.ApprovalRequest{
		Reviewer: "reviewer", Audience: "pilot-audience", Channels: []string{"email"},
		ApprovedAt: time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC), Attested: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := distributionkit.WriteApproval(kitDir, approvalRaw); err != nil {
		t.Fatal(err)
	}
	subscriberPath := filepath.Join(root, "private", "subscribers.csv")
	suppressionPath := filepath.Join(root, "private", "suppressions.csv")
	if err := os.MkdirAll(filepath.Dir(subscriberPath), 0o700); err != nil {
		t.Fatal(err)
	}
	subscribers := strings.Join([]string{
		"email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,unsubscribe_url",
		"alpha@example.invalid,pilot-audience,active,2026-07-10T01:00:00Z,double_opt_in,test-form,v1,https://subscriptions.example.invalid/u?token=opaque-alpha",
		"beta@example.invalid,pilot-audience,active,2026-07-11T01:00:00Z,double_opt_in,test-form,v1,https://subscriptions.example.invalid/u?token=opaque-beta",
		"",
	}, "\n")
	if err := os.WriteFile(subscriberPath, []byte(subscribers), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(suppressionPath, []byte("email,reason,suppressed_at\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	preflightAt := time.Date(2026, 7, 17, 12, 30, 0, 0, time.UTC)
	preflight, err := deliverypreflight.Build(deliverypreflight.Request{
		KitDir: kitDir, SubscriberCSV: subscriberPath, SuppressionCSV: suppressionPath,
		GeneratedAt: preflightAt, MaxRecipients: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	preflightPath := filepath.Join(root, "delivery-preflight.json")
	if err := deliverypreflight.Write(preflightPath, kitDir, preflight.JSON); err != nil {
		t.Fatal(err)
	}
	return deliveryFixture{
		root: root, kitDir: kitDir, subscriberPath: subscriberPath, suppressionPath: suppressionPath,
		preflightPath: preflightPath, ledgerPath: filepath.Join(root, "private", "delivery-ledger.db"),
		preflightAt: preflightAt, authorizedAt: preflightAt.Add(time.Minute), sendAt: preflightAt.Add(2 * time.Minute),
	}
}

func (fixture deliveryFixture) authorizationRequest() AuthorizationRequest {
	return AuthorizationRequest{
		PreflightPath: fixture.preflightPath, Provider: "resend", Sender: "TradeGravity <brief@example.org>",
		ReplyTo: "maintainer@example.org", AuthorizedBy: "maintainer", AuthorizedAt: fixture.authorizedAt,
		ExpiresAt: fixture.authorizedAt.Add(20 * time.Minute),
		Attestations: Attestations{
			SenderDomainAuthenticated: true, BounceComplaintReady: true,
			PrivacyControlsReviewed: true, PilotRecipientsConfirmed: true,
		},
	}
}

func (fixture deliveryFixture) writeAuthorization(t *testing.T) string {
	t.Helper()
	_, raw, err := Authorize(fixture.authorizationRequest())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(fixture.root, "launch.json")
	if err := WriteAuthorization(path, raw); err != nil {
		t.Fatal(err)
	}
	return path
}

func (fixture deliveryFixture) writeAuthorizationAt(t *testing.T, name string, authorizedAt, expiresAt time.Time) string {
	t.Helper()
	request := fixture.authorizationRequest()
	request.AuthorizedAt = authorizedAt
	request.ExpiresAt = expiresAt
	_, raw, err := Authorize(request)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(fixture.root, name)
	if err := WriteAuthorization(path, raw); err != nil {
		t.Fatal(err)
	}
	return path
}

func (fixture deliveryFixture) openLedger(t *testing.T) *Ledger {
	t.Helper()
	ledger, err := OpenLedger(fixture.ledgerPath, bytes.Repeat([]byte("l"), 32))
	if err != nil {
		t.Fatal(err)
	}
	return ledger
}

func (fixture deliveryFixture) editionID(t *testing.T) string {
	t.Helper()
	plan, _, err := ReadPreflight(fixture.preflightPath)
	if err != nil {
		t.Fatal(err)
	}
	return plan.EditionID
}

func (fixture deliveryFixture) deliveryRequest(authorizationPath string, provider Provider) DeliveryRequest {
	return DeliveryRequest{
		KitDir: fixture.kitDir, SubscriberCSV: fixture.subscriberPath, SuppressionCSV: fixture.suppressionPath,
		PreflightPath: fixture.preflightPath, AuthorizationPath: authorizationPath, LedgerPath: fixture.ledgerPath,
		LedgerSecret: bytes.Repeat([]byte("l"), 32), SendAt: fixture.sendAt, Provider: provider, SendLive: true,
	}
}

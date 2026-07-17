package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	svix "github.com/svix/svix-webhooks/go"
)

const testWebhookSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"

func TestSignedResendFeedbackSuppressesOnceAndBlocksLaterImport(t *testing.T) {
	_, registry := newTestRegistry(t)
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := registry.ImportConsents(context.Background(), testConsentCSV(), now); err != nil {
		t.Fatal(err)
	}
	handler, err := registry.HandlerWithResendWebhook(testWebhookSecret, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	bounced := webhookPayload(t, "email.bounced", now, "alpha@example.invalid")
	response := sendSignedWebhook(t, handler, "msg_bounced_alpha", now, bounced)
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("signed bounce response = %d %q", response.Code, response.Body.String())
	}
	active, suppressed, err := registry.ExportAudience(context.Background(), "pilot-audience")
	if err != nil {
		t.Fatal(err)
	}
	if len(readCSV(t, active)) != 2 || len(readCSV(t, suppressed)) != 2 {
		t.Fatalf("bounce was not applied: active=%d suppressed=%d", len(readCSV(t, active)), len(readCSV(t, suppressed)))
	}
	if row := readCSV(t, suppressed)[1]; row[0] != "alpha@example.invalid" || row[1] != "bounced" || row[2] != now.Format(time.RFC3339) {
		t.Fatalf("unexpected bounce suppression: %v", row)
	}

	repeat := sendSignedWebhook(t, handler, "msg_bounced_alpha", now, bounced)
	if repeat.Code != http.StatusNoContent {
		t.Fatalf("duplicate webhook response = %d", repeat.Code)
	}
	_, repeatedSuppressions, err := registry.ExportAudience(context.Background(), "pilot-audience")
	if err != nil || len(readCSV(t, repeatedSuppressions)) != 2 {
		t.Fatalf("duplicate webhook changed suppression rows: err=%v rows=%d", err, len(readCSV(t, repeatedSuppressions)))
	}

	invalid := httptest.NewRequest(http.MethodPost, "/service/webhooks/resend", nil)
	invalid.Header.Set("Content-Type", "application/json")
	invalid.Header.Set("svix-id", "msg_invalid")
	invalid.Header.Set("svix-timestamp", strconv.FormatInt(now.Unix(), 10))
	invalid.Header.Set("svix-signature", "v1,invalid")
	invalidResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid signature response = %d", invalidResponse.Code)
	}

	preexisting := webhookPayload(t, "email.suppressed", now, "gamma@example.invalid")
	preexistingResponse := sendSignedWebhook(t, handler, "msg_suppressed_gamma", now, preexisting)
	if preexistingResponse.Code != http.StatusNoContent {
		t.Fatalf("preexisting suppression response = %d", preexistingResponse.Code)
	}
	gammaConsent := []byte("email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version\n" +
		"gamma@example.invalid,pilot-audience,active," + now.Add(-time.Hour).Format(time.RFC3339) + ",double_opt_in,test-form,v1\n")
	result, err := registry.ImportConsents(context.Background(), gammaConsent, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if result.Inserted != 0 || result.Updated != 0 || result.SuppressedSkipped != 1 {
		t.Fatalf("provider-suppressed address was imported: %+v", result)
	}
	_, allSuppressions, err := registry.ExportAudience(context.Background(), "pilot-audience")
	if err != nil {
		t.Fatal(err)
	}
	rows := readCSV(t, allSuppressions)
	if len(rows) != 3 || rows[2][0] != "gamma@example.invalid" || rows[2][1] != "invalid" {
		t.Fatalf("global suppression was not included in the private export: %v", rows)
	}
}

func TestResendWebhookIgnoresSignedNonSuppressionEvents(t *testing.T) {
	_, registry := newTestRegistry(t)
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := registry.ImportConsents(context.Background(), testConsentCSV(), now); err != nil {
		t.Fatal(err)
	}
	handler, err := registry.HandlerWithResendWebhook(testWebhookSecret, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	payload := webhookPayload(t, "email.delivered", now, "alpha@example.invalid")
	response := sendSignedWebhook(t, handler, "msg_delivered_alpha", now, payload)
	if response.Code != http.StatusNoContent {
		t.Fatalf("non-suppression event response = %d", response.Code)
	}
	active, suppressed, err := registry.ExportAudience(context.Background(), "pilot-audience")
	if err != nil || len(readCSV(t, active)) != 3 || len(readCSV(t, suppressed)) != 1 {
		t.Fatalf("non-suppression event changed state: err=%v", err)
	}
}

func webhookPayload(t *testing.T, eventType string, occurredAt time.Time, email string) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"type":       eventType,
		"created_at": occurredAt.Format(time.RFC3339Nano),
		"data": map[string]any{
			"email_id": "provider-email-id",
			"to":       []string{email},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func sendSignedWebhook(t *testing.T, handler http.Handler, eventID string, timestamp time.Time, payload []byte) *httptest.ResponseRecorder {
	t.Helper()
	webhook, err := svix.NewWebhook(testWebhookSecret)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := webhook.Sign(eventID, timestamp, payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/service/webhooks/resend", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("svix-id", eventID)
	request.Header.Set("svix-timestamp", strconv.FormatInt(timestamp.Unix(), 10))
	request.Header.Set("svix-signature", signature)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

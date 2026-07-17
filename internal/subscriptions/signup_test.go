package subscriptions

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type recordingConfirmationSender struct {
	messages []ConfirmationEmail
	failures int
}

func (sender *recordingConfirmationSender) SendConfirmation(_ context.Context, message ConfirmationEmail) (string, error) {
	sender.messages = append(sender.messages, message)
	if sender.failures > 0 {
		sender.failures--
		return "", errors.New("simulated provider uncertainty")
	}
	return "msg_test_123", nil
}

func testSignupConfig() SignupConfig {
	return SignupConfig{
		Audience: "tradegravity-briefing", ConsentSource: "public-form", PrivacyNoticeVersion: "v1",
		PrivacyNoticeURL: "https://subscriptions.example.invalid/privacy", ConfirmationTTL: 30 * time.Minute,
		DispatchCooldown: time.Minute, MaxPending: 100,
	}
}

func postForm(handler http.Handler, path string, values url.Values) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestDoubleOptInRequiresExplicitConfirmationPOST(t *testing.T) {
	_, registry := newTestRegistry(t)
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	sender := &recordingConfirmationSender{}
	handler, err := registry.HandlerWithOptions(HandlerOptions{Now: func() time.Time { return now }, Signup: &SignupOptions{Config: testSignupConfig(), Sender: sender}})
	if err != nil {
		t.Fatal(err)
	}
	response := postForm(handler, "/service/subscribe", url.Values{"email": {"reader@example.invalid"}, "privacy": {"accepted"}})
	if response.Code != http.StatusAccepted {
		t.Fatalf("signup status=%d body=%s", response.Code, response.Body.String())
	}
	if len(sender.messages) != 1 {
		t.Fatalf("confirmation sends=%d", len(sender.messages))
	}
	if strings.Contains(response.Body.String(), "reader@example.invalid") || strings.Contains(response.Body.String(), "token=") {
		t.Fatal("public response leaked subscriber identity or token")
	}
	confirmationURL, err := url.Parse(sender.messages[0].ConfirmationURL)
	if err != nil {
		t.Fatal(err)
	}
	if confirmationURL.Path != "/service/confirm" {
		t.Fatalf("confirmation path=%s", confirmationURL.Path)
	}
	parts := strings.Split(confirmationURL.Query().Get("token"), ".")
	if len(parts) != 2 {
		t.Fatal("invalid confirmation token shape")
	}
	payload, err := url.QueryUnescape(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, "reader") || strings.Contains(payload, "tradegravity") {
		t.Fatal("confirmation token leaked identity")
	}

	get := httptest.NewRequest(http.MethodGet, confirmationURL.RequestURI(), nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, get)
	if getResponse.Code != http.StatusOK || !strings.Contains(getResponse.Body.String(), "has not activated") {
		t.Fatalf("confirmation GET=%d %s", getResponse.Code, getResponse.Body.String())
	}
	active, _, err := registry.ExportAudience(context.Background(), testSignupConfig().Audience)
	if err != nil {
		t.Fatal(err)
	}
	if len(readCSV(t, active)) != 1 {
		t.Fatal("confirmation GET activated the subscription")
	}

	confirm := postForm(handler, confirmationURL.RequestURI(), url.Values{"confirm": {"yes"}})
	if confirm.Code != http.StatusOK || strings.Contains(confirm.Body.String(), "reader@example.invalid") {
		t.Fatalf("confirmation POST=%d %s", confirm.Code, confirm.Body.String())
	}
	active, _, err = registry.ExportAudience(context.Background(), testSignupConfig().Audience)
	if err != nil {
		t.Fatal(err)
	}
	rows := readCSV(t, active)
	if len(rows) != 2 || rows[1][0] != "reader@example.invalid" || rows[1][4] != "double_opt_in" {
		t.Fatalf("active export=%v", rows)
	}
	repeat := postForm(handler, confirmationURL.RequestURI(), url.Values{"confirm": {"yes"}})
	if repeat.Code != http.StatusOK {
		t.Fatalf("repeat confirmation=%d", repeat.Code)
	}
	secondSignup := postForm(handler, "/service/subscribe", url.Values{"email": {"reader@example.invalid"}, "privacy": {"accepted"}})
	if secondSignup.Code != http.StatusAccepted || len(sender.messages) != 1 {
		t.Fatal("active address triggered another confirmation")
	}
}

func TestConfirmationRetryUsesStableIdempotencyKey(t *testing.T) {
	_, registry := newTestRegistry(t)
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	sender := &recordingConfirmationSender{failures: 1}
	handler, err := registry.HandlerWithOptions(HandlerOptions{Now: func() time.Time { return now }, Signup: &SignupOptions{Config: testSignupConfig(), Sender: sender}})
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{"email": {"retry@example.invalid"}, "privacy": {"accepted"}}
	if got := postForm(handler, "/service/subscribe", values).Code; got != http.StatusServiceUnavailable {
		t.Fatalf("first status=%d", got)
	}
	if got := postForm(handler, "/service/subscribe", values).Code; got != http.StatusAccepted {
		t.Fatalf("cooldown status=%d", got)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("cooldown triggered send: %d", len(sender.messages))
	}
	now = now.Add(61 * time.Second)
	if got := postForm(handler, "/service/subscribe", values).Code; got != http.StatusAccepted {
		t.Fatalf("retry status=%d", got)
	}
	if len(sender.messages) != 2 || sender.messages[0].IdempotencyKey != sender.messages[1].IdempotencyKey || sender.messages[0].ConfirmationURL != sender.messages[1].ConfirmationURL {
		t.Fatalf("retry was not stable: %+v", sender.messages)
	}
}

func TestGlobalProviderSuppressionBlocksConfirmation(t *testing.T) {
	_, registry := newTestRegistry(t)
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	_, err := registry.SuppressAddress(context.Background(), "blocked@example.invalid", "bounced", "evt-1", "email.bounced", now.Add(-time.Minute), now)
	if err != nil {
		t.Fatal(err)
	}
	sender := &recordingConfirmationSender{}
	handler, err := registry.HandlerWithOptions(HandlerOptions{Now: func() time.Time { return now }, Signup: &SignupOptions{Config: testSignupConfig(), Sender: sender}})
	if err != nil {
		t.Fatal(err)
	}
	response := postForm(handler, "/service/subscribe", url.Values{"email": {"blocked@example.invalid"}, "privacy": {"accepted"}})
	if response.Code != http.StatusAccepted || len(sender.messages) != 0 {
		t.Fatalf("suppressed signup status=%d sends=%d", response.Code, len(sender.messages))
	}
}

func TestConfirmationExpiresAndPurposeIsSeparated(t *testing.T) {
	_, registry := newTestRegistry(t)
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	dispatch, err := registry.RequestSubscription(context.Background(), "expiry@example.invalid", testSignupConfig(), now)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(dispatch.ConfirmationURL, strings.Split(dispatch.ConfirmationURL, "?")[0]+"?token=")
	decoded, _ := url.QueryUnescape(token)
	if registry.ValidateToken(decoded) == nil {
		t.Fatal("confirmation token was accepted as unsubscribe token")
	}
	forged := decoded[:len(decoded)-1] + "x"
	if err := registry.ValidateConfirmation(forged, now); !errors.Is(err, ErrInvalidConfirmation) {
		t.Fatalf("forged confirmation validation=%v", err)
	}
	if err := registry.ValidateConfirmation(decoded, now.Add(31*time.Minute)); !errors.Is(err, ErrInvalidConfirmation) {
		t.Fatalf("expired validation=%v", err)
	}
	if _, err := registry.ConfirmSubscription(context.Background(), decoded, now.Add(31*time.Minute)); !errors.Is(err, ErrInvalidConfirmation) {
		t.Fatalf("expired confirmation=%v", err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.Split(decoded, ".")[0])
	if err != nil || strings.Contains(string(payload), "expiry@example.invalid") || strings.Contains(string(payload), testSignupConfig().Audience) {
		t.Fatalf("confirmation token payload leaked identity: %q err=%v", payload, err)
	}
}

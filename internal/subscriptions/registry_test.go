package subscriptions

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"tradegravity/internal/deliverypreflight"
	"tradegravity/internal/distributionkit"
)

func TestRegistryExportsOpaqueLinksAndGETDoesNotUnsubscribe(t *testing.T) {
	root, registry := newTestRegistry(t)
	importedAt := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	result, err := registry.ImportConsents(context.Background(), testConsentCSV(), importedAt)
	if err != nil {
		t.Fatalf("ImportConsents() error = %v", err)
	}
	if result.Inserted != 2 || result.Updated != 0 || result.SuppressedSkipped != 0 {
		t.Fatalf("unexpected import result: %+v", result)
	}

	subscribers, suppressions, err := registry.ExportAudience(context.Background(), "pilot-audience")
	if err != nil {
		t.Fatal(err)
	}
	subscriberRows := readCSV(t, subscribers)
	if len(subscriberRows) != 3 || len(readCSV(t, suppressions)) != 1 {
		t.Fatalf("unexpected export rows: subscribers=%d suppressions=%d", len(subscriberRows), len(readCSV(t, suppressions)))
	}
	if !equalStrings(subscriberRows[0], deliveryHeader) || subscriberRows[1][0] != "alpha@example.invalid" {
		t.Fatalf("unexpected subscriber export: %v", subscriberRows)
	}
	unsubscribeURL, err := url.Parse(subscriberRows[1][7])
	if err != nil || unsubscribeURL.Scheme != "https" || unsubscribeURL.Path != "/service/unsubscribe" {
		t.Fatalf("unexpected unsubscribe URL: %q err=%v", subscriberRows[1][7], err)
	}
	token := unsubscribeURL.Query().Get("token")
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		t.Fatalf("unexpected token shape: %q", token)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || bytes.Contains(payload, []byte("alpha@example.invalid")) || bytes.Contains(payload, []byte("pilot-audience")) {
		t.Fatalf("token payload leaked subscriber identity: %s", payload)
	}
	if err := registry.ValidateToken(token); err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	mutated := token[:len(token)-1] + "x"
	if err := registry.ValidateToken(mutated); err == nil {
		t.Fatal("ValidateToken() accepted a modified signature")
	}

	fixedNow := time.Date(2026, 7, 17, 12, 30, 0, 0, time.UTC)
	handler := registry.Handler(func() time.Time { return fixedNow })
	get := httptest.NewRequest(http.MethodGet, unsubscribeURL.RequestURI(), nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, get)
	if getResponse.Code != http.StatusOK || !strings.Contains(getResponse.Body.String(), "has not changed") || !strings.Contains(getResponse.Body.String(), `action="/service/unsubscribe?token=`) || strings.Contains(getResponse.Body.String(), "alpha@example.invalid") {
		t.Fatalf("unsafe GET response: code=%d body=%s", getResponse.Code, getResponse.Body.String())
	}
	for _, header := range []string{"Cache-Control", "Content-Security-Policy", "Referrer-Policy", "X-Content-Type-Options", "X-Frame-Options"} {
		if getResponse.Header().Get(header) == "" {
			t.Fatalf("GET response omitted security header %s", header)
		}
	}
	activeAfterGET, suppressedAfterGET, err := registry.ExportAudience(context.Background(), "pilot-audience")
	if err != nil || len(readCSV(t, activeAfterGET)) != 3 || len(readCSV(t, suppressedAfterGET)) != 1 {
		t.Fatalf("GET changed subscription state: err=%v", err)
	}

	badPost := httptest.NewRequest(http.MethodPost, unsubscribeURL.RequestURI(), strings.NewReader("List-Unsubscribe=One-Click"))
	badPostResponse := httptest.NewRecorder()
	handler.ServeHTTP(badPostResponse, badPost)
	if badPostResponse.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("POST without form content type = %d", badPostResponse.Code)
	}

	post := httptest.NewRequest(http.MethodPost, unsubscribeURL.RequestURI(), strings.NewReader("List-Unsubscribe=One-Click"))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postResponse := httptest.NewRecorder()
	handler.ServeHTTP(postResponse, post)
	if postResponse.Code != http.StatusOK || !strings.Contains(postResponse.Body.String(), "now suppressed") || strings.Contains(postResponse.Body.String(), "alpha@example.invalid") {
		t.Fatalf("unexpected POST response: code=%d body=%s", postResponse.Code, postResponse.Body.String())
	}

	repeat := httptest.NewRequest(http.MethodPost, unsubscribeURL.RequestURI(), strings.NewReader("List-Unsubscribe=One-Click"))
	repeat.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	repeatResponse := httptest.NewRecorder()
	handler.ServeHTTP(repeatResponse, repeat)
	if repeatResponse.Code != http.StatusOK {
		t.Fatalf("idempotent POST response = %d", repeatResponse.Code)
	}

	active, suppressed, err := registry.ExportAudience(context.Background(), "pilot-audience")
	if err != nil {
		t.Fatal(err)
	}
	if len(readCSV(t, active)) != 2 || len(readCSV(t, suppressed)) != 2 {
		t.Fatalf("unsubscribe export counts are wrong: active=%d suppressed=%d", len(readCSV(t, active)), len(readCSV(t, suppressed)))
	}
	suppressionRows := readCSV(t, suppressed)
	if suppressionRows[1][0] != "alpha@example.invalid" || suppressionRows[1][1] != "unsubscribed" || suppressionRows[1][2] != fixedNow.Format(time.RFC3339) {
		t.Fatalf("unexpected suppression row: %v", suppressionRows[1])
	}

	reimported, err := registry.ImportConsents(context.Background(), testConsentCSV(), importedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if reimported.Updated != 1 || reimported.SuppressedSkipped != 1 || reimported.Inserted != 0 {
		t.Fatalf("suppression was not durable across import: %+v", reimported)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(root, "subscriptions.db"))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("database mode = %v err=%v", info.Mode().Perm(), err)
		}
	}
}

func TestRegistryExportsFeedEmailPreflightWithoutLeakingTokens(t *testing.T) {
	root, registry := newTestRegistry(t)
	if _, err := registry.ImportConsents(context.Background(), testConsentCSV(), time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	subscribers, suppressions, err := registry.ExportAudience(context.Background(), "pilot-audience")
	if err != nil {
		t.Fatal(err)
	}
	subscriberPath := filepath.Join(root, "subscribers.csv")
	suppressionPath := filepath.Join(root, "suppressions.csv")
	if err := os.WriteFile(subscriberPath, subscribers, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(suppressionPath, suppressions, 0o600); err != nil {
		t.Fatal(err)
	}

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
		ApprovedAt: time.Date(2026, 7, 17, 12, 10, 0, 0, time.UTC), Attested: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := distributionkit.WriteApproval(kitDir, approvalRaw); err != nil {
		t.Fatal(err)
	}
	preflight, err := deliverypreflight.Build(deliverypreflight.Request{
		KitDir: kitDir, SubscriberCSV: subscriberPath, SuppressionCSV: suppressionPath,
		GeneratedAt: time.Date(2026, 7, 17, 12, 30, 0, 0, time.UTC), MaxRecipients: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preflight.Plan.Counts.Eligible != 2 || !preflight.Plan.UnsubscribeURLsValidated || preflight.Plan.DeliveryAuthorized {
		t.Fatalf("unexpected registry-to-preflight result: %+v", preflight.Plan)
	}
	if bytes.Contains(preflight.JSON, []byte("example.invalid")) || bytes.Contains(preflight.JSON, []byte("token=")) {
		t.Fatal("aggregate preflight leaked an address or unsubscribe token")
	}
}

func TestOpenRejectsShortSecretAndInsecureURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subscriptions.db")
	if _, err := Open(path, []byte("short"), "https://subscriptions.example.invalid/"); err == nil {
		t.Fatal("Open() accepted a short secret")
	}
	if _, err := Open(path, bytes.Repeat([]byte("s"), 32), "http://subscriptions.example.invalid/"); err == nil {
		t.Fatal("Open() accepted an insecure public URL")
	}
}

func TestWritePrivateExportsRefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	subscriberPath := filepath.Join(root, "private", "subscribers.csv")
	suppressionPath := filepath.Join(root, "private", "suppressions.csv")
	if err := WritePrivateExports(subscriberPath, []byte("subscribers\n"), suppressionPath, []byte("suppressions\n")); err != nil {
		t.Fatalf("WritePrivateExports() error = %v", err)
	}
	if err := WritePrivateExports(subscriberPath, []byte("changed\n"), suppressionPath, []byte("changed\n")); err == nil {
		t.Fatal("WritePrivateExports() overwrote existing private files")
	}
	stored, err := os.ReadFile(subscriberPath)
	if err != nil || string(stored) != "subscribers\n" {
		t.Fatalf("subscriber export changed: %q err=%v", stored, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(suppressionPath)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("private export mode = %v", info.Mode().Perm())
		}
	}
}

func newTestRegistry(t *testing.T) (string, *Registry) {
	t.Helper()
	root := t.TempDir()
	registry, err := Open(filepath.Join(root, "subscriptions.db"), bytes.Repeat([]byte("s"), 32), "https://subscriptions.example.invalid/service/")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.Close() })
	return root, registry
}

func testConsentCSV() []byte {
	return []byte(strings.Join([]string{
		"email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version",
		"alpha@example.invalid,pilot-audience,active,2026-07-10T01:00:00Z,double_opt_in,test-form,v1",
		"beta@example.invalid,pilot-audience,active,2026-07-11T01:00:00Z,double_opt_in,test-form,v1",
		"",
	}, "\n"))
}

func readCSV(t *testing.T, raw []byte) [][]string {
	t.Helper()
	rows, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

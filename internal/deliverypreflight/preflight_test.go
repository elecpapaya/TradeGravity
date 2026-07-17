package deliverypreflight

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tradegravity/internal/distributionkit"
)

func TestBuildProducesAggregateOnlyConsentPreflight(t *testing.T) {
	root, kitDir := buildApprovedKit(t)
	subscribers := writeFixture(t, root, "subscribers.csv", strings.Join([]string{
		"email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,unsubscribe_url",
		"alpha@example.invalid,pilot-audience,active,2026-07-10T01:00:00Z,double_opt_in,website-form,v1,https://subscriptions.example.invalid/u/opaque-alpha-token",
		"beta@example.invalid,pilot-audience,active,2026-07-11T02:00:00Z,double_opt_in,website-form,v1,https://subscriptions.example.invalid/u/opaque-beta-token",
		"",
	}, "\n"))
	suppressions := writeFixture(t, root, "suppressions.csv", strings.Join([]string{
		"email,reason,suppressed_at",
		"beta@example.invalid,unsubscribed,2026-07-12T03:00:00Z",
		"old@example.invalid,bounced,2026-07-01T00:00:00Z",
		"",
	}, "\n"))

	result, err := Build(Request{
		KitDir:         kitDir,
		SubscriberCSV:  subscribers,
		SuppressionCSV: suppressions,
		GeneratedAt:    time.Date(2026, 7, 17, 12, 0, 0, 0, time.FixedZone("KST", 9*60*60)),
		MaxRecipients:  25,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if result.Plan.Status != "consent_preflight_passed" || result.Plan.Channel != "email" || result.Plan.Audience != "pilot-audience" {
		t.Fatalf("unexpected preflight identity: %+v", result.Plan)
	}
	if result.Plan.Counts.Consented != 2 || result.Plan.Counts.Suppressed != 1 || result.Plan.Counts.SuppressionRows != 2 || result.Plan.Counts.Eligible != 1 {
		t.Fatalf("unexpected preflight counts: %+v", result.Plan.Counts)
	}
	if !result.Plan.ConsentValidated || !result.Plan.SuppressionApplied || !result.Plan.UnsubscribeURLsValidated || result.Plan.ContainsRecipientAddresses || result.Plan.ProviderConfigured || result.Plan.DeliveryAuthorized {
		t.Fatalf("preflight safety state is wrong: %+v", result.Plan)
	}
	if strings.Join(result.Plan.RequiredProviderHeaders, ",") != "List-Unsubscribe,List-Unsubscribe-Post" || result.Plan.ListUnsubscribePostValue != "List-Unsubscribe=One-Click" || !result.Plan.UnsubscribeHTTPSRequired || strings.Join(result.Plan.RequiredDKIMCoveredHeaders, ",") != "List-Unsubscribe,List-Unsubscribe-Post" || len(result.Plan.EmailTemplateSHA256) != 64 {
		t.Fatalf("unsubscribe contract is incomplete: %+v", result.Plan)
	}
	if len(result.EligibleRecipients) != 1 || result.EligibleRecipients[0].Email != "alpha@example.invalid" || result.EligibleRecipients[0].UnsubscribeURL != "https://subscriptions.example.invalid/u/opaque-alpha-token" {
		t.Fatalf("unexpected in-memory eligible recipients: %v", result.EligibleRecipients)
	}
	for _, forbidden := range []string{"alpha@example.invalid", "beta@example.invalid", "opaque-alpha-token", "opaque-beta-token", subscribers, suppressions} {
		if bytes.Contains(result.JSON, []byte(forbidden)) {
			t.Fatalf("aggregate plan leaked recipient PII or a local path: %s", forbidden)
		}
	}

	output := filepath.Join(root, "delivery-preflight.json")
	if err := Write(output, kitDir, result.JSON); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := Write(output, kitDir, result.JSON); err == nil {
		t.Fatal("Write() overwrote an existing preflight")
	}
	if err := Write(filepath.Join(kitDir, "delivery-preflight.json"), kitDir, result.JSON); err == nil {
		t.Fatal("Write() placed recipient-source metadata inside the approved kit")
	}
}

func TestBuildFailsClosedForConsentAudienceSuppressionAndLimit(t *testing.T) {
	root, kitDir := buildApprovedKit(t)
	emptySuppressions := writeFixture(t, root, "empty-suppressions.csv", "email,reason,suppressed_at\n")
	generatedAt := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		subscribers  string
		suppressions string
		max          int
		want         string
	}{
		{
			name:         "wrong audience",
			subscribers:  "email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,unsubscribe_url\nalpha@example.invalid,another-audience,active,2026-07-10T00:00:00Z,double_opt_in,form,v1,https://subscriptions.example.invalid/u/wrong-audience-token\n",
			suppressions: emptySuppressions,
			max:          25,
			want:         "audience does not match",
		},
		{
			name:         "single opt in",
			subscribers:  "email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,unsubscribe_url\nalpha@example.invalid,pilot-audience,active,2026-07-10T00:00:00Z,single_opt_in,form,v1,https://subscriptions.example.invalid/u/single-token\n",
			suppressions: emptySuppressions,
			max:          25,
			want:         "not active double opt-in",
		},
		{
			name:         "pilot limit",
			subscribers:  "email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,unsubscribe_url\nalpha@example.invalid,pilot-audience,active,2026-07-10T00:00:00Z,double_opt_in,form,v1,https://subscriptions.example.invalid/u/limit-alpha-token\nbeta@example.invalid,pilot-audience,active,2026-07-10T00:00:00Z,double_opt_in,form,v1,https://subscriptions.example.invalid/u/limit-beta-token\n",
			suppressions: emptySuppressions,
			max:          1,
			want:         "exceeds pilot limit",
		},
		{
			name:         "fully suppressed",
			subscribers:  "email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,unsubscribe_url\nalpha@example.invalid,pilot-audience,active,2026-07-10T00:00:00Z,double_opt_in,form,v1,https://subscriptions.example.invalid/u/suppressed-token\n",
			suppressions: "email,reason,suppressed_at\nalpha@example.invalid,complaint,2026-07-11T00:00:00Z\n",
			max:          25,
			want:         "no eligible recipients",
		},
		{
			name:         "insecure unsubscribe URL",
			subscribers:  "email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,unsubscribe_url\nalpha@example.invalid,pilot-audience,active,2026-07-10T00:00:00Z,double_opt_in,form,v1,http://subscriptions.example.invalid/u/insecure-token\n",
			suppressions: emptySuppressions,
			max:          25,
			want:         "must be absolute HTTPS",
		},
		{
			name:         "address in unsubscribe URL",
			subscribers:  "email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,unsubscribe_url\nalpha@example.invalid,pilot-audience,active,2026-07-10T00:00:00Z,double_opt_in,form,v1,https://subscriptions.example.invalid/u/alpha%40example.invalid\n",
			suppressions: emptySuppressions,
			max:          25,
			want:         "must not expose an email address",
		},
		{
			name:         "duplicate unsubscribe URL",
			subscribers:  "email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,unsubscribe_url\nalpha@example.invalid,pilot-audience,active,2026-07-10T00:00:00Z,double_opt_in,form,v1,https://subscriptions.example.invalid/u/shared-token\nbeta@example.invalid,pilot-audience,active,2026-07-10T00:00:00Z,double_opt_in,form,v1,https://subscriptions.example.invalid/u/shared-token\n",
			suppressions: emptySuppressions,
			max:          25,
			want:         "duplicates an unsubscribe URL",
		},
		{
			name:         "future consent",
			subscribers:  "email,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,unsubscribe_url\nalpha@example.invalid,pilot-audience,active,2026-07-18T00:00:00Z,double_opt_in,form,v1,https://subscriptions.example.invalid/u/future-token\n",
			suppressions: emptySuppressions,
			max:          25,
			want:         "future consent timestamp",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			subscriberPath := writeFixture(t, root, strings.ReplaceAll(test.name, " ", "-")+"-subscribers.csv", test.subscribers)
			suppressionPath := test.suppressions
			if !filepath.IsAbs(suppressionPath) {
				suppressionPath = writeFixture(t, root, strings.ReplaceAll(test.name, " ", "-")+"-suppressions.csv", test.suppressions)
			}
			_, err := Build(Request{KitDir: kitDir, SubscriberCSV: subscriberPath, SuppressionCSV: suppressionPath, GeneratedAt: generatedAt, MaxRecipients: test.max})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Build() error = %v, want %q", err, test.want)
			}
		})
	}

	insideKit := filepath.Join(kitDir, "email", "body.md")
	_, err := Build(Request{KitDir: kitDir, SubscriberCSV: insideKit, SuppressionCSV: emptySuppressions, GeneratedAt: generatedAt, MaxRecipients: 25})
	if err == nil || !strings.Contains(err.Error(), "outside the distribution kit") {
		t.Fatalf("Build() inside-kit source error = %v", err)
	}
}

func buildApprovedKit(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	briefingPath := filepath.Join("..", "..", "examples", "sample-data", "briefing.json")
	briefing, err := os.ReadFile(briefingPath)
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
		Reviewer:   "reviewer",
		Audience:   "pilot-audience",
		Channels:   []string{"email"},
		ApprovedAt: time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC),
		Attested:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := distributionkit.WriteApproval(kitDir, approvalRaw); err != nil {
		t.Fatal(err)
	}
	return root, kitDir
}

func writeFixture(t *testing.T, root, name, content string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

package comtrade

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestSafeTransportErrorRedactsRequestURL(t *testing.T) {
	const secret = "do-not-log-this-key"
	original := &url.Error{
		Op:  "Get",
		URL: "https://example.test/data?subscription-key=" + secret,
		Err: context.DeadlineExceeded,
	}
	got := safeTransportError("comtrade: request failed", original)
	if strings.Contains(got.Error(), secret) || strings.Contains(got.Error(), "subscription-key") || strings.Contains(got.Error(), "example.test") {
		t.Fatalf("transport error leaked request URL: %v", got)
	}
	if !strings.Contains(got.Error(), "deadline exceeded") {
		t.Fatalf("transport error lost safe cause: %v", got)
	}
}

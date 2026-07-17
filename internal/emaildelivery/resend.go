package emaildelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	resendEndpoint      = "https://api.resend.com/emails"
	maximumProviderBody = 64 << 10
)

type ResendProvider struct {
	apiKey   string
	client   *http.Client
	endpoint string
}

func NewResendProvider(apiKey string, client *http.Client) (*ResendProvider, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || strings.ContainsAny(apiKey, "\r\n") {
		return nil, errors.New("RESEND_API_KEY is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &ResendProvider{apiKey: apiKey, client: client, endpoint: resendEndpoint}, nil
}

func (provider *ResendProvider) Name() string { return "resend" }

func (provider *ResendProvider) Send(ctx context.Context, message Message, idempotencyKey string) (string, error) {
	return provider.sendTo(ctx, message, idempotencyKey, provider.endpoint)
}

func (provider *ResendProvider) sendTo(ctx context.Context, message Message, idempotencyKey, endpoint string) (string, error) {
	if strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 256 || strings.ContainsAny(idempotencyKey, "\r\n") {
		return "", errors.New("provider idempotency key is invalid")
	}
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.Scheme != "https" || parsedEndpoint.Host == "" || parsedEndpoint.User != nil || parsedEndpoint.RawQuery != "" || parsedEndpoint.Fragment != "" {
		return "", errors.New("provider endpoint must be absolute HTTPS")
	}
	payload := struct {
		From    string            `json:"from"`
		To      []string          `json:"to"`
		Subject string            `json:"subject"`
		HTML    string            `json:"html"`
		Text    string            `json:"text"`
		ReplyTo string            `json:"reply_to,omitempty"`
		Headers map[string]string `json:"headers,omitempty"`
	}{
		From: message.From, To: []string{message.To}, Subject: message.Subject,
		HTML: message.HTML, Text: message.Text, ReplyTo: message.ReplyTo,
	}
	if message.ListUnsubscribe != "" || message.ListUnsubscribePost != "" {
		if message.ListUnsubscribe == "" || message.ListUnsubscribePost == "" {
			return "", errors.New("unsubscribe headers must be supplied together")
		}
		payload.Headers = map[string]string{
			"List-Unsubscribe":      message.ListUnsubscribe,
			"List-Unsubscribe-Post": message.ListUnsubscribePost,
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode provider request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("build provider request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+provider.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", idempotencyKey)
	request.Header.Set("User-Agent", "TradeGravity-email-pilot/1.0")
	response, err := provider.client.Do(request)
	if err != nil {
		return "", errors.New("provider request failed without a confirmed response")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumProviderBody+1))
	if err != nil {
		return "", errors.New("provider response could not be read")
	}
	if len(body) > maximumProviderBody {
		return "", errors.New("provider response exceeded the safety limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("provider rejected the request with HTTP %d", response.StatusCode)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := decodeStrictJSON(body, &result); err != nil || strings.TrimSpace(result.ID) == "" || len(result.ID) > 256 || strings.ContainsAny(result.ID, "\r\n") {
		return "", errors.New("provider success response omitted a valid message ID")
	}
	return result.ID, nil
}

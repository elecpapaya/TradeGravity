package emaildelivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tradegravity/internal/deliverypreflight"
)

const unsubscribePlaceholder = "{{UNSUBSCRIBE_URL}}"

type Provider interface {
	Name() string
	Send(context.Context, Message, string) (string, error)
}

type Message struct {
	From                string
	ReplyTo             string
	To                  string
	Subject             string
	HTML                string
	Text                string
	ListUnsubscribe     string
	ListUnsubscribePost string
}

type DeliveryRequest struct {
	KitDir            string
	SubscriberCSV     string
	SuppressionCSV    string
	PreflightPath     string
	AuthorizationPath string
	LedgerPath        string
	LedgerSecret      []byte
	SendAt            time.Time
	Provider          Provider
	SendLive          bool
}

type DeliveryResult struct {
	EditionID string
	Audience  string
	Eligible  int
	Accepted  int
	Skipped   int
	Pending   int
}

func Deliver(ctx context.Context, request DeliveryRequest) (DeliveryResult, error) {
	if !request.SendLive {
		return DeliveryResult{}, errors.New("live delivery requires the explicit send-live acknowledgement")
	}
	if request.Provider == nil || request.Provider.Name() != "resend" {
		return DeliveryResult{}, errors.New("the authorized resend provider is required")
	}
	originalPlan, preflightRaw, err := ReadPreflight(request.PreflightPath)
	if err != nil {
		return DeliveryResult{}, err
	}
	authorization, err := LoadAuthorization(request.AuthorizationPath, preflightRaw, request.SendAt)
	if err != nil {
		return DeliveryResult{}, err
	}
	if authorization.Provider != request.Provider.Name() {
		return DeliveryResult{}, errors.New("configured provider does not match launch authorization")
	}
	live, err := deliverypreflight.Build(deliverypreflight.Request{
		KitDir:         request.KitDir,
		SubscriberCSV:  request.SubscriberCSV,
		SuppressionCSV: request.SuppressionCSV,
		GeneratedAt:    request.SendAt.UTC(),
		MaxRecipients:  authorization.MaxRecipients,
	})
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("rerun live delivery preflight: %w", err)
	}
	if err := VerifyLivePlan(authorization, originalPlan, live.Plan); err != nil {
		return DeliveryResult{}, err
	}
	template, err := loadEmailTemplate(request.KitDir)
	if err != nil {
		return DeliveryResult{}, err
	}
	ledger, err := OpenLedger(request.LedgerPath, request.LedgerSecret)
	if err != nil {
		return DeliveryResult{}, err
	}
	defer ledger.Close()

	result := DeliveryResult{
		EditionID: authorization.EditionID,
		Audience:  authorization.Audience,
		Eligible:  len(live.EligibleRecipients),
	}
	for _, recipient := range live.EligibleRecipients {
		message, digest, err := renderMessage(template, authorization, recipient)
		if err != nil {
			return result, err
		}
		prepared, err := ledger.Prepare(ctx, DeliveryAttempt{
			EditionID:           authorization.EditionID,
			Audience:            authorization.Audience,
			Email:               recipient.Email,
			Provider:            request.Provider.Name(),
			ContentSHA256:       digest,
			AuthorizationSHA256: authorization.FileSHA256,
			AttemptedAt:         request.SendAt.UTC(),
		})
		if err != nil {
			pending, accepted, countErr := ledger.Counts(ctx, authorization.EditionID, authorization.Audience)
			if countErr == nil {
				result.Pending = pending
				result.Accepted = accepted
			}
			return result, fmt.Errorf("prepare recipient delivery: %w", err)
		}
		if prepared.AlreadyAccepted {
			result.Skipped++
			continue
		}
		messageID, err := request.Provider.Send(ctx, message, prepared.IdempotencyKey)
		if err != nil {
			result.Pending++
			return result, fmt.Errorf("provider did not confirm delivery acceptance; pending ledger entry requires reconciliation: %w", err)
		}
		if err := ledger.MarkAccepted(ctx, prepared.DeliveryKey, messageID, request.SendAt.UTC()); err != nil {
			result.Pending++
			return result, fmt.Errorf("provider accepted delivery but local ledger update failed; reconcile before retry: %w", err)
		}
		result.Accepted++
	}
	pending, accepted, err := ledger.Counts(ctx, authorization.EditionID, authorization.Audience)
	if err != nil {
		return result, fmt.Errorf("count delivery ledger: %w", err)
	}
	result.Pending = pending
	result.Accepted = accepted
	return result, nil
}

type emailTemplate struct {
	Subject string
	HTML    string
	Text    string
}

func loadEmailTemplate(kitDir string) (emailTemplate, error) {
	root, err := filepath.Abs(strings.TrimSpace(kitDir))
	if err != nil || strings.TrimSpace(kitDir) == "" {
		return emailTemplate{}, errors.New("distribution-kit directory is required")
	}
	read := func(relative string) ([]byte, error) {
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > maximumPrivateFileSize {
			return nil, fmt.Errorf("approved email file %s is unavailable or unsafe", relative)
		}
		return os.ReadFile(path)
	}
	subjectRaw, err := read("email/subject.txt")
	if err != nil {
		return emailTemplate{}, err
	}
	htmlRaw, err := read("email/body.html")
	if err != nil {
		return emailTemplate{}, err
	}
	textRaw, err := read("email/body.md")
	if err != nil {
		return emailTemplate{}, err
	}
	subject := strings.TrimSpace(string(subjectRaw))
	if subject == "" || len([]rune(subject)) > 200 || strings.ContainsAny(subject, "\r\n") {
		return emailTemplate{}, errors.New("approved email subject is empty, too long, or multiline")
	}
	if bytes.Count(htmlRaw, []byte(unsubscribePlaceholder)) != 1 || bytes.Count(textRaw, []byte(unsubscribePlaceholder)) != 1 {
		return emailTemplate{}, errors.New("approved email bodies must each contain exactly one unsubscribe placeholder")
	}
	return emailTemplate{Subject: subject, HTML: string(htmlRaw), Text: string(textRaw)}, nil
}

func renderMessage(template emailTemplate, authorization Authorization, recipient deliverypreflight.Recipient) (Message, string, error) {
	if recipient.Email == "" || recipient.UnsubscribeURL == "" || strings.ContainsAny(recipient.UnsubscribeURL, "\r\n") {
		return Message{}, "", errors.New("eligible recipient is missing a safe address or unsubscribe URL")
	}
	htmlBody := strings.Replace(template.HTML, unsubscribePlaceholder, recipient.UnsubscribeURL, 1)
	textBody := strings.Replace(template.Text, unsubscribePlaceholder, recipient.UnsubscribeURL, 1)
	if strings.Contains(htmlBody, unsubscribePlaceholder) || strings.Contains(textBody, unsubscribePlaceholder) {
		return Message{}, "", errors.New("unsubscribe placeholder remained after recipient rendering")
	}
	message := Message{
		From:                authorization.Sender,
		ReplyTo:             authorization.ReplyTo,
		To:                  recipient.Email,
		Subject:             template.Subject,
		HTML:                htmlBody,
		Text:                textBody,
		ListUnsubscribe:     "<" + recipient.UnsubscribeURL + ">",
		ListUnsubscribePost: "List-Unsubscribe=One-Click",
	}
	digestSource := strings.Join([]string{
		message.From,
		message.ReplyTo,
		message.To,
		message.Subject,
		message.HTML,
		message.Text,
		message.ListUnsubscribe,
		message.ListUnsubscribePost,
	}, "\x00")
	digest := sha256.Sum256([]byte(digestSource))
	return message, hex.EncodeToString(digest[:]), nil
}

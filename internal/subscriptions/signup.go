package subscriptions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const confirmationTokenDomain = "tradegravity-confirmation-v1:"

var ErrInvalidConfirmation = errors.New("invalid or expired confirmation token")

type SignupConfig struct {
	Audience             string
	ConsentSource        string
	PrivacyNoticeVersion string
	PrivacyNoticeURL     string
	ConfirmationTTL      time.Duration
	DispatchCooldown     time.Duration
	MaxPending           int
}

type ConfirmationDispatch struct {
	PendingID       string
	Email           string
	ConfirmationURL string
	IdempotencyKey  string
	ExpiresAt       time.Time
	ShouldDispatch  bool
}

type ConfirmResult struct {
	Activated     bool
	AlreadyActive bool
}

func validateSignupConfig(config SignupConfig) error {
	if err := validateAudience(config.Audience); err != nil {
		return err
	}
	if err := validateLabel(config.ConsentSource); err != nil {
		return fmt.Errorf("consent source: %w", err)
	}
	if err := validateLabel(config.PrivacyNoticeVersion); err != nil {
		return fmt.Errorf("privacy notice version: %w", err)
	}
	privacy, err := url.Parse(strings.TrimSpace(config.PrivacyNoticeURL))
	if err != nil || privacy.Scheme != "https" || privacy.Host == "" || privacy.User != nil {
		return errors.New("privacy notice URL must be absolute HTTPS")
	}
	if config.ConfirmationTTL < 5*time.Minute || config.ConfirmationTTL > 24*time.Hour {
		return errors.New("confirmation TTL must be between 5 minutes and 24 hours")
	}
	if config.DispatchCooldown < 30*time.Second || config.DispatchCooldown > config.ConfirmationTTL {
		return errors.New("dispatch cooldown is invalid")
	}
	if config.MaxPending < 1 || config.MaxPending > 10000 {
		return errors.New("maximum pending subscriptions is invalid")
	}
	return nil
}

func (registry *Registry) RequestSubscription(ctx context.Context, email string, config SignupConfig, requestedAt time.Time) (ConfirmationDispatch, error) {
	if err := validateSignupConfig(config); err != nil {
		return ConfirmationDispatch{}, err
	}
	address, err := canonicalEmail(email)
	if err != nil || requestedAt.IsZero() {
		return ConfirmationDispatch{}, errors.New("subscription request is invalid")
	}
	now := requestedAt.UTC()
	tx, err := registry.db.BeginTx(ctx, nil)
	if err != nil {
		return ConfirmationDispatch{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM pending_subscriptions WHERE expires_at <= ?`, now.Format(time.RFC3339)); err != nil {
		return ConfirmationDispatch{}, fmt.Errorf("expire pending subscriptions: %w", err)
	}
	var marker int
	if err = tx.QueryRowContext(ctx, `SELECT 1 FROM address_suppressions WHERE email_normalized = ?`, address).Scan(&marker); err == nil {
		_ = tx.Commit()
		return ConfirmationDispatch{}, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ConfirmationDispatch{}, err
	}
	var subscriptionStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM subscriptions WHERE email_normalized = ? AND audience = ?`, address, config.Audience).Scan(&subscriptionStatus)
	if err == nil && subscriptionStatus == "active" {
		_ = tx.Commit()
		return ConfirmationDispatch{}, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ConfirmationDispatch{}, err
	}
	if err == nil && subscriptionStatus != "suppressed" {
		return ConfirmationDispatch{}, errors.New("stored subscription status is invalid")
	}
	if subscriptionStatus == "suppressed" {
		var reason string
		if err := tx.QueryRowContext(ctx, `SELECT suppression_reason FROM subscriptions WHERE email_normalized = ? AND audience = ?`, address, config.Audience).Scan(&reason); err != nil {
			return ConfirmationDispatch{}, err
		}
		if reason != "unsubscribed" {
			_ = tx.Commit()
			return ConfirmationDispatch{}, nil
		}
	}
	var id, status, expiresRaw string
	var lastDispatch sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id, status, expires_at, last_dispatch_at FROM pending_subscriptions WHERE email_normalized = ? AND audience = ?`, address, config.Audience).Scan(&id, &status, &expiresRaw, &lastDispatch)
	if err == nil {
		expires, parseErr := time.Parse(time.RFC3339, expiresRaw)
		if parseErr != nil {
			return ConfirmationDispatch{}, errors.New("stored confirmation expiry is invalid")
		}
		if status == "confirmed" || expires.Before(now) || expires.Equal(now) {
			if _, err := tx.ExecContext(ctx, `DELETE FROM pending_subscriptions WHERE id = ?`, id); err != nil {
				return ConfirmationDispatch{}, err
			}
		} else if status == "sent" {
			_ = tx.Commit()
			return ConfirmationDispatch{}, nil
		} else if status == "dispatch_pending" {
			if lastDispatch.Valid {
				last, parseErr := time.Parse(time.RFC3339, lastDispatch.String)
				if parseErr != nil {
					return ConfirmationDispatch{}, errors.New("stored dispatch time is invalid")
				}
				if now.Before(last.Add(config.DispatchCooldown)) {
					_ = tx.Commit()
					return ConfirmationDispatch{}, nil
				}
			}
			return registry.prepareDispatch(ctx, tx, id, address, expires, now)
		} else {
			return ConfirmationDispatch{}, errors.New("stored pending status is invalid")
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ConfirmationDispatch{}, err
	}
	var pendingCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pending_subscriptions WHERE status != 'confirmed' AND expires_at > ?`, now.Format(time.RFC3339)).Scan(&pendingCount); err != nil {
		return ConfirmationDispatch{}, err
	}
	if pendingCount >= config.MaxPending {
		_ = tx.Commit()
		return ConfirmationDispatch{}, nil
	}
	id, err = randomID()
	if err != nil {
		return ConfirmationDispatch{}, err
	}
	expires := now.Add(config.ConfirmationTTL)
	_, err = tx.ExecContext(ctx, `INSERT INTO pending_subscriptions (id,email,email_normalized,audience,status,consent_source,privacy_notice_version,requested_at,expires_at) VALUES (?,?,?,?,'dispatch_pending',?,?,?,?)`, id, address, address, config.Audience, config.ConsentSource, config.PrivacyNoticeVersion, now.Format(time.RFC3339), expires.Format(time.RFC3339))
	if err != nil {
		return ConfirmationDispatch{}, fmt.Errorf("create pending subscription: %w", err)
	}
	return registry.prepareDispatch(ctx, tx, id, address, expires, now)
}

func (registry *Registry) prepareDispatch(ctx context.Context, tx *sql.Tx, id, email string, expires, now time.Time) (ConfirmationDispatch, error) {
	if _, err := tx.ExecContext(ctx, `UPDATE pending_subscriptions SET last_dispatch_at = ? WHERE id = ? AND status = 'dispatch_pending'`, now.Format(time.RFC3339), id); err != nil {
		return ConfirmationDispatch{}, err
	}
	if err := tx.Commit(); err != nil {
		return ConfirmationDispatch{}, err
	}
	token, err := registry.confirmationTokenFor(id)
	if err != nil {
		return ConfirmationDispatch{}, err
	}
	confirmation := *registry.unsubscribeBase
	confirmation.Path = strings.TrimSuffix(confirmation.Path, "unsubscribe") + "confirm"
	query := confirmation.Query()
	query.Set("token", token)
	confirmation.RawQuery = query.Encode()
	return ConfirmationDispatch{PendingID: id, Email: email, ConfirmationURL: confirmation.String(), IdempotencyKey: "tradegravity-confirm/" + id, ExpiresAt: expires, ShouldDispatch: true}, nil
}

func (registry *Registry) MarkConfirmationDispatched(ctx context.Context, id, providerMessageID string, dispatchedAt time.Time) error {
	providerMessageID = strings.TrimSpace(providerMessageID)
	if len(id) != 32 || providerMessageID == "" || len(providerMessageID) > 256 || strings.ContainsAny(providerMessageID, "\r\n@:/") || dispatchedAt.IsZero() {
		return errors.New("confirmation dispatch evidence is invalid")
	}
	result, err := registry.db.ExecContext(ctx, `UPDATE pending_subscriptions SET status='sent', provider_message_id=? WHERE id=? AND status='dispatch_pending'`, providerMessageID, id)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 1 {
		return nil
	}
	var status, stored string
	if err := registry.db.QueryRowContext(ctx, `SELECT status, COALESCE(provider_message_id,'') FROM pending_subscriptions WHERE id=?`, id).Scan(&status, &stored); err != nil || status != "sent" || stored != providerMessageID {
		return errors.New("confirmation dispatch could not be recorded")
	}
	return nil
}

func (registry *Registry) ValidateConfirmation(token string, at time.Time) error {
	payload, err := registry.verifyPurposeToken(token, confirmationTokenDomain)
	if err != nil || at.IsZero() {
		return ErrInvalidConfirmation
	}
	var status, expiresRaw string
	if err := registry.db.QueryRow(`SELECT status, expires_at FROM pending_subscriptions WHERE id=?`, payload.ID).Scan(&status, &expiresRaw); err != nil {
		return ErrInvalidConfirmation
	}
	expires, err := time.Parse(time.RFC3339, expiresRaw)
	if err != nil || status == "confirmed" || !at.UTC().Before(expires) {
		return ErrInvalidConfirmation
	}
	return nil
}

func (registry *Registry) ConfirmSubscription(ctx context.Context, token string, at time.Time) (ConfirmResult, error) {
	payload, err := registry.verifyPurposeToken(token, confirmationTokenDomain)
	if err != nil || at.IsZero() {
		return ConfirmResult{}, ErrInvalidConfirmation
	}
	now := at.UTC()
	tx, err := registry.db.BeginTx(ctx, nil)
	if err != nil {
		return ConfirmResult{}, err
	}
	defer tx.Rollback()
	var email, audience, status, source, version, expiresRaw string
	err = tx.QueryRowContext(ctx, `SELECT email_normalized,audience,status,consent_source,privacy_notice_version,expires_at FROM pending_subscriptions WHERE id=?`, payload.ID).Scan(&email, &audience, &status, &source, &version, &expiresRaw)
	if err != nil {
		return ConfirmResult{}, ErrInvalidConfirmation
	}
	expires, err := time.Parse(time.RFC3339, expiresRaw)
	if err != nil || !now.Before(expires) {
		return ConfirmResult{}, ErrInvalidConfirmation
	}
	if status == "confirmed" {
		_ = tx.Commit()
		return ConfirmResult{AlreadyActive: true}, nil
	}
	var blocked int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM address_suppressions WHERE email_normalized=?`, email).Scan(&blocked); err == nil {
		return ConfirmResult{}, ErrInvalidConfirmation
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ConfirmResult{}, err
	}
	var subscriptionID, subscriptionStatus, reason string
	err = tx.QueryRowContext(ctx, `SELECT id,status,COALESCE(suppression_reason,'') FROM subscriptions WHERE email_normalized=? AND audience=?`, email, audience).Scan(&subscriptionID, &subscriptionStatus, &reason)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		subscriptionID, err = randomID()
		if err != nil {
			return ConfirmResult{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO subscriptions (id,email,email_normalized,audience,status,consented_at,consent_method,consent_source,privacy_notice_version,created_at) VALUES (?,?,?,?,'active',?,'double_opt_in',?,?,?)`, subscriptionID, email, email, audience, now.Format(time.RFC3339), source, version, now.Format(time.RFC3339))
	case err != nil:
		return ConfirmResult{}, err
	case subscriptionStatus == "active":
	case subscriptionStatus == "suppressed" && reason == "unsubscribed":
		_, err = tx.ExecContext(ctx, `UPDATE subscriptions SET status='active',consented_at=?,consent_source=?,privacy_notice_version=?,suppression_reason=NULL,suppressed_at=NULL,updated_at=? WHERE id=?`, now.Format(time.RFC3339), source, version, now.Format(time.RFC3339), subscriptionID)
	default:
		return ConfirmResult{}, ErrInvalidConfirmation
	}
	if err != nil {
		return ConfirmResult{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE pending_subscriptions SET status='confirmed',confirmed_at=? WHERE id=?`, now.Format(time.RFC3339), payload.ID)
	if err != nil {
		return ConfirmResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ConfirmResult{}, err
	}
	return ConfirmResult{Activated: true}, nil
}

func (registry *Registry) confirmationTokenFor(id string) (string, error) {
	return registry.purposeTokenFor(id, confirmationTokenDomain)
}

package subscriptions

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	svix "github.com/svix/svix-webhooks/go"
)

const maximumWebhookBody = 64 << 10

var unsubscribePage = template.Must(template.New("unsubscribe").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>TradeGravity email preferences</title></head>
<body><main><h1>{{.Title}}</h1><p>{{.Message}}</p>{{if .Token}}<form method="post" action="{{.Action}}?token={{.Token}}"><input type="hidden" name="List-Unsubscribe" value="One-Click"><button type="submit">Unsubscribe</button></form>{{end}}</main></body></html>`))

var signupPage = template.Must(template.New("signup").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>TradeGravity email briefing</title></head>
<body><main><h1>{{.Title}}</h1><p>{{.Message}}</p>{{if .Form}}<form method="post" action="{{.Action}}"><label>Email <input name="email" type="email" autocomplete="email" required maxlength="254"></label><input name="website" type="text" tabindex="-1" autocomplete="off" aria-hidden="true" hidden><label><input type="checkbox" name="privacy" value="accepted" required> I agree to the <a href="{{.PrivacyURL}}">privacy notice</a>.</label><button type="submit">Send confirmation email</button></form>{{end}}</main></body></html>`))

var confirmationPage = template.Must(template.New("confirmation").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>TradeGravity email confirmation</title></head>
<body><main><h1>{{.Title}}</h1><p>{{.Message}}</p>{{if .Token}}<form method="post" action="{{.Action}}?token={{.Token}}"><input type="hidden" name="confirm" value="yes"><button type="submit">Confirm subscription</button></form>{{end}}</main></body></html>`))

type unsubscribeView struct {
	Title   string
	Message string
	Token   string
	Action  string
}

type signupView struct {
	Title, Message, Action, PrivacyURL string
	Form                               bool
}

type ConfirmationEmail struct {
	To, ConfirmationURL, IdempotencyKey string
	ExpiresAt                           time.Time
}

type ConfirmationSender interface {
	SendConfirmation(context.Context, ConfirmationEmail) (string, error)
}

type SignupOptions struct {
	Config SignupConfig
	Sender ConfirmationSender
}

type HandlerOptions struct {
	Now                 func() time.Time
	ResendWebhookSecret string
	Signup              *SignupOptions
}

func (registry *Registry) Handler(now func() time.Time) http.Handler {
	handler, _ := registry.HandlerWithOptions(HandlerOptions{Now: now})
	return handler
}

func (registry *Registry) HandlerWithResendWebhook(webhookSecret string, now func() time.Time) (http.Handler, error) {
	return registry.HandlerWithOptions(HandlerOptions{Now: now, ResendWebhookSecret: webhookSecret})
}

func (registry *Registry) HandlerWithOptions(options HandlerOptions) (http.Handler, error) {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	var webhook *svix.Webhook
	if strings.TrimSpace(options.ResendWebhookSecret) != "" {
		var err error
		webhook, err = svix.NewWebhook(strings.TrimSpace(options.ResendWebhookSecret))
		if err != nil {
			return nil, errors.New("RESEND_WEBHOOK_SECRET is invalid")
		}
	}
	if options.Signup != nil {
		if options.Signup.Sender == nil {
			return nil, errors.New("confirmation sender is required")
		}
		if err := validateSignupConfig(options.Signup.Config); err != nil {
			return nil, err
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(response http.ResponseWriter, request *http.Request) {
		setSecurityHeaders(response)
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), time.Second)
		defer cancel()
		if err := registry.db.PingContext(ctx); err != nil {
			http.Error(response, "unavailable", http.StatusServiceUnavailable)
			return
		}
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = response.Write([]byte("ok\n"))
	})
	unsubscribePath := registry.unsubscribeBase.Path
	mux.HandleFunc(unsubscribePath, func(response http.ResponseWriter, request *http.Request) {
		setSecurityHeaders(response)
		token := request.URL.Query().Get("token")
		switch request.Method {
		case http.MethodGet:
			if err := registry.ValidateToken(token); err != nil {
				renderUnsubscribe(response, http.StatusBadRequest, unsubscribeView{Title: "Link unavailable", Message: "This unsubscribe link is invalid. No subscription was changed."})
				return
			}
			renderUnsubscribe(response, http.StatusOK, unsubscribeView{Title: "Unsubscribe from TradeGravity", Message: "Confirm that you no longer want this publication. Opening this page has not changed your subscription.", Token: token, Action: unsubscribePath})
		case http.MethodPost:
			request.Body = http.MaxBytesReader(response, request.Body, 1024)
			if !strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "application/x-www-form-urlencoded") {
				renderUnsubscribe(response, http.StatusUnsupportedMediaType, unsubscribeView{Title: "Request unavailable", Message: "The one-click unsubscribe request used an unsupported media type. No subscription was changed."})
				return
			}
			if err := request.ParseForm(); err != nil || request.Form.Get("List-Unsubscribe") != "One-Click" {
				renderUnsubscribe(response, http.StatusBadRequest, unsubscribeView{Title: "Request unavailable", Message: "The one-click unsubscribe request was invalid. No subscription was changed."})
				return
			}
			_, err := registry.Unsubscribe(request.Context(), token, now().UTC())
			if err != nil {
				status := http.StatusInternalServerError
				if errors.Is(err, ErrInvalidToken) {
					status = http.StatusBadRequest
				}
				renderUnsubscribe(response, status, unsubscribeView{Title: "Request unavailable", Message: "The unsubscribe request could not be completed. No additional information was disclosed."})
				return
			}
			renderUnsubscribe(response, http.StatusOK, unsubscribeView{Title: "Unsubscribed", Message: "This address is now suppressed from the selected TradeGravity publication."})
		default:
			response.Header().Set("Allow", strings.Join([]string{http.MethodGet, http.MethodPost}, ", "))
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	if options.Signup != nil {
		prefix := strings.TrimSuffix(unsubscribePath, "unsubscribe")
		subscribePath, confirmPath := prefix+"subscribe", prefix+"confirm"
		config, sender := options.Signup.Config, options.Signup.Sender
		mux.HandleFunc(subscribePath, func(response http.ResponseWriter, request *http.Request) {
			setSecurityHeaders(response)
			switch request.Method {
			case http.MethodGet:
				renderSignup(response, http.StatusOK, signupView{Title: "Subscribe to TradeGravity", Message: "Receive the research briefing after confirming your address.", Action: subscribePath, PrivacyURL: config.PrivacyNoticeURL, Form: true})
			case http.MethodPost:
				request.Body = http.MaxBytesReader(response, request.Body, 4096)
				if !strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "application/x-www-form-urlencoded") {
					renderSignup(response, http.StatusUnsupportedMediaType, signupView{Title: "Request unavailable", Message: "The subscription request was invalid."})
					return
				}
				if err := request.ParseForm(); err != nil || request.Form.Get("privacy") != "accepted" || request.Form.Get("website") != "" {
					renderSignup(response, http.StatusBadRequest, signupView{Title: "Request unavailable", Message: "The subscription request was invalid."})
					return
				}
				dispatch, err := registry.RequestSubscription(request.Context(), request.Form.Get("email"), config, now().UTC())
				if err != nil {
					renderSignup(response, http.StatusBadRequest, signupView{Title: "Request unavailable", Message: "The subscription request was invalid."})
					return
				}
				if dispatch.ShouldDispatch {
					messageID, sendErr := sender.SendConfirmation(request.Context(), ConfirmationEmail{To: dispatch.Email, ConfirmationURL: dispatch.ConfirmationURL, IdempotencyKey: dispatch.IdempotencyKey, ExpiresAt: dispatch.ExpiresAt})
					if sendErr != nil || registry.MarkConfirmationDispatched(request.Context(), dispatch.PendingID, messageID, now().UTC()) != nil {
						renderSignup(response, http.StatusServiceUnavailable, signupView{Title: "Temporarily unavailable", Message: "The confirmation request could not be completed. Please try again later."})
						return
					}
				}
				renderSignup(response, http.StatusAccepted, signupView{Title: "Check your inbox", Message: "If this address is eligible, a confirmation email has been sent. The subscription is not active until it is confirmed."})
			default:
				response.Header().Set("Allow", strings.Join([]string{http.MethodGet, http.MethodPost}, ", "))
				http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			}
		})
		mux.HandleFunc(confirmPath, func(response http.ResponseWriter, request *http.Request) {
			setSecurityHeaders(response)
			token := request.URL.Query().Get("token")
			switch request.Method {
			case http.MethodGet:
				if registry.ValidateConfirmation(token, now().UTC()) != nil {
					renderConfirmation(response, http.StatusBadRequest, "Link unavailable", "This confirmation link is invalid or expired.", "", confirmPath)
					return
				}
				renderConfirmation(response, http.StatusOK, "Confirm TradeGravity subscription", "Opening this page has not activated the subscription. Confirm below to finish.", token, confirmPath)
			case http.MethodPost:
				request.Body = http.MaxBytesReader(response, request.Body, 1024)
				if !strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "application/x-www-form-urlencoded") {
					renderConfirmation(response, http.StatusUnsupportedMediaType, "Request unavailable", "The confirmation request was invalid.", "", confirmPath)
					return
				}
				if err := request.ParseForm(); err != nil || request.Form.Get("confirm") != "yes" {
					renderConfirmation(response, http.StatusBadRequest, "Request unavailable", "The confirmation request was invalid.", "", confirmPath)
					return
				}
				if _, err := registry.ConfirmSubscription(request.Context(), token, now().UTC()); err != nil {
					renderConfirmation(response, http.StatusBadRequest, "Request unavailable", "The confirmation request could not be completed.", "", confirmPath)
					return
				}
				renderConfirmation(response, http.StatusOK, "Subscription confirmed", "The address is now subscribed to the selected TradeGravity briefing.", "", confirmPath)
			default:
				response.Header().Set("Allow", strings.Join([]string{http.MethodGet, http.MethodPost}, ", "))
				http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			}
		})
	}
	if webhook != nil {
		webhookPath := strings.TrimSuffix(unsubscribePath, "unsubscribe") + "webhooks/resend"
		mux.HandleFunc(webhookPath, func(response http.ResponseWriter, request *http.Request) {
			setSecurityHeaders(response)
			if request.Method != http.MethodPost {
				response.Header().Set("Allow", http.MethodPost)
				http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if !strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "application/json") {
				http.Error(response, "invalid webhook", http.StatusUnsupportedMediaType)
				return
			}
			request.Body = http.MaxBytesReader(response, request.Body, maximumWebhookBody)
			raw, err := io.ReadAll(request.Body)
			if err != nil || webhook.Verify(raw, request.Header) != nil {
				http.Error(response, "invalid webhook", http.StatusBadRequest)
				return
			}
			var event struct {
				Type      string `json:"type"`
				CreatedAt string `json:"created_at"`
				Data      struct {
					To []string `json:"to"`
				} `json:"data"`
			}
			if err := json.Unmarshal(raw, &event); err != nil {
				http.Error(response, "invalid webhook", http.StatusBadRequest)
				return
			}
			reason := ""
			switch event.Type {
			case "email.bounced":
				reason = "bounced"
			case "email.complained":
				reason = "complaint"
			case "email.suppressed":
				reason = "invalid"
			default:
				response.WriteHeader(http.StatusNoContent)
				return
			}
			occurredAt, err := time.Parse(time.RFC3339Nano, event.CreatedAt)
			if err != nil || len(event.Data.To) != 1 {
				http.Error(response, "invalid webhook", http.StatusBadRequest)
				return
			}
			if _, err := registry.SuppressAddress(request.Context(), event.Data.To[0], reason, request.Header.Get("svix-id"), event.Type, occurredAt, now().UTC()); err != nil {
				http.Error(response, "webhook unavailable", http.StatusInternalServerError)
				return
			}
			response.WriteHeader(http.StatusNoContent)
		})
	}
	return mux, nil
}

func renderSignup(response http.ResponseWriter, status int, view signupView) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(status)
	_ = signupPage.Execute(response, view)
}
func renderConfirmation(response http.ResponseWriter, status int, title, message, token, action string) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(status)
	_ = confirmationPage.Execute(response, unsubscribeView{Title: title, Message: message, Token: token, Action: action})
}

func renderUnsubscribe(response http.ResponseWriter, status int, view unsubscribeView) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(status)
	_ = unsubscribePage.Execute(response, view)
}

func setSecurityHeaders(response http.ResponseWriter) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Security-Policy", "default-src 'none'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("X-Frame-Options", "DENY")
}

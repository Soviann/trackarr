package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
)

// pushHTTPClient caps every outbound push request at 5 seconds. The default
// webpush-go client has no timeout; without this cap a stalled FCM/Mozilla
// endpoint would hang the caller (and, when called inside a DB transaction,
// block the sole write connection).
var pushHTTPClient = &http.Client{Timeout: 5 * time.Second}

// PushNotifier abstracts push notification operations.
// Use NewNoopNotifier() when VAPID keys are not configured.
type PushNotifier interface {
	Subscribe(ctx context.Context, rawJSON string) error
	Unsubscribe(ctx context.Context) error
	HasSubscription() bool
	SendNotification(ctx context.Context, title, body, url string) error
}

// PushService implements PushNotifier with real web push notifications.
type PushService struct {
	mu         sync.RWMutex
	writeDB    *sql.DB
	settings   *repository.SettingRepository
	publicKey  string
	privateKey string
	subject    string
}

func NewPushService(writeDB *sql.DB, settings *repository.SettingRepository, publicKey, privateKey, subject string) *PushService {
	return &PushService{
		writeDB:    writeDB,
		settings:   settings,
		publicKey:  publicKey,
		privateKey: privateKey,
		subject:    subject,
	}
}

// SetKeys updates the VAPID keys and subject thread-safely at runtime.
func (s *PushService) SetKeys(publicKey, privateKey, subject string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.publicKey = publicKey
	s.privateKey = privateKey
	s.subject = subject
}

// EnsureVAPIDKeys loads existing VAPID keys from the settings repository or environment.
// If none exist, it auto-generates a fresh NIST P-256 key pair and persists them to SQLite.
func EnsureVAPIDKeys(ctx context.Context, writeDB *sql.DB, settings *repository.SettingRepository, envPub, envPriv, envSub string) (pubKey, privKey, subject string, err error) {
	if settings != nil {
		pubKey, _ = settings.Get("vapid_public_key")
		privKey, _ = settings.Get("vapid_private_key")
		subject, _ = settings.Get("vapid_subject")
	}

	if pubKey == "" && envPub != "" {
		pubKey = envPub
	}
	if privKey == "" && envPriv != "" {
		privKey = envPriv
	}
	if subject == "" && envSub != "" {
		subject = envSub
	}

	if pubKey != "" && privKey != "" {
		if subject == "" {
			subject = "mailto:admin@localhost"
		}
		return pubKey, privKey, subject, nil
	}

	// Auto-generate fresh VAPID keys
	return GenerateAndSaveVAPIDKeys(ctx, writeDB, subject)
}

// GenerateAndSaveVAPIDKeys creates a fresh VAPID keypair and saves it to the SQLite settings table.
func GenerateAndSaveVAPIDKeys(ctx context.Context, writeDB *sql.DB, subject string) (pubKey, privKey, finalSub string, err error) {
	privKey, pubKey, err = webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", "", fmt.Errorf("generate vapid keys: %w", err)
	}

	finalSub = subject
	if finalSub == "" {
		finalSub = "mailto:admin@localhost"
	}

	if writeDB != nil {
		if err := database.WithTxContext(ctx, writeDB, func(tx *sql.Tx) error {
			w := repository.NewSettingWriter(tx)
			if err := w.Set(ctx, "vapid_public_key", pubKey); err != nil {
				return err
			}
			if err := w.Set(ctx, "vapid_private_key", privKey); err != nil {
				return err
			}
			return w.Set(ctx, "vapid_subject", finalSub)
		}); err != nil {
			return "", "", "", fmt.Errorf("persist vapid keys: %w", err)
		}
	}

	log.Printf("🔑 Auto-generated fresh VAPID push keys and saved to database")
	return pubKey, privKey, finalSub, nil
}

const settingKeyPushSubscription = "push_subscription"

type pushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (s *PushService) Subscribe(ctx context.Context, rawJSON string) error {
	var sub pushSubscription
	if err := json.Unmarshal([]byte(rawJSON), &sub); err != nil {
		return fmt.Errorf("invalid subscription JSON: %w", err)
	}
	if sub.Endpoint == "" {
		return fmt.Errorf("subscription endpoint required")
	}
	if err := validatePushEndpoint(sub.Endpoint); err != nil {
		return err
	}
	return database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		return repository.NewSettingWriter(tx).Set(ctx, settingKeyPushSubscription, rawJSON)
	})
}

// allowedPushHosts is the set of vendor push gateways this app may dispatch
// to. Any other host in a Subscribe payload is treated as a probe / SSRF
// attempt — even though VAPID auth would later reject the delivery, we don't
// want our server making outbound requests to arbitrary URLs on the user's
// behalf.
var allowedPushHosts = []string{
	"push.services.mozilla.com",
	"fcm.googleapis.com",
	"notify.windows.com",
	"web.push.apple.com",
}

func validatePushEndpoint(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("subscription endpoint not a URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("subscription endpoint must use https")
	}
	host := parsed.Hostname()
	for _, allowed := range allowedPushHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}
	return fmt.Errorf("subscription endpoint host not allowed: %s", host)
}

func (s *PushService) Unsubscribe(ctx context.Context) error {
	return database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		return repository.NewSettingWriter(tx).Delete(ctx, settingKeyPushSubscription)
	})
}

func (s *PushService) HasSubscription() bool {
	val, err := s.settings.Get(settingKeyPushSubscription)
	return err == nil && val != ""
}

func (s *PushService) SendNotification(ctx context.Context, title, body, url string) error {
	raw, err := s.settings.Get(settingKeyPushSubscription)
	if err != nil {
		return nil // No subscription, silently skip
	}

	var sub webpush.Subscription
	if err := json.Unmarshal([]byte(raw), &sub); err != nil {
		return fmt.Errorf("parse subscription: %w", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"url":   url,
	})

	s.mu.RLock()
	pubKey := s.publicKey
	privKey := s.privateKey
	subContact := s.subject
	s.mu.RUnlock()

	resp, err := webpush.SendNotification(payload, &sub, &webpush.Options{
		HTTPClient:      pushHTTPClient,
		VAPIDPublicKey:  pubKey,
		VAPIDPrivateKey: privKey,
		Subscriber:      subContact,
	})
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 300 {
		return nil
	}

	// 404 Not Found / 410 Gone → subscription is permanently dead. Remove it so
	// the next rating prompt / dead-task / series-ended notification doesn't
	// burn another HTTP call against the same broken endpoint.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		if err := s.Unsubscribe(ctx); err != nil {
			log.Printf("push: status %d — removing dead subscription failed: %v", resp.StatusCode, err)
		} else {
			log.Printf("push: status %d — removed dead subscription", resp.StatusCode)
		}
		return nil
	}

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	apiErr := &matching.APIError{
		Service:    "push",
		StatusCode: resp.StatusCode,
		Body:       string(bodyBytes),
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			apiErr.RetryAfter = time.Duration(secs) * time.Second
		}
	}
	log.Printf("push notification failed: status %d", resp.StatusCode)
	return apiErr
}

// noopNotifier silently ignores all push operations.
type noopNotifier struct{}

// NewNoopNotifier returns a PushNotifier that does nothing.
// Use when VAPID keys are not configured.
func NewNoopNotifier() PushNotifier {
	return noopNotifier{}
}

func (noopNotifier) Subscribe(context.Context, string) error {
	return fmt.Errorf("push notifications not configured")
}

func (noopNotifier) Unsubscribe(context.Context) error {
	return fmt.Errorf("push notifications not configured")
}
func (noopNotifier) HasSubscription() bool { return false }
func (noopNotifier) SendNotification(context.Context, string, string, string) error {
	return nil
}

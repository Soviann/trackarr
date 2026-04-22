package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

// pushHTTPClient caps every outbound push request at 5 seconds. The default
// webpush-go client has no timeout; without this cap a stalled FCM/Mozilla
// endpoint would hang the caller (and, when called inside a DB transaction,
// block the sole write connection).
var pushHTTPClient = &http.Client{Timeout: 5 * time.Second}

// PushNotifier abstracts push notification operations.
// Use NewNoopNotifier() when VAPID keys are not configured.
type PushNotifier interface {
	Subscribe(rawJSON string) error
	Unsubscribe() error
	HasSubscription() bool
	SendNotification(title, body, url string) error
}

// PushService implements PushNotifier with real web push notifications.
type PushService struct {
	settings   *repository.SettingRepository
	publicKey  string
	privateKey string
	subject    string
}

func NewPushService(settings *repository.SettingRepository, publicKey, privateKey, subject string) *PushService {
	return &PushService{
		settings:   settings,
		publicKey:  publicKey,
		privateKey: privateKey,
		subject:    subject,
	}
}

const settingKeyPushSubscription = "push_subscription"

type pushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (s *PushService) Subscribe(rawJSON string) error {
	var sub pushSubscription
	if err := json.Unmarshal([]byte(rawJSON), &sub); err != nil {
		return fmt.Errorf("invalid subscription JSON: %w", err)
	}
	if sub.Endpoint == "" {
		return fmt.Errorf("subscription endpoint required")
	}
	return s.settings.Set(settingKeyPushSubscription, rawJSON)
}

func (s *PushService) Unsubscribe() error {
	return s.settings.Delete(settingKeyPushSubscription)
}

func (s *PushService) HasSubscription() bool {
	val, err := s.settings.Get(settingKeyPushSubscription)
	return err == nil && val != ""
}

func (s *PushService) SendNotification(title, body, url string) error {
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

	resp, err := webpush.SendNotification(payload, &sub, &webpush.Options{
		HTTPClient:      pushHTTPClient,
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
		Subscriber:      s.subject,
	})
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("push notification failed: status %d", resp.StatusCode)
	}

	return nil
}

// noopNotifier silently ignores all push operations.
type noopNotifier struct{}

// NewNoopNotifier returns a PushNotifier that does nothing.
// Use when VAPID keys are not configured.
func NewNoopNotifier() PushNotifier {
	return noopNotifier{}
}

func (noopNotifier) Subscribe(string) error                { return fmt.Errorf("push notifications not configured") }
func (noopNotifier) Unsubscribe() error                    { return fmt.Errorf("push notifications not configured") }
func (noopNotifier) HasSubscription() bool                 { return false }
func (noopNotifier) SendNotification(_, _, _ string) error { return nil }

// Package domain holds the outgoing webhook rules: validation, event catalog, signing, retry schedule and the
// network address blocklist.
package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// PayloadVersion is the version of the event envelope.
const PayloadVersion = "v1"

// Header names of a delivery.
const (
	HeaderSignature = "X-CoreLog-Signature"
	HeaderTimestamp = "X-CoreLog-Timestamp"
	HeaderEvent     = "X-CoreLog-Event"
	HeaderDelivery  = "X-CoreLog-Delivery"
)

// Limits.
const (
	MaxNameLen        = 120
	MaxURLLen         = 2000
	MaxEvents         = 20
	MaxFilterLen      = 2000
	MaxExcerptBytes   = 1024
	MaxAttempts       = 6
	RequestTimeout    = 10 * time.Second
	EventPing         = "ping"
	EventWildcard     = "*"
	secretRandomBytes = 24
)

// Events lists the event types a webhook can subscribe to.
var Events = []string{
	"issue.created", "issue.updated", "issue.transitioned", "issue.deleted", "issue.restored",
	"comment.created", "sprint.started", "sprint.completed", "version.released",
}

// IsSubscribable reports whether the type is a known event or the wildcard.
func IsSubscribable(event string) bool {
	if event == EventWildcard {
		return true
	}
	for _, e := range Events {
		if e == event {
			return true
		}
	}
	return false
}

// DeliveryStatus is the state of one delivery.
type DeliveryStatus string

const (
	StatusPending DeliveryStatus = "pending"
	StatusSuccess DeliveryStatus = "success"
	StatusFailed  DeliveryStatus = "failed"
	StatusDead    DeliveryStatus = "dead"
)

// Errors.
var (
	ErrNotFound         = apperror.WithCode(apperror.KindNotFound, "webhook.not_found", "Webhook no encontrado")
	ErrDeliveryNotFound = apperror.WithCode(apperror.KindNotFound, "webhook.delivery_not_found", "Entrega no encontrada")
	ErrProjectNotInOrg  = apperror.WithCode(apperror.KindNotFound, "webhook.project_not_found", "Proyecto no encontrado en la organizacion")
	ErrUnavailable      = apperror.WithCode(apperror.KindInternal, "webhook.unavailable", "Los webhooks no estan disponibles")
)

func invalid(field, msg string) error {
	return apperror.WithCode(apperror.KindValidation, "webhook.invalid", "Webhook no valido: "+msg).
		WithDetails(apperror.FieldError{Field: field, Message: msg})
}

// Webhook is an outgoing webhook endpoint of an organization.
type Webhook struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	ProjectID   *uuid.UUID
	Name        string
	URL         string
	Events      []string
	FilterQuery *string
	Enabled     bool
	CreatedBy   *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Subscribes reports whether the webhook wants the event type.
func (w Webhook) Subscribes(event string) bool {
	for _, e := range w.Events {
		if e == EventWildcard || e == event {
			return true
		}
	}
	return false
}

// Delivery is one attempt series of one event to one webhook.
type Delivery struct {
	ID              uuid.UUID
	WebhookID       uuid.UUID
	EventID         *int64
	EventType       string
	Payload         []byte
	Status          DeliveryStatus
	Attempts        int
	NextAttemptAt   *time.Time
	ResponseStatus  *int
	ResponseExcerpt string
	DurationMS      int
	CreatedAt       time.Time
}

// ValidateName trims and checks the name.
func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLen {
		return "", invalid("name", "el nombre es obligatorio y admite hasta 120 caracteres")
	}
	return name, nil
}

// ValidateEvents checks and deduplicates the subscription list.
func ValidateEvents(events []string) ([]string, error) {
	if len(events) == 0 || len(events) > MaxEvents {
		return nil, invalid("events", "indica entre 1 y 20 eventos")
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(events))
	for _, e := range events {
		e = strings.TrimSpace(e)
		if !IsSubscribable(e) {
			return nil, invalid("events", "evento desconocido: "+e)
		}
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return out, nil
}

// ValidateURL checks the endpoint: http or https, a host, no credentials in the URL, and, unless private
// targets are allowed, a host that is not a blocked literal address. DNS names are resolved at send time.
func ValidateURL(raw string, allowPrivate bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > MaxURLLen {
		return "", invalid("url", "la URL es obligatoria y admite hasta 2000 caracteres")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return "", invalid("url", "la URL no es valida")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", invalid("url", "solo se admiten URLs http o https")
	}
	if u.User != nil {
		return "", invalid("url", "la URL no puede incluir usuario ni contrasena")
	}
	if !allowPrivate {
		host := strings.ToLower(u.Hostname())
		if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") {
			return "", invalid("url", "la URL apunta a una direccion no permitida")
		}
		if ip, ok := ParseIP(host); ok && IsBlockedIP(ip) {
			return "", invalid("url", "la URL apunta a una direccion no permitida")
		}
	}
	return u.String(), nil
}

// ValidateFilter checks the optional CoreQL filter text length; its syntax is checked by the search port.
func ValidateFilter(q *string) (*string, error) {
	if q == nil {
		return nil, nil
	}
	t := strings.TrimSpace(*q)
	if t == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(t) > MaxFilterLen {
		return nil, invalid("filter_query", "el filtro admite hasta 2000 caracteres")
	}
	return &t, nil
}

// NewSecret returns a random signing secret ("whsec_" plus hex).
func NewSecret() (string, error) {
	b := make([]byte, secretRandomBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(b), nil
}

// Sign returns the signature header value for a body sent at the timestamp: "sha256=" + hex(HMAC-SHA256 of
// "<timestamp>.<body>").
func Sign(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a received signature in constant time. Receivers should also check the timestamp age.
func Verify(secret string, timestamp int64, body []byte, signature string) bool {
	return hmac.Equal([]byte(Sign(secret, timestamp, body)), []byte(signature))
}

// backoff is the wait before the next attempt after the n-th failed attempt (1-based).
var backoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour}

// NextAttempt decides the outcome of an attempt. attempts counts the attempts made including this one. A
// success ends the delivery; a failure schedules a retry, and the sixth failure makes it dead.
func NextAttempt(success bool, attempts int, now time.Time) (DeliveryStatus, *time.Time) {
	if success {
		return StatusSuccess, nil
	}
	if attempts >= MaxAttempts {
		return StatusDead, nil
	}
	next := now.Add(backoff[attempts-1])
	return StatusFailed, &next
}

// Excerpt cuts a response body to the stored size without breaking a UTF-8 sequence.
func Excerpt(b []byte) string {
	if len(b) > MaxExcerptBytes {
		b = b[:MaxExcerptBytes]
	}
	s := strings.ToValidUTF8(string(b), "")
	return strings.ReplaceAll(s, "\x00", "")
}

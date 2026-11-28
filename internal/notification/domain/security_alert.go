package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Security alert notifications. They ignore the in-app opt-out: a person must always learn about a login.
const (
	KindNewLogin = "security.new_login"
	// SecurityLink is the page of the web application that lists the sessions.
	SecurityLink = "/profile/security"
	// LoginAlertWindow is how long a device stays "known": a login from the same browser and masked address
	// seen in this window raises no alert.
	LoginAlertWindow = 30 * 24 * time.Hour
)

// NewLogin describes a successful login that opened a session.
type NewLogin struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	UserAgent string
	IP        string
	At        time.Time
}

// Fingerprint identifies the device of a login: SHA-256 of the user agent and the masked address.
func (l NewLogin) Fingerprint() string {
	sum := sha256.Sum256([]byte(l.UserAgent + "|" + MaskIP(l.IP)))
	return hex.EncodeToString(sum[:])
}

// NewLoginAlert builds the in-app notification of a login. Its data carries what the interface shows and the
// two actions it offers: revoke the new session ("No fui yo") and open the session list.
func NewLoginAlert(l NewLogin) (*Notification, error) {
	device := ParseUserAgent(l.UserAgent)
	ip := MaskIP(l.IP)
	ipText := "IP desconocida"
	if ip != "" {
		ipText = "IP " + ip
	}
	at := l.At.UTC()
	message := fmt.Sprintf("Se inició sesión desde %s (%s) el %s UTC", device.Label(), ipText, at.Format("02/01/2006 15:04"))
	n, err := New(l.UserID, KindNewLogin, message, SecurityLink)
	if err != nil {
		return nil, err
	}
	n.CreatedAt = at
	n.Data = map[string]any{
		"title":      "Nuevo inicio de sesión",
		"session_id": l.SessionID.String(),
		"device":     device.Label(),
		"browser":    device.Browser,
		"os":         device.OS,
		"ip":         ip,
		"at":         at.Format(time.RFC3339),
		"actions": []map[string]string{
			{"id": "revoke_session", "label": "No fui yo", "method": "DELETE", "path": "/users/me/sessions/" + l.SessionID.String()},
			{"id": "view_sessions", "label": "Ver mis sesiones", "link": SecurityLink},
		},
	}
	return n, nil
}

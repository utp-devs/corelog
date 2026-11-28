package usecase

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// SecurityAlertService raises the alert of a login from a new device.
//
// Rule: a login raises an alert unless the same browser and masked address logged in during the last 30 days,
// or the account has no device on record yet (its very first login has nothing to compare with). The in-app
// notice ignores the in-app preference; the email follows the email preference (off disables it).
type SecurityAlertService struct {
	notes    port.Repository
	devices  port.DeviceHistory
	contacts port.ContactReader
	outbox   port.Outbox
	clock    clock.Clock
	baseURL  string
}

var _ port.SecurityAlerts = (*SecurityAlertService)(nil)

// NewSecurityAlertService builds the service. contacts and outbox may be nil: no email is sent then.
func NewSecurityAlertService(notes port.Repository, devices port.DeviceHistory, clk clock.Clock) *SecurityAlertService {
	return &SecurityAlertService{notes: notes, devices: devices, clock: clk}
}

// WithEmail enables the email copy of the alert. baseURL is the public address of the web application.
func (s *SecurityAlertService) WithEmail(contacts port.ContactReader, outbox port.Outbox, baseURL string) *SecurityAlertService {
	s.contacts, s.outbox, s.baseURL = contacts, outbox, strings.TrimRight(baseURL, "/")
	return s
}

// NewLogin implements port.SecurityAlerts.
func (s *SecurityAlertService) NewLogin(ctx context.Context, login domain.NewLogin) error {
	now := s.clock.Now()
	login.At = now
	lastSeen, hasHistory, err := s.devices.Touch(ctx, login.UserID, login.Fingerprint(), now)
	if err != nil {
		return err
	}
	if !hasHistory || (lastSeen != nil && now.Sub(*lastSeen) < domain.LoginAlertWindow) {
		return nil
	}
	note, err := domain.NewLoginAlert(login)
	if err != nil {
		return err
	}
	// The save hook of the repository pushes the notice to the other open sessions of the user.
	if err := s.notes.Save(ctx, note); err != nil {
		return err
	}
	return s.sendEmail(ctx, note, login.SessionID)
}

func (s *SecurityAlertService) sendEmail(ctx context.Context, note *domain.Notification, sessionID uuid.UUID) error {
	if s.contacts == nil || s.outbox == nil {
		return nil
	}
	prefs, err := s.notes.GetPreferences(ctx, note.UserID)
	if err != nil {
		return err
	}
	if prefs.Email == domain.EmailOff {
		return nil
	}
	found, err := s.contacts.Contacts(ctx, []uuid.UUID{note.UserID})
	if err != nil {
		return err
	}
	contact, ok := found[note.UserID]
	if !ok || !contact.Active || contact.Email == "" {
		return nil
	}
	link := s.baseURL + domain.SecurityLink
	text := fmt.Sprintf("%s.\n\nSi no fuiste tú, cierra esa sesión desde %s", note.Message, link)
	body := fmt.Sprintf("<p>%s.</p><p>Si no fuiste tú, <a href=\"%s\">cierra esa sesión</a>.</p>", html.EscapeString(note.Message), html.EscapeString(link))
	_, err = s.outbox.Enqueue(ctx, port.EmailMessage{
		UserID: note.UserID, To: contact.Email, Kind: domain.KindNewLogin, Subject: "Nuevo inicio de sesión en CoreLog",
		Text: text, HTML: body, IdempotencyKey: "security.new_login:" + sessionID.String(),
	}, s.clock.Now())
	return err
}

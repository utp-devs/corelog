package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

type alertRepo struct {
	saved []*domain.Notification
	prefs domain.Preferences
	err   error
}

func (f *alertRepo) Save(_ context.Context, n *domain.Notification) error {
	if f.err != nil {
		return f.err
	}
	n.ID = uuid.New()
	f.saved = append(f.saved, n)
	return nil
}
func (f *alertRepo) ListByUser(context.Context, uuid.UUID, int) ([]*domain.Notification, error) {
	return nil, nil
}
func (f *alertRepo) UnreadCount(context.Context, uuid.UUID) (int, error)  { return 0, nil }
func (f *alertRepo) MarkRead(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (f *alertRepo) MarkAllRead(context.Context, uuid.UUID) error         { return nil }
func (f *alertRepo) GetPreferences(context.Context, uuid.UUID) (domain.Preferences, error) {
	return f.prefs, nil
}
func (f *alertRepo) SavePreferences(context.Context, uuid.UUID, domain.Preferences) error {
	return nil
}

type alertDevices struct {
	last    *time.Time
	history bool
	err     error
}

func (f *alertDevices) Touch(context.Context, uuid.UUID, string, time.Time) (*time.Time, bool, error) {
	return f.last, f.history, f.err
}

type alertContacts struct{ c map[uuid.UUID]port.Contact }

func (f alertContacts) Contacts(context.Context, []uuid.UUID) (map[uuid.UUID]port.Contact, error) {
	return f.c, nil
}

type alertOutbox struct {
	port.Outbox
	sent []port.EmailMessage
}

func (f *alertOutbox) Enqueue(_ context.Context, m port.EmailMessage, _ time.Time) (bool, error) {
	f.sent = append(f.sent, m)
	return true, nil
}

var alertNow = time.Date(2026, 11, 27, 22, 41, 0, 0, time.UTC)

func someLogin() domain.NewLogin {
	return domain.NewLogin{UserID: uuid.New(), SessionID: uuid.New(), UserAgent: "Chrome/1 Windows NT", IP: "203.0.113.9"}
}

func TestNewLoginAlertRules(t *testing.T) {
	old := alertNow.Add(-domain.LoginAlertWindow - time.Hour)
	recent := alertNow.Add(-24 * time.Hour)
	tests := []struct {
		name    string
		devices *alertDevices
		want    int
	}{
		{"unknown device of an account with history", &alertDevices{history: true}, 1},
		{"device last seen long ago", &alertDevices{history: true, last: &old}, 1},
		{"known device", &alertDevices{history: true, last: &recent}, 0},
		{"first login of the account", &alertDevices{history: false}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &alertRepo{prefs: domain.Preferences{InApp: false, Email: domain.EmailOff}}
			svc := NewSecurityAlertService(repo, tt.devices, clock.NewFixed(alertNow))
			if err := svc.NewLogin(context.Background(), someLogin()); err != nil {
				t.Fatal(err)
			}
			if len(repo.saved) != tt.want {
				t.Fatalf("saved %d, want %d", len(repo.saved), tt.want)
			}
			if tt.want == 1 && (repo.saved[0].Kind != domain.KindNewLogin || !repo.saved[0].CreatedAt.Equal(alertNow)) {
				t.Fatalf("alert: %+v", repo.saved[0])
			}
		})
	}
}

func TestNewLoginAlertErrors(t *testing.T) {
	boom := errors.New("boom")
	svc := NewSecurityAlertService(&alertRepo{}, &alertDevices{err: boom}, clock.NewFixed(alertNow))
	if err := svc.NewLogin(context.Background(), someLogin()); !errors.Is(err, boom) {
		t.Fatalf("device error: %v", err)
	}
	svc = NewSecurityAlertService(&alertRepo{err: boom}, &alertDevices{history: true}, clock.NewFixed(alertNow))
	if err := svc.NewLogin(context.Background(), someLogin()); !errors.Is(err, boom) {
		t.Fatalf("save error: %v", err)
	}
	svc = NewSecurityAlertService(&alertRepo{}, &alertDevices{history: true}, clock.NewFixed(alertNow))
	if err := svc.NewLogin(context.Background(), domain.NewLogin{}); err == nil {
		t.Fatal("invalid login must fail")
	}
}

func TestNewLoginAlertEmail(t *testing.T) {
	login := someLogin()
	contacts := alertContacts{c: map[uuid.UUID]port.Contact{login.UserID: {ID: login.UserID, Email: "a@example.test", Active: true}}}
	build := func(prefs domain.Preferences, c port.ContactReader) (*SecurityAlertService, *alertOutbox) {
		ob := &alertOutbox{}
		svc := NewSecurityAlertService(&alertRepo{prefs: prefs}, &alertDevices{history: true}, clock.NewFixed(alertNow)).WithEmail(c, ob, "https://app.test/")
		return svc, ob
	}

	svc, ob := build(domain.Preferences{InApp: false, Email: domain.EmailDaily}, contacts)
	if err := svc.NewLogin(context.Background(), login); err != nil {
		t.Fatal(err)
	}
	if len(ob.sent) != 1 || ob.sent[0].To != "a@example.test" || ob.sent[0].IdempotencyKey != "security.new_login:"+login.SessionID.String() {
		t.Fatalf("email: %+v", ob.sent)
	}
	want := "https://app.test/profile/security"
	if !strings.Contains(ob.sent[0].Text, want) || !strings.Contains(ob.sent[0].HTML, want) {
		t.Fatalf("link missing: %q", ob.sent[0].Text)
	}

	svc, ob = build(domain.Preferences{Email: domain.EmailOff}, contacts)
	_ = svc.NewLogin(context.Background(), login)
	if len(ob.sent) != 0 {
		t.Fatal("email off must not send")
	}
	svc, ob = build(domain.Preferences{Email: domain.EmailImmediate}, alertContacts{c: map[uuid.UUID]port.Contact{}})
	_ = svc.NewLogin(context.Background(), login)
	if len(ob.sent) != 0 {
		t.Fatal("no contact, no email")
	}
}

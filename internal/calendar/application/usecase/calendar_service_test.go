package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/calendar/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

type fakeReader struct {
	events []port.Event
	calls  int
}

func (f *fakeReader) Events(context.Context, uuid.UUID, time.Time, time.Time) ([]port.Event, error) {
	f.calls++
	return f.events, nil
}

type viewAuthorizer struct{ err error }

func (a viewAuthorizer) Authorize(context.Context, string, uuid.UUID, string, uuid.UUID) error {
	return a.err
}

func TestCalendarServiceChecksViewAndRange(t *testing.T) {
	ctx := context.Background()
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)

	reader := &fakeReader{events: []port.Event{{Kind: "issue_due", Date: from, Title: "Entregar"}}}
	svc := NewCalendarService(reader, viewAuthorizer{})
	events, err := svc.Events(ctx, uuid.New(), "agent", uuid.New(), from, to)
	if err != nil || len(events) != 1 {
		t.Fatalf("Events = %d, %v; want 1", len(events), err)
	}

	denied := NewCalendarService(reader, viewAuthorizer{err: apperror.New(apperror.KindForbidden, "forbidden project operation")})
	if _, err := denied.Events(ctx, uuid.New(), "requester", uuid.New(), from, to); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("non member should be forbidden, got %v", err)
	}

	calls := reader.calls
	if _, err := svc.Events(ctx, uuid.New(), "agent", uuid.New(), to, from); !errors.Is(err, domain.ErrInvalidRange) {
		t.Fatalf("reversed range: got %v", err)
	}
	if _, err := svc.Events(ctx, uuid.New(), "agent", uuid.New(), from, from.AddDate(2, 0, 0)); !errors.Is(err, domain.ErrInvalidRange) {
		t.Fatalf("range over one year: got %v", err)
	}
	if reader.calls != calls {
		t.Fatal("invalid ranges must not reach the repository")
	}
}

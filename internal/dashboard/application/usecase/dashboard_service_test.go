package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
)

type fixedReader struct {
	at    time.Time
	err   error
	calls int
}

func (f *fixedReader) Summary(_ context.Context, _ uuid.UUID, now time.Time) (port.Summary, error) {
	f.calls++
	f.at = now
	return port.Summary{AssignedOpen: 3}, f.err
}

func TestDashboardServiceUsesTheCurrentTime(t *testing.T) {
	reader := &fixedReader{}
	svc := NewDashboardService(reader)
	before := time.Now().UTC()

	summary, err := svc.Me(context.Background(), uuid.New())
	if err != nil || summary.AssignedOpen != 3 {
		t.Fatalf("Me = %+v, %v", summary, err)
	}
	if reader.at.Before(before) {
		t.Fatalf("reader received an old time %v", reader.at)
	}

	failing := NewDashboardService(&fixedReader{err: errors.New("db down")})
	if _, err := failing.Me(context.Background(), uuid.New()); err == nil {
		t.Fatal("reader errors must be returned")
	}
}

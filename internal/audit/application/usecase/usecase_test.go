package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/audit/application/port"
	"github.com/jhonsferg/corelog/internal/audit/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// fakeRepo keeps entries in memory and ignores duplicate source event ids,
// like the PostgreSQL adapter does.
type fakeRepo struct {
	entries     []domain.Entry
	saveErr     error
	purgeCutoff time.Time
	purgeCalls  int
	names       map[uuid.UUID]string
}

func (f *fakeRepo) Save(_ context.Context, e domain.Entry) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	if e.SourceEventID != nil {
		for _, existing := range f.entries {
			if existing.SourceEventID != nil && *existing.SourceEventID == *e.SourceEventID {
				return nil
			}
		}
	}
	e.ID = int64(len(f.entries) + 1)
	f.entries = append(f.entries, e)
	return nil
}

func (f *fakeRepo) List(context.Context, port.ListFilter) (port.ListPage, error) {
	return port.ListPage{Entries: f.entries}, nil
}

func (f *fakeRepo) ActorName(_ context.Context, id uuid.UUID) (string, error) {
	return f.names[id], nil
}

func (f *fakeRepo) PurgeOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	f.purgeCalls++
	f.purgeCutoff = cutoff
	return 0, nil
}

func TestConsumerNameAndInterface(t *testing.T) {
	t.Parallel()
	var c events.Consumer = NewConsumer(&fakeRepo{}, clock.NewFixed(time.Now()))
	if c.Name() != "audit" {
		t.Fatalf("Name() = %q, want audit", c.Name())
	}
}

func TestConsumerStoresMappedEvent(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	c := NewConsumer(repo, clock.NewFixed(now))
	org := uuid.New()

	err := c.Handle(context.Background(), events.Event{
		ID: 10, OrgID: &org, Type: "project.created", EntityType: "project",
		Payload: map[string]any{"actor_name": "Ana", "name": "Web"},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(repo.entries) != 1 {
		t.Fatalf("stored %d entries, want 1", len(repo.entries))
	}
	got := repo.entries[0]
	if got.Summary != "Ana creó el proyecto \"Web\"" || got.OrganizationID == nil || *got.OrganizationID != org {
		t.Fatalf("unexpected entry %+v", got)
	}
	if !got.CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt = %v, want clock time %v for events without timestamp", got.CreatedAt, now)
	}
}

func TestConsumerIgnoresUnmappedEvents(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	c := NewConsumer(repo, clock.NewFixed(time.Now()))
	if err := c.Handle(context.Background(), events.Event{ID: 1, Type: "board.column_resized", EntityType: "board"}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(repo.entries) != 0 {
		t.Fatalf("stored %d entries for unmapped event", len(repo.entries))
	}
}

func TestConsumerIsIdempotentForRedelivery(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	c := NewConsumer(repo, clock.NewFixed(time.Now()))
	e := events.Event{ID: 5, Type: "auth.login", EntityType: "user", Payload: map[string]any{"actor_name": "Ana"}}
	for i := 0; i < 3; i++ {
		if err := c.Handle(context.Background(), e); err != nil {
			t.Fatalf("Handle attempt %d: %v", i, err)
		}
	}
	if len(repo.entries) != 1 {
		t.Fatalf("redelivery stored %d entries, want 1", len(repo.entries))
	}
}

func TestConsumerPropagatesRepositoryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("db down")
	c := NewConsumer(&fakeRepo{saveErr: boom}, clock.NewFixed(time.Now()))
	err := c.Handle(context.Background(), events.Event{ID: 1, Type: "auth.login", EntityType: "user"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
}

func TestRetentionJobCutoff(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		days int
		want time.Time
	}{
		{"configured days", 30, now.AddDate(0, 0, -30)},
		{"zero selects default 365", 0, now.AddDate(0, 0, -365)},
		{"negative selects default 365", -5, now.AddDate(0, 0, -365)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeRepo{}
			job := NewRetentionJob(repo, tt.days, clock.NewFixed(now))
			if err := job(context.Background()); err != nil {
				t.Fatalf("job: %v", err)
			}
			if repo.purgeCalls != 1 || !repo.purgeCutoff.Equal(tt.want) {
				t.Fatalf("purge calls=%d cutoff=%v, want 1 and %v", repo.purgeCalls, repo.purgeCutoff, tt.want)
			}
		})
	}
}

func TestConsumerResolvesActorNameWhenPayloadHasNone(t *testing.T) {
	t.Parallel()
	actor := uuid.New()
	repo := &fakeRepo{names: map[uuid.UUID]string{actor: "Luis Gómez"}}
	c := NewConsumer(repo, clock.NewFixed(time.Now()))
	err := c.Handle(context.Background(), events.Event{ID: 3, ActorID: &actor, Type: "auth.password_reset", EntityType: "user", Payload: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if got := repo.entries[0].Summary; got != "Luis Gómez restableció su contraseña" {
		t.Fatalf("summary = %q", got)
	}
}

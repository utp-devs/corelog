package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/domain"
)

// recordingIndex guarda lo que recibe el indice para comprobar la normalizacion y la regla de admin.
type recordingIndex struct {
	admin bool
	query string
	limit int
	calls int
}

func (r *recordingIndex) Search(_ context.Context, _ uuid.UUID, admin bool, query string, limit int) ([]domain.Hit, error) {
	r.calls++
	r.admin = admin
	r.query = query
	r.limit = limit
	return []domain.Hit{{Kind: domain.KindIssue, Title: "x"}}, nil
}

func TestSearchServiceValidatesAndDelegates(t *testing.T) {
	ctx := context.Background()
	index := &recordingIndex{}
	svc := NewSearchService(index)

	if _, err := svc.Search(ctx, uuid.New(), "agent", "  a  ", 10); !errors.Is(err, domain.ErrQueryTooShort) {
		t.Fatalf("one character: got %v, want ErrQueryTooShort", err)
	}
	long := make([]rune, 101)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := svc.Search(ctx, uuid.New(), "agent", string(long), 10); !errors.Is(err, domain.ErrQueryTooLong) {
		t.Fatalf("101 characters: got %v, want ErrQueryTooLong", err)
	}
	if index.calls != 0 {
		t.Fatal("invalid queries must not reach the index")
	}

	if _, err := svc.Search(ctx, uuid.New(), "admin", "  export ", 500); err != nil {
		t.Fatalf("valid query: %v", err)
	}
	if index.query != "export" || !index.admin || index.limit != domain.MaxLimit {
		t.Fatalf("index got query=%q admin=%v limit=%d", index.query, index.admin, index.limit)
	}
}

func TestSearchServiceOnlyGlobalAdminSeesAll(t *testing.T) {
	index := &recordingIndex{}
	svc := NewSearchService(index)
	if _, err := svc.Search(context.Background(), uuid.New(), "requester", "export", 0); err != nil {
		t.Fatal(err)
	}
	if index.admin {
		t.Fatal("a non admin must be scoped to memberships")
	}
	if index.limit != domain.DefaultLimit {
		t.Fatalf("default limit = %d, want %d", index.limit, domain.DefaultLimit)
	}
}

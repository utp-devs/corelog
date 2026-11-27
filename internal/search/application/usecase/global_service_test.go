package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

type fakeIndex struct {
	req  port.GlobalRequest
	hits []domain.GlobalHit
	err  error
	n    int
}

func (f *fakeIndex) SearchAll(_ context.Context, req port.GlobalRequest) ([]domain.GlobalHit, error) {
	f.req = req
	f.n++
	return f.hits, f.err
}

type fakeNoteSource struct {
	query string
	limit int
	hits  []domain.GlobalHit
	err   error
}

func (f *fakeNoteSource) SearchNotes(_ context.Context, _ uuid.UUID, q string, limit int) ([]domain.GlobalHit, error) {
	f.query, f.limit = q, limit
	return f.hits, f.err
}

func TestGlobalServiceSearchAll(t *testing.T) {
	idx := &fakeIndex{hits: []domain.GlobalHit{{Hit: domain.Hit{Kind: domain.KindIssue, ID: "1"}}}}
	notes := &fakeNoteSource{hits: []domain.GlobalHit{{Hit: domain.Hit{Kind: domain.KindNote, ID: "n"}}}}
	svc := NewGlobalService(idx, notes)
	v := port.Viewer{UserID: uuid.New()}

	hits, err := svc.SearchAll(context.Background(), v, "  login  ", []domain.Kind{domain.KindIssue, domain.KindNote, domain.KindTeam}, 500)
	if err != nil || len(hits) != 2 || hits[0].Kind != domain.KindIssue || hits[1].Kind != domain.KindNote {
		t.Fatalf("%v %+v", err, hits)
	}
	if idx.req.Query != "login" || idx.req.Limit != domain.MaxLimit || len(idx.req.Kinds) != 2 || idx.req.Kinds[1] != domain.KindTeam || notes.query != "login" || notes.limit != domain.MaxLimit {
		t.Fatalf("requests: %+v %+v", idx.req, notes)
	}
	// Default kinds, and no index call when only notes are wanted.
	if _, err = svc.SearchAll(context.Background(), v, "ab", nil, 0); err != nil || len(idx.req.Kinds) != 2 || idx.req.Limit != domain.DefaultLimit {
		t.Fatalf("defaults: %v %+v", err, idx.req)
	}
	idx.n = 0
	if hits, _ = svc.SearchAll(context.Background(), v, "ab", []domain.Kind{domain.KindNote}, 0); idx.n != 0 || len(hits) != 1 {
		t.Fatalf("notes only: %d %v", idx.n, hits)
	}
	// Validation and failures.
	for _, q := range []string{"a", "   ", ""} {
		if _, err := svc.SearchAll(context.Background(), v, q, nil, 0); !errors.Is(err, domain.ErrQueryTooShort) {
			t.Errorf("%q: %v", q, err)
		}
	}
	boom := errors.New("boom")
	idx.err = boom
	if _, err := svc.SearchAll(context.Background(), v, "ab", nil, 0); !errors.Is(err, boom) {
		t.Fatalf("index error: %v", err)
	}
	idx.err = nil
	notes.err = boom
	if _, err := svc.SearchAll(context.Background(), v, "ab", []domain.Kind{domain.KindNote}, 0); !errors.Is(err, boom) {
		t.Fatalf("notes error: %v", err)
	}
	// Without a notes source the kind is skipped.
	if hits, err := NewGlobalService(idx, nil).SearchAll(context.Background(), v, "ab", []domain.Kind{domain.KindNote}, 0); err != nil || len(hits) != 0 {
		t.Fatalf("no notes: %v %v", hits, err)
	}
}

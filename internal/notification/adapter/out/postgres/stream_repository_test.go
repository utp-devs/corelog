package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

func streamUser(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'u', 'u', $2, 'x', 'user')`, id, id.String()[:8]+"@example.test"); err != nil {
		t.Fatal(err)
	}
	return id
}

func saveAt(t *testing.T, repo *NotificationRepository, user uuid.UUID, kind, msg string, at time.Time, data map[string]any) *domain.Notification {
	t.Helper()
	n, err := domain.New(user, kind, msg, "/x")
	if err != nil {
		t.Fatal(err)
	}
	n.CreatedAt, n.Data = at, data
	if err := repo.Save(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSaveStoresDataAndRunsTheHook(t *testing.T) {
	db := testdb.New(t)
	repo := NewNotificationRepository(db)
	user := streamUser(t, db)
	var hooked []*domain.Notification
	SetSaveHook(db, func(_ context.Context, n *domain.Notification) { hooked = append(hooked, n) })
	t.Cleanup(func() { SetSaveHook(db, nil) })

	withData := saveAt(t, repo, user, "k", "with data", base, map[string]any{"session_id": "s1", "n": float64(2)})
	plain := saveAt(t, repo, user, "k", "plain", base.Add(time.Second), nil)
	if len(hooked) != 2 || hooked[0].ID != withData.ID || hooked[1].ID != plain.ID {
		t.Fatalf("hook calls: %d", len(hooked))
	}
	items, err := repo.ListByUser(context.Background(), user, 10)
	if err != nil || len(items) != 2 {
		t.Fatalf("list: %v %d", err, len(items))
	}
	if items[1].Data["session_id"] != "s1" || items[1].Data["n"] != float64(2) || items[0].Data != nil {
		t.Fatalf("data: %v / %v", items[1].Data, items[0].Data)
	}
	SetSaveHook(db, nil)
	saveAt(t, repo, user, "k", "after removal", base.Add(2*time.Second), nil)
	if len(hooked) != 2 {
		t.Fatal("removed hook must not run")
	}
}

func TestFindByIDIsScopedToTheUser(t *testing.T) {
	db := testdb.New(t)
	repo := NewNotificationRepository(db)
	owner, other := streamUser(t, db), streamUser(t, db)
	n := saveAt(t, repo, owner, "k", "mine", base, nil)
	got, err := repo.FindByID(context.Background(), owner, n.ID)
	if err != nil || got.Message != "mine" {
		t.Fatalf("owner: %v %v", got, err)
	}
	if _, err := repo.FindByID(context.Background(), other, n.ID); !errors.Is(err, domain.ErrNotificationNotFound) {
		t.Fatalf("other user: %v", err)
	}
	if _, err := repo.FindByID(context.Background(), owner, uuid.New()); !errors.Is(err, domain.ErrNotificationNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestListAfterReplaysInOrderBoundedAndFiltered(t *testing.T) {
	db := testdb.New(t)
	repo := NewNotificationRepository(db)
	user, other := streamUser(t, db), streamUser(t, db)
	ctx := context.Background()
	mySession, otherSession := uuid.New(), uuid.New()

	first := saveAt(t, repo, user, "k", "first", base, nil)
	second := saveAt(t, repo, user, "k", "second", base.Add(time.Second), nil)
	ownAlert := saveAt(t, repo, user, domain.KindNewLogin, "own", base.Add(2*time.Second), map[string]any{"session_id": mySession.String()})
	foreignAlert := saveAt(t, repo, user, domain.KindNewLogin, "other", base.Add(3*time.Second), map[string]any{"session_id": otherSession.String()})
	saveAt(t, repo, other, "k", "someone else", base.Add(4*time.Second), nil)
	_ = ownAlert

	got, err := repo.ListAfter(ctx, user, first.ID, mySession, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != second.ID || got[1].ID != foreignAlert.ID {
		t.Fatalf("replay: %v", messages(got))
	}
	if limited, _ := repo.ListAfter(ctx, user, first.ID, uuid.Nil, 1); len(limited) != 1 || limited[0].ID != second.ID {
		t.Fatalf("limit: %v", messages(limited))
	}
	if none, _ := repo.ListAfter(ctx, user, uuid.New(), mySession, 50); len(none) != 0 {
		t.Fatal("unknown id replays nothing")
	}
	// An id that belongs to another user is unknown for this one.
	if none, _ := repo.ListAfter(ctx, other, first.ID, mySession, 50); len(none) != 0 {
		t.Fatal("foreign id replays nothing")
	}
}

func messages(items []*domain.Notification) []string {
	out := make([]string, 0, len(items))
	for _, n := range items {
		out = append(out, n.Message)
	}
	return out
}

func TestFingerprintTouch(t *testing.T) {
	db := testdb.New(t)
	repo := NewFingerprintRepository(db)
	user := streamUser(t, db)
	ctx := context.Background()

	last, history, err := repo.Touch(ctx, user, "fp1", base)
	if err != nil || last != nil || history {
		t.Fatalf("first login: %v %v %v", last, history, err)
	}
	last, history, err = repo.Touch(ctx, user, "fp1", base.Add(time.Hour))
	if err != nil || last == nil || !last.Equal(base) || !history {
		t.Fatalf("same device: %v %v %v", last, history, err)
	}
	last, history, err = repo.Touch(ctx, user, "fp2", base.Add(2*time.Hour))
	if err != nil || last != nil || !history {
		t.Fatalf("new device of an account with history: %v %v %v", last, history, err)
	}
	if _, history, _ := repo.Touch(ctx, streamUser(t, db), "fp1", base); history {
		t.Fatal("history is per user")
	}
}

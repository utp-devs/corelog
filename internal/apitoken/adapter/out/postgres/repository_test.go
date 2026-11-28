package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/apitoken/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

const testHash = "ab0123456789012345678901234567890123456789012345678901234567890f"

func TestRepositoryLifecycle(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	user, other := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{user, other} {
		if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'u', 'u', $2, 'x', 'user')`, id, id.String()+"@example.test"); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	exp := now.Add(time.Hour)
	tok := domain.Token{UserID: user, Name: "ci", Prefix: "cl_pat_abcd", Scopes: []domain.Scope{domain.ScopeRead, domain.ScopeWrite}, ExpiresAt: &exp, CreatedAt: now}
	if err := repo.Insert(ctx, &tok, testHash); err != nil {
		t.Fatal(err)
	}
	if tok.ID == uuid.Nil {
		t.Fatal("id not filled")
	}
	if n, err := repo.CountActive(ctx, user, now); err != nil || n != 1 {
		t.Fatalf("active %d %v", n, err)
	}
	if n, _ := repo.CountActive(ctx, user, now.Add(2*time.Hour)); n != 0 {
		t.Fatal("expired tokens are not active")
	}
	list, err := repo.List(ctx, user)
	if err != nil || len(list) != 1 || len(list[0].Scopes) != 2 || list[0].Scopes[0] != domain.ScopeRead || list[0].ExpiresAt == nil {
		t.Fatalf("list %+v %v", list, err)
	}
	if list, _ := repo.List(ctx, other); len(list) != 0 {
		t.Fatal("tokens of another user listed")
	}

	found, owner, stored, err := repo.FindByHash(ctx, testHash)
	if err != nil || found.ID != tok.ID || stored != testHash || owner.Role != "user" || owner.Deactivated {
		t.Fatalf("find %+v %+v %q %v", found, owner, stored, err)
	}
	if _, _, _, err := repo.FindByHash(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown hash: %v", err)
	}
	if err := repo.Touch(ctx, tok.ID, now, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	first, _, _, _ := repo.FindByHash(ctx, testHash)
	if first.LastUsedAt == nil {
		t.Fatal("last_used_at not set")
	}
	_ = repo.Touch(ctx, tok.ID, now.Add(time.Second), now.Add(-time.Minute)) // throttled: the stored value is recent
	if again, _, _, _ := repo.FindByHash(ctx, testHash); !again.LastUsedAt.Equal(*first.LastUsedAt) {
		t.Fatal("touch must be throttled")
	}

	if _, err := repo.Revoke(ctx, other, tok.ID, now); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign revoke: %v", err)
	}
	revoked, err := repo.Revoke(ctx, user, tok.ID, now)
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("revoke %+v %v", revoked, err)
	}
	again, _ := repo.Revoke(ctx, user, tok.ID, now.Add(time.Hour))
	if !again.RevokedAt.Equal(*revoked.RevokedAt) {
		t.Fatal("the first revocation instant must be kept")
	}
	if n, _ := repo.CountActive(ctx, user, now); n != 0 {
		t.Fatal("revoked tokens are not active")
	}
	if _, err := db.Exec(`UPDATE users SET deactivated_at = now() WHERE id = $1`, user); err != nil {
		t.Fatal(err)
	}
	if _, owner, _, _ := repo.FindByHash(ctx, testHash); !owner.Deactivated {
		t.Fatal("deactivated owner not reported")
	}
}

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/legal/domain"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

var now = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

func newUser(t *testing.T, repo *Repository) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := repo.db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'Legal', 'Legal', $2, 'x', 'user')`, id, id.String()+"@example.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func publish(t *testing.T, repo *Repository, slug domain.Slug, required bool, at time.Time) *domain.Document {
	t.Helper()
	d, err := domain.NewDraft(slug, "Titulo "+string(slug), "resumen", "cuerpo", &required, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Publish(context.Background(), d, at); err != nil {
		t.Fatalf("publish: %v", err)
	}
	return d
}

func TestPublishAutoIncrementsVersionAndCurrentIsTheLatestInEffect(t *testing.T) {
	repo := NewRepository(testdb.New(t))
	ctx := context.Background()
	v1 := publish(t, repo, domain.SlugPrivacyPolicy, true, now.Add(-2*time.Hour))
	v2 := publish(t, repo, domain.SlugPrivacyPolicy, true, now.Add(-time.Hour))
	future := publish(t, repo, domain.SlugPrivacyPolicy, true, now.Add(time.Hour))
	publish(t, repo, domain.SlugCookies, false, now.Add(-time.Hour))
	if v1.Version != 1 || v2.Version != 2 || future.Version != 3 {
		t.Fatalf("versions = %d %d %d", v1.Version, v2.Version, future.Version)
	}

	cur, err := repo.ListCurrent(ctx, now)
	if err != nil || len(cur) != 2 {
		t.Fatalf("current = %+v %v", cur, err)
	}
	for _, d := range cur {
		if d.BodyMarkdown != "" {
			t.Errorf("list leaks the body of %s", d.Slug)
		}
		if d.Slug == domain.SlugPrivacyPolicy && d.Version != 2 {
			t.Errorf("current privacy version = %d, want 2", d.Version)
		}
	}
	got, err := repo.GetCurrent(ctx, domain.SlugPrivacyPolicy, now)
	if err != nil || got.Version != 2 || got.BodyMarkdown != "cuerpo" {
		t.Fatalf("GetCurrent = %+v %v", got, err)
	}
	if got, err := repo.GetVersion(ctx, domain.SlugPrivacyPolicy, 1); err != nil || got.Version != 1 {
		t.Fatalf("GetVersion = %+v %v", got, err)
	}
	if _, err := repo.GetVersion(ctx, domain.SlugPrivacyPolicy, 9); !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("missing version: %v", err)
	}
	if _, err := repo.GetCurrent(ctx, domain.SlugTermsOfUse, now); !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("missing slug: %v", err)
	}
	all, _ := repo.ListAll(ctx)
	if len(all) != 4 {
		t.Fatalf("ListAll = %d rows", len(all))
	}
}

func TestSaveConsentIsIdempotentAndAnonymizedIPIsStored(t *testing.T) {
	repo := NewRepository(testdb.New(t))
	ctx := context.Background()
	user := newUser(t, repo)
	publish(t, repo, domain.SlugCommunications, false, now.Add(-time.Hour))

	c := domain.NewConsent(user, domain.Ref{Slug: domain.SlugCommunications, Version: 1}, domain.SourceProfile, "203.0.113.200", "agent/1.0", now)
	inserted, err := repo.SaveConsent(ctx, c)
	if err != nil || !inserted {
		t.Fatalf("first save: %v %v", inserted, err)
	}
	if inserted, err := repo.SaveConsent(ctx, c); err != nil || inserted {
		t.Fatalf("second save must be a no-op: %v %v", inserted, err)
	}
	rows, err := repo.ListConsents(ctx, &user, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %+v %v", rows, err)
	}
	if rows[0].IP != "203.0.113.0" || rows[0].UserAgent != "agent/1.0" || rows[0].Source != domain.SourceProfile {
		t.Fatalf("stored consent = %+v", rows[0])
	}

	// Revocation, then accepting again creates a new active row while the audit keeps both.
	if ok, err := repo.RevokeConsent(ctx, user, domain.SlugCommunications, now.Add(time.Minute)); err != nil || !ok {
		t.Fatalf("revoke: %v %v", ok, err)
	}
	if ok, _ := repo.RevokeConsent(ctx, user, domain.SlugCommunications, now); ok {
		t.Fatal("revoking twice must report nothing to revoke")
	}
	active, _ := repo.ActiveConsents(ctx, user)
	if len(active) != 0 {
		t.Fatalf("active after revoke = %+v", active)
	}
	if inserted, err := repo.SaveConsent(ctx, c); err != nil || !inserted {
		t.Fatalf("accept after revoke: %v %v", inserted, err)
	}
	rows, _ = repo.ListConsents(ctx, &user, 10)
	if len(rows) != 2 {
		t.Fatalf("audit rows = %d, want 2", len(rows))
	}
	if all, _ := repo.ListConsents(ctx, nil, 10); len(all) != 2 {
		t.Fatalf("unfiltered audit = %d", len(all))
	}
	if other, _ := repo.ListConsents(ctx, ptr(uuid.New()), 10); len(other) != 0 {
		t.Fatalf("another user sees %d rows", len(other))
	}
}

func ptr[T any](v T) *T { return &v }

func TestSaveConsentWithoutIPOrUserAgentStoresNull(t *testing.T) {
	repo := NewRepository(testdb.New(t))
	ctx := context.Background()
	user := newUser(t, repo)
	publish(t, repo, domain.SlugPrivacyPolicy, true, now.Add(-time.Hour))
	c := domain.NewConsent(user, domain.Ref{Slug: domain.SlugPrivacyPolicy, Version: 1}, domain.SourceRegister, "", "", now)
	if _, err := repo.SaveConsent(ctx, c); err != nil {
		t.Fatal(err)
	}
	var nulls int
	if err := repo.db.Get(&nulls, `SELECT count(*) FROM user_consents WHERE ip IS NULL AND user_agent IS NULL`); err != nil || nulls != 1 {
		t.Fatalf("null columns = %d %v", nulls, err)
	}
}

func TestConsentRollsBackWithTheTransaction(t *testing.T) {
	db := testdb.New(t)
	repo := NewRepository(db)
	ctx := context.Background()
	user := newUser(t, repo)
	publish(t, repo, domain.SlugPrivacyPolicy, true, now.Add(-time.Hour))
	boom := errors.New("boom")
	err := database.NewTxManager(db).InTx(ctx, func(ctx context.Context) error {
		c := domain.NewConsent(user, domain.Ref{Slug: domain.SlugPrivacyPolicy, Version: 1}, domain.SourceRegister, "10.0.0.1", "", now)
		if _, err := repo.SaveConsent(ctx, c); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if rows, _ := repo.ListConsents(ctx, &user, 10); len(rows) != 0 {
		t.Fatalf("consent survived the rollback: %+v", rows)
	}
}

func TestConsentsCascadeWithTheUser(t *testing.T) {
	repo := NewRepository(testdb.New(t))
	ctx := context.Background()
	user := newUser(t, repo)
	publish(t, repo, domain.SlugPrivacyPolicy, true, now.Add(-time.Hour))
	if _, err := repo.SaveConsent(ctx, domain.NewConsent(user, domain.Ref{Slug: domain.SlugPrivacyPolicy, Version: 1}, domain.SourceRegister, "", "", now)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.db.Exec(`DELETE FROM users WHERE id = $1`, user); err != nil {
		t.Fatal(err)
	}
	if rows, _ := repo.ListConsents(ctx, nil, 10); len(rows) != 0 {
		t.Fatalf("consents remain after user deletion: %d", len(rows))
	}
}

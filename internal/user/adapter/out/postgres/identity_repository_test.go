package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

func newIdentityUser(t *testing.T, repo *UserRepository, first, last, email string) *domain.User {
	t.Helper()
	u, err := domain.NewUser(first+" "+last, email, "x", domain.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	u.FirstName, u.LastName = first, last
	if err := repo.Save(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestUserRepositoryIdentityRoundTrip(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewUserRepository(db)

	u := newIdentityUser(t, repo, "María José", "Quispe", "mj@corelog.test")
	phone, err := domain.NewPhone("PE", "+51 912 345 678")
	if err != nil {
		t.Fatal(err)
	}
	u.SetPhone(phone)
	if err := u.SetLocale("es-PE"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, u); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FirstName != "María José" || got.LastName != "Quispe" || got.Name != "María José Quispe" || got.Locale != "es-PE" {
		t.Errorf("identity: %+v", got)
	}
	if got.Phone == nil || got.Phone.E164 != "+51912345678" || got.Phone.Country != "PE" || got.Phone.National != "912345678" || got.PhoneVerifiedAt != nil {
		t.Errorf("phone: %+v", got.Phone)
	}

	// The trigger keeps name consistent when only the parts change through the repository.
	got.SetNames("Lucía", "Soto")
	if err := repo.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	again, _ := repo.FindByID(ctx, u.ID)
	var name string
	if err := db.Get(&name, `SELECT name FROM users WHERE id = $1`, u.ID); err != nil || name != "Lucía Soto" || again.Name != "Lucía Soto" {
		t.Errorf("name column = %q (%v)", name, err)
	}

	// Clearing the phone removes every phone column, including the verification.
	again.SetPhone(nil)
	if err := repo.Update(ctx, again); err != nil {
		t.Fatal(err)
	}
	cleared, _ := repo.FindByID(ctx, u.ID)
	if cleared.Phone != nil || cleared.PhoneVerifiedAt != nil {
		t.Errorf("not cleared: %+v", cleared)
	}
}

func TestUserRepositorySaveLegacyNameOnly(t *testing.T) {
	db := testdb.New(t)
	repo := NewUserRepository(db)
	u, _ := domain.NewUser("Juan Carlos Perez Lopez", "legacy@corelog.test", "x", domain.RoleUser)
	if err := repo.Save(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.FindByID(context.Background(), u.ID)
	if got.FirstName != "Juan Carlos" || got.LastName != "Perez Lopez" || got.Name != "Juan Carlos Perez Lopez" {
		t.Errorf("%+v", got)
	}
}

// TestSplitNameMatchesSQL guards that the Go and SQL splitting rules agree.
func TestSplitNameMatchesSQL(t *testing.T) {
	db := testdb.New(t)
	for _, in := range []string{"Madonna", "Ana Perez", "Ana Perez Lopez", "Juan Carlos Perez Lopez", "A B C D E", "  Luis   Gomez  "} {
		var first, last string
		if err := db.QueryRow(`SELECT first_part, last_part FROM users_split_name($1)`, in).Scan(&first, &last); err != nil {
			t.Fatal(err)
		}
		gf, gl := domain.SplitName(in)
		if first != gf || last != gl {
			t.Errorf("%q: sql=(%q,%q) go=(%q,%q)", in, first, last, gf, gl)
		}
	}
}

func TestUserRepositoryVerifiedPhoneIsUnique(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	repo := NewUserRepository(db)
	a := newIdentityUser(t, repo, "Ana", "Uno", "a@corelog.test")
	b := newIdentityUser(t, repo, "Beto", "Dos", "b@corelog.test")
	p, _ := domain.NewPhone("PE", "912345678")
	for _, u := range []*domain.User{a, b} {
		pp := *p
		u.SetPhone(&pp)
		if err := repo.Update(ctx, u); err != nil {
			t.Fatalf("unverified numbers must not collide: %v", err)
		}
	}
	a.MarkPhoneVerified(time.Now())
	if err := repo.Update(ctx, a); err != nil {
		t.Fatal(err)
	}
	b.MarkPhoneVerified(time.Now())
	if err := repo.Update(ctx, b); !errors.Is(err, domain.ErrPhoneTaken) {
		t.Fatalf("second verified owner: %v", err)
	}
}

func TestPhoneVerificationRepository(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	users := NewUserRepository(db)
	repo := NewPhoneVerificationRepository(db)
	u := newIdentityUser(t, users, "Ana", "Uno", "a@corelog.test")
	other := newIdentityUser(t, users, "Beto", "Dos", "b@corelog.test")
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

	mk := func(user *domain.User, code string, at time.Time) *domain.PhoneVerification {
		return &domain.PhoneVerification{UserID: user.ID, E164: "+51912345678", CodeHash: domain.HashPhoneCode(code), ExpiresAt: at.Add(domain.PhoneCodeTTL), CreatedAt: at}
	}
	for i := 0; i < 3; i++ {
		if err := repo.CreateWithinLimit(ctx, mk(u, "123456", now), now.Add(-time.Hour), 3); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if err := repo.CreateWithinLimit(ctx, mk(u, "123456", now), now.Add(-time.Hour), 3); !errors.Is(err, domain.ErrPhoneSendLimit) {
		t.Fatalf("fourth send: %v", err)
	}
	if err := repo.CreateWithinLimit(ctx, mk(other, "654321", now), now.Add(-time.Hour), 3); err != nil {
		t.Fatalf("the limit must be per user: %v", err)
	}
	if err := repo.CreateWithinLimit(ctx, mk(u, "123456", now.Add(2*time.Hour)), now.Add(time.Hour), 3); err != nil {
		t.Fatalf("old sends must leave the window: %v", err)
	}

	v, err := repo.FindLatestOpen(ctx, u.ID, "+51912345678")
	if err != nil || !v.Matches("123456") || v.Matches("000000") {
		t.Fatalf("latest: %+v %v", v, err)
	}
	if _, err := repo.FindLatestOpen(ctx, u.ID, "+51900000000"); !errors.Is(err, domain.ErrPhoneCodeExpired) {
		t.Errorf("other number: %v", err)
	}
	for want := 1; want <= 2; want++ {
		if n, err := repo.RegisterAttempt(ctx, v.ID); err != nil || n != want {
			t.Fatalf("attempt: %d %v", n, err)
		}
	}
	if err := repo.Consume(ctx, u.ID, v.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindLatestOpen(ctx, u.ID, "+51912345678"); !errors.Is(err, domain.ErrPhoneCodeExpired) {
		t.Errorf("consumed codes must not be found: %v", err)
	}
	var stored []byte
	if err := db.Get(&stored, `SELECT code_hash FROM user_phone_verifications WHERE id = $1`, v.ID); err != nil || len(stored) != 32 || string(stored) == "123456" {
		t.Errorf("code must be stored hashed: %v", err)
	}
}

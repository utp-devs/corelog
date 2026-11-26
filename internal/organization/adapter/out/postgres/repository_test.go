package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/organization/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

var testNow = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

// newUser inserts a platform user and returns its ID. The password hash is a placeholder, never used.
func newUser(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(context.Background(), `
INSERT INTO users (id, name, display_name, email, password_hash, role)
VALUES ($1, $2, $2, $3, 'not-a-real-hash', 'user')`,
		id, "Usuario de prueba", id.String()+"@example.test")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

// newOrg builds and stores an organization with the given slug, owned by owner.
func newOrg(t *testing.T, repo *postgres.Repository, owner uuid.UUID, name, slug string) *domain.Organization {
	t.Helper()
	org, err := domain.NewOrganization(name, slug, "Descripción", owner, testNow)
	if err != nil {
		t.Fatalf("new organization: %v", err)
	}
	if err := repo.Create(context.Background(), org); err != nil {
		t.Fatalf("create %s: %v", slug, err)
	}
	if org.ID == uuid.Nil {
		t.Fatal("create did not assign an ID")
	}
	return org
}

func requireIs(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestCreateAndGetByID(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	owner := newUser(t, db)

	org := newOrg(t, repo, owner, "Equipo Norte", "equipo-norte")

	got, err := repo.GetByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Name != "Equipo Norte" || got.Slug != "equipo-norte" || got.Description != "Descripción" {
		t.Fatalf("unexpected org: %+v", got)
	}
	if got.CreatedBy != owner {
		t.Fatalf("created by = %s, want %s", got.CreatedBy, owner)
	}
	if !got.CreatedAt.Equal(testNow) || !got.UpdatedAt.Equal(testNow) {
		t.Fatalf("timestamps = %v / %v, want %v", got.CreatedAt, got.UpdatedAt, testNow)
	}
	if got.IsDeleted() {
		t.Fatal("new organization must not be deleted")
	}

	bySlug, err := repo.GetBySlug(ctx, "EQUIPO-norte")
	if err != nil || bySlug.ID != org.ID {
		t.Fatalf("get by slug (case-insensitive): %v, %+v", err, bySlug)
	}
}

func TestGetMissingReturnsNotFound(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	requireIs(t, err, domain.ErrNotFound)
	_, err = repo.GetBySlug(ctx, "no-existe")
	requireIs(t, err, domain.ErrNotFound)
}

func TestSlugUniquenessIsCaseInsensitive(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	owner := newUser(t, db)
	newOrg(t, repo, owner, "Acme", "acme-co")

	// The repository receives the slug as given; a mixed-case duplicate must still clash.
	dup := &domain.Organization{Name: "Otra", Slug: "Acme-Co", CreatedBy: owner, CreatedAt: testNow, UpdatedAt: testNow, Settings: domain.DefaultSettings()}
	requireIs(t, repo.Create(ctx, dup), domain.ErrSlugTaken)
}

func TestSlugStaysReservedAfterSoftDelete(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	owner := newUser(t, db)
	org := newOrg(t, repo, owner, "Antigua", "antigua")

	if err := repo.SoftDelete(ctx, org.ID, testNow.Add(time.Hour)); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	again, err := domain.NewOrganization("Nueva", "antigua", "", owner, testNow)
	if err != nil {
		t.Fatalf("new organization: %v", err)
	}
	requireIs(t, repo.Create(ctx, again), domain.ErrSlugTaken)

	deleted, err := repo.GetBySlug(ctx, "antigua")
	if err != nil {
		t.Fatalf("get by slug must include deleted orgs: %v", err)
	}
	if !deleted.IsDeleted() || deleted.DeletedAt == nil || !deleted.DeletedAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("deleted_at = %v", deleted.DeletedAt)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	owner := newUser(t, db)

	org := newOrg(t, repo, owner, "Defaults", "defaults-org")
	if org.Settings != domain.DefaultSettings() {
		t.Fatalf("settings = %+v, want defaults %+v", org.Settings, domain.DefaultSettings())
	}
	got, err := repo.GetByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Settings != domain.DefaultSettings() {
		t.Fatalf("stored defaults = %+v", got.Settings)
	}

	custom := domain.Settings{
		WhoCanCreateProjects:     domain.CreatorAdmins,
		WhoCanCreateTeams:        domain.CreatorMembers,
		DefaultProjectVisibility: domain.VisibilityPrivate,
		Require2FA:               true,
		MaxUploadBytes:           5 << 20,
		HoursPerDay:              7.5,
		DaysPerWeek:              4.5,
		Timezone:                 "America/Bogota",
	}
	got.Settings = custom
	got.Name = "Renombrada"
	got.Description = "Nueva descripción"
	got.UpdatedAt = testNow.Add(time.Minute)
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	back, err := repo.GetByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if back.Settings != custom {
		t.Fatalf("settings round trip = %+v, want %+v", back.Settings, custom)
	}
	if back.Name != "Renombrada" || back.Description != "Nueva descripción" || !back.UpdatedAt.Equal(testNow.Add(time.Minute)) {
		t.Fatalf("update not persisted: %+v", back)
	}
	if back.Settings.Timezone != "America/Bogota" {
		t.Fatalf("timezone = %q", back.Settings.Timezone)
	}
}

func TestUpdateMissingReturnsNotFound(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	org := &domain.Organization{ID: uuid.New(), Name: "X", Slug: "x-missing", Settings: domain.DefaultSettings(), UpdatedAt: testNow}
	requireIs(t, repo.Update(context.Background(), org), domain.ErrNotFound)
}

func TestMembersLifecycle(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	owner := newUser(t, db)
	other := newUser(t, db)
	org := newOrg(t, repo, owner, "Miembros", "miembros")

	ownerMember, err := domain.NewMember(org.ID, owner, domain.RoleOwner, testNow)
	if err != nil {
		t.Fatalf("new member: %v", err)
	}
	if err := repo.AddMember(ctx, ownerMember); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	m, err := domain.NewMember(org.ID, other, domain.RoleMember, testNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("new member: %v", err)
	}
	if err := repo.AddMember(ctx, m); err != nil {
		t.Fatalf("add member: %v", err)
	}

	dup, _ := domain.NewMember(org.ID, other, domain.RoleGuest, testNow)
	requireIs(t, repo.AddMember(ctx, dup), domain.ErrMemberExists)

	got, err := repo.GetMember(ctx, org.ID, other)
	if err != nil || got.Role != domain.RoleMember || !got.JoinedAt.Equal(testNow.Add(time.Minute)) {
		t.Fatalf("get member: %v, %+v", err, got)
	}

	list, err := repo.ListMembers(ctx, org.ID)
	if err != nil || len(list) != 2 || list[0].UserID != owner {
		t.Fatalf("list members: %v, %+v", err, list)
	}

	n, err := repo.CountOwners(ctx, org.ID)
	if err != nil || n != 1 {
		t.Fatalf("count owners = %d, %v; want 1", n, err)
	}

	if err := repo.UpdateMemberRole(ctx, org.ID, other, domain.RoleOwner); err != nil {
		t.Fatalf("update role: %v", err)
	}
	n, _ = repo.CountOwners(ctx, org.ID)
	if n != 2 {
		t.Fatalf("owners after promotion = %d, want 2", n)
	}

	requireIs(t, repo.UpdateMemberRole(ctx, org.ID, uuid.New(), domain.RoleAdmin), domain.ErrMemberNotFound)
	_, err = repo.GetMember(ctx, org.ID, uuid.New())
	requireIs(t, err, domain.ErrMemberNotFound)

	if err := repo.RemoveMember(ctx, org.ID, other); err != nil {
		t.Fatalf("remove: %v", err)
	}
	requireIs(t, repo.RemoveMember(ctx, org.ID, other), domain.ErrMemberNotFound)
	n, _ = repo.CountOwners(ctx, org.ID)
	if n != 1 {
		t.Fatalf("owners after removal = %d, want 1", n)
	}
}

func TestListByUserOrdersByNameAndSkipsDeleted(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	user := newUser(t, db)
	stranger := newUser(t, db)

	zeta := newOrg(t, repo, user, "Zeta", "zeta-org")
	alpha := newOrg(t, repo, user, "Alpha", "alpha-org")
	gone := newOrg(t, repo, user, "Borrada", "borrada-org")
	newOrg(t, repo, stranger, "Ajena", "ajena-org")

	join := func(orgID uuid.UUID, r domain.Role) {
		t.Helper()
		m, _ := domain.NewMember(orgID, user, r, testNow)
		if err := repo.AddMember(ctx, m); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}
	join(zeta.ID, domain.RoleOwner)
	join(alpha.ID, domain.RoleGuest)
	join(gone.ID, domain.RoleAdmin)
	if err := repo.SoftDelete(ctx, gone.ID, testNow); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	list, err := repo.ListByUser(ctx, user)
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %d entries, want 2", len(list))
	}
	if list[0].Organization.Name != "Alpha" || list[0].Role != domain.RoleGuest {
		t.Fatalf("first = %+v", list[0])
	}
	if list[1].Organization.Name != "Zeta" || list[1].Role != domain.RoleOwner {
		t.Fatalf("second = %+v", list[1])
	}

	none, err := repo.ListByUser(ctx, uuid.New())
	if err != nil || len(none) != 0 {
		t.Fatalf("unknown user: %v, %d entries", err, len(none))
	}
}

func TestSoftDeleteHidesFromLookupsButNotSlug(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	owner := newUser(t, db)
	org := newOrg(t, repo, owner, "Ocultar", "ocultar")
	owners := domain.Member{OrganizationID: org.ID, UserID: owner, Role: domain.RoleOwner, JoinedAt: testNow}
	if err := repo.AddMember(ctx, &owners); err != nil {
		t.Fatalf("add owner: %v", err)
	}

	if err := repo.SoftDelete(ctx, org.ID, testNow); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	_, err := repo.GetByID(ctx, org.ID)
	requireIs(t, err, domain.ErrNotFound)
	list, err := repo.ListByUser(ctx, owner)
	if err != nil || len(list) != 0 {
		t.Fatalf("list after delete = %d, %v", len(list), err)
	}
	_, err = repo.GetMember(ctx, org.ID, owner)
	requireIs(t, err, domain.ErrMemberNotFound)
	n, err := repo.CountOwners(ctx, org.ID)
	if err != nil || n != 0 {
		t.Fatalf("owners of deleted org = %d, %v; want 0", n, err)
	}
	members, err := repo.ListMembers(ctx, org.ID)
	if err != nil || len(members) != 0 {
		t.Fatalf("members of deleted org = %d, %v", len(members), err)
	}
	got, err := repo.GetBySlug(ctx, "ocultar")
	if err != nil || got.ID != org.ID {
		t.Fatalf("get by slug must still find the deleted org: %v", err)
	}
	requireIs(t, repo.SoftDelete(ctx, org.ID, testNow), domain.ErrNotFound)
}

func TestTransactionParticipation(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	tm := database.NewTxManager(db)
	ctx := context.Background()
	owner := newUser(t, db)

	boom := errors.New("boom")
	err := tm.InTx(ctx, func(ctx context.Context) error {
		org, err := domain.NewOrganization("Revertida", "revertida", "", owner, testNow)
		if err != nil {
			return err
		}
		if err := repo.Create(ctx, org); err != nil {
			return err
		}
		m, _ := domain.NewMember(org.ID, owner, domain.RoleOwner, testNow)
		if err := repo.AddMember(ctx, m); err != nil {
			return err
		}
		return boom
	})
	requireIs(t, err, boom)

	_, err = repo.GetBySlug(ctx, "revertida")
	requireIs(t, err, domain.ErrNotFound)
	var members int
	if err := db.GetContext(ctx, &members, `SELECT COUNT(*) FROM organization_members WHERE user_id = $1`, owner); err != nil {
		t.Fatalf("count members: %v", err)
	}
	if members != 0 {
		t.Fatalf("rolled back membership still present: %d", members)
	}

	err = tm.InTx(ctx, func(ctx context.Context) error {
		org, err := domain.NewOrganization("Confirmada", "confirmada", "", owner, testNow)
		if err != nil {
			return err
		}
		return repo.Create(ctx, org)
	})
	if err != nil {
		t.Fatalf("committed tx: %v", err)
	}
	if _, err := repo.GetBySlug(ctx, "confirmada"); err != nil {
		t.Fatalf("committed org missing: %v", err)
	}
}

var _ port.Repository = (*postgres.Repository)(nil)

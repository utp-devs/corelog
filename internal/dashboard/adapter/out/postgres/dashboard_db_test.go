package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/dashboard/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

const defaultOrg = "00000000-0000-4000-8000-000000000002"

type countingQuerier struct {
	*sqlx.DB
	n *atomic.Int32
}

func (c countingQuerier) GetContext(ctx context.Context, dest any, q string, a ...any) error {
	c.n.Add(1)
	return c.DB.GetContext(ctx, dest, q, a...)
}
func (c countingQuerier) SelectContext(ctx context.Context, dest any, q string, a ...any) error {
	c.n.Add(1)
	return c.DB.SelectContext(ctx, dest, q, a...)
}
func (c countingQuerier) ExecContext(ctx context.Context, q string, a ...any) (sql.Result, error) {
	c.n.Add(1)
	return c.DB.ExecContext(ctx, q, a...)
}

type fixture struct {
	t       *testing.T
	db      *sqlx.DB
	repo    *Repository
	data    *DataSource
	counter *atomic.Int32
	org     uuid.UUID
	otherOr uuid.UUID
	owner   uuid.UUID // member, lead of both projects
	member  uuid.UUID // member without project role
	admin   uuid.UUID // organization admin
	teamer  uuid.UUID // member through a team
	other   uuid.UUID // member of another organization
	open    uuid.UUID
	secret  uuid.UUID
	team    uuid.UUID
}

func (f *fixture) exec(q string, args ...any) {
	f.t.Helper()
	if _, err := f.db.Exec(q, args...); err != nil {
		f.t.Fatalf("%v\n%s", err, q)
	}
}

func (f *fixture) user(name string) uuid.UUID {
	id := uuid.New()
	f.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, $2, $2, $3, 'x', 'user')`, id, name, name+"@example.test")
	return id
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := testdb.New(t)
	counter := &atomic.Int32{}
	repo, data := NewRepository(db), NewDataSource(db)
	cq := countingQuerier{DB: db, n: counter}
	repo.db, data.db = cq, cq
	f := &fixture{t: t, db: db, repo: repo, data: data, counter: counter, org: uuid.MustParse(defaultOrg), otherOr: uuid.New(), open: uuid.New(), secret: uuid.New(), team: uuid.New()}
	f.owner, f.member, f.admin, f.teamer, f.other = f.user("ana"), f.user("beto"), f.user("carla"), f.user("dani"), f.user("eva")
	f.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member'), ($1, $3, 'member'), ($1, $4, 'admin'), ($1, $5, 'member')`, f.org, f.owner, f.member, f.admin, f.teamer)
	f.exec(`INSERT INTO organizations (id, name, slug) VALUES ($1, 'Otra', 'otra')`, f.otherOr)
	f.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'owner')`, f.otherOr, f.other)
	f.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $3, 'OPN', 'Abierto', 'kanban', 'scrum', $4, 'open'), ($2, $3, 'SEC', 'Secreto', 'kanban', 'scrum', $4, 'private')`, f.open, f.secret, f.org, f.owner)
	f.exec(`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $3, 'lead'), ($2, $3, 'lead')`, f.open, f.secret, f.owner)
	f.exec(`INSERT INTO teams (id, name, organization_id) VALUES ($1, 'Equipo', $2)`, f.team, f.org)
	f.exec(`INSERT INTO team_members (team_id, user_id) VALUES ($1, $2)`, f.team, f.teamer)
	return f
}

func v(id uuid.UUID) port.Viewer { return port.Viewer{UserID: id} }

func (f *fixture) create(owner uuid.UUID, name string, gadgets ...domain.Gadget) *domain.Dashboard {
	f.t.Helper()
	now := time.Now().UTC()
	d := &domain.Dashboard{OwnerID: owner, OrganizationID: f.org, Name: name, Layout: domain.Layout2, CreatedAt: now, UpdatedAt: now, Gadgets: gadgets}
	if err := f.repo.Create(context.Background(), d); err != nil {
		f.t.Fatal(err)
	}
	return d
}

func gadget(t string, col, pos int, cfg string) domain.Gadget {
	return domain.Gadget{Type: t, Title: t, Config: json.RawMessage(cfg), ColumnIndex: col, Position: pos}
}

func TestDashboardAccessMatrixPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := f.create(f.owner, "Compartido", gadget(domain.GadgetText, 0, 0, `{"markdown":"hola"}`))
	now := time.Now().UTC()

	shares := []domain.Share{
		{Type: domain.ShareUser, TargetID: f.member, Permission: domain.PermissionView},
		{Type: domain.ShareTeam, TargetID: f.team, Permission: domain.PermissionEdit},
	}
	if err := f.repo.ReplaceShares(ctx, d.ID, f.org, shares, now); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		who    string
		id     uuid.UUID
		access domain.Access
	}{
		{"owner", f.owner, domain.AccessOwner},
		{"user share", f.member, domain.AccessView},
		{"team share edit", f.teamer, domain.AccessEdit},
		{"org admin", f.admin, domain.AccessView},
		{"other organization", f.other, domain.AccessNone},
	}
	for _, c := range cases {
		got, err := f.repo.Get(ctx, v(c.id), d.ID)
		if c.access == domain.AccessNone {
			if apperror.CodeOf(err) != domain.CodeNotFound {
				t.Errorf("%s: err = %v, want not found", c.who, err)
			}
			continue
		}
		if err != nil || got.Access != c.access || len(got.Gadgets) != 1 || !got.Shared {
			t.Errorf("%s: %+v %v, want %v", c.who, got, err, c.access)
		}
	}

	// Organization share reaches every member, not the outsider.
	if err := f.repo.ReplaceShares(ctx, d.ID, f.org, []domain.Share{{Type: domain.ShareOrganization, TargetID: f.org, Permission: domain.PermissionView}}, now); err != nil {
		t.Fatal(err)
	}
	if got, err := f.repo.Get(ctx, v(f.teamer), d.ID); err != nil || got.Access != domain.AccessView {
		t.Fatalf("organization share: %+v %v", got, err)
	}
	if _, err := f.repo.Get(ctx, v(f.other), d.ID); err == nil {
		t.Fatal("organization share leaked to another organization")
	}
	// A project share reaches who can browse the project: the secret project does not reach a plain member.
	if err := f.repo.ReplaceShares(ctx, d.ID, f.org, []domain.Share{{Type: domain.ShareProject, TargetID: f.secret, Permission: domain.PermissionView}}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Get(ctx, v(f.member), d.ID); err == nil {
		t.Fatal("share with a private project reached a non-member of it")
	}
	if err := f.repo.ReplaceShares(ctx, d.ID, f.org, []domain.Share{{Type: domain.ShareProject, TargetID: f.open, Permission: domain.PermissionView}}, now); err != nil {
		t.Fatal(err)
	}
	if got, err := f.repo.Get(ctx, v(f.member), d.ID); err != nil || got.Access != domain.AccessView {
		t.Fatalf("share with an open project: %+v %v", got, err)
	}

	// Targets of another organization, or that do not exist, are rejected and nothing changes.
	for name, bad := range map[string]domain.Share{
		"user of another organization": {Type: domain.ShareUser, TargetID: f.other, Permission: domain.PermissionView},
		"unknown team":                 {Type: domain.ShareTeam, TargetID: uuid.New(), Permission: domain.PermissionView},
		"unknown project":              {Type: domain.ShareProject, TargetID: uuid.New(), Permission: domain.PermissionView},
		"other organization":           {Type: domain.ShareOrganization, TargetID: f.otherOr, Permission: domain.PermissionView},
	} {
		err := f.repo.ReplaceShares(ctx, d.ID, f.org, []domain.Share{bad}, now)
		if apperror.CodeOf(err) != domain.CodeInvalidShare {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	list, err := f.repo.Shares(ctx, d.ID)
	if err != nil || len(list) != 1 || list[0].TargetName != "Abierto" {
		t.Fatalf("shares after rejections: %+v %v", list, err)
	}
	if err := f.repo.ReplaceShares(ctx, d.ID, f.org, nil, now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Get(ctx, v(f.member), d.ID); err == nil {
		t.Fatal("removing the shares did not revoke access")
	}
}

func TestDashboardCrudPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	d := f.create(f.owner, "Uno", gadget(domain.GadgetText, 1, 1, `{"markdown":"b"}`), gadget(domain.GadgetAssignedToMe, 1, 0, `{"limit":10,"include_done":false}`))
	got, err := f.repo.Get(ctx, v(f.owner), d.ID)
	if err != nil || got.OwnerName != "ana" || got.Layout != domain.Layout2 || len(got.Gadgets) != 2 || got.Gadgets[0].Type != domain.GadgetAssignedToMe {
		t.Fatalf("get: %+v %v", got, err)
	}
	if string(got.Gadgets[1].Config) != `{"markdown": "b"}` {
		t.Fatalf("config = %s", got.Gadgets[1].Config)
	}
	if err := f.repo.Create(ctx, &domain.Dashboard{OwnerID: f.owner, OrganizationID: f.org, Name: "uno", Layout: domain.Layout1}); apperror.CodeOf(err) != domain.CodeNameTaken {
		t.Fatalf("duplicate name: %v", err)
	}
	// Another owner can reuse the name.
	f.create(f.member, "Uno")

	// The default is unique per owner and swaps atomically.
	two := f.create(f.owner, "Dos")
	yes, no := true, false
	now := time.Now().UTC()
	if err := f.repo.UpdateFields(ctx, d.ID, port.FieldsPatch{IsDefault: &yes}, now); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.UpdateFields(ctx, two.ID, port.FieldsPatch{IsDefault: &yes}, now); err != nil {
		t.Fatal(err)
	}
	def, err := f.repo.FindDefault(ctx, v(f.owner))
	if err != nil || def == nil || def.ID != two.ID {
		t.Fatalf("default: %+v %v", def, err)
	}
	if first, _ := f.repo.Get(ctx, v(f.owner), d.ID); first.IsDefault {
		t.Fatal("two defaults")
	}
	if err := f.repo.UpdateFields(ctx, two.ID, port.FieldsPatch{IsDefault: &no}, now); err != nil {
		t.Fatal(err)
	}
	if def, _ := f.repo.FindDefault(ctx, v(f.owner)); def != nil {
		t.Fatal("default not cleared")
	}
	if def, _ := f.repo.FindDefault(ctx, v(f.member)); def != nil {
		t.Fatal("somebody else's default found")
	}
	// A default created through Create replaces the previous one.
	f.create(f.owner, "Tres")
	third := &domain.Dashboard{OwnerID: f.owner, OrganizationID: f.org, Name: "Cuatro", Layout: domain.Layout1, IsDefault: true, CreatedAt: now, UpdatedAt: now}
	if err := f.repo.Create(ctx, third); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.UpdateFields(ctx, d.ID, port.FieldsPatch{IsDefault: &yes}, now); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.UpdateFields(ctx, uuid.New(), port.FieldsPatch{}, now); apperror.CodeOf(err) != domain.CodeNotFound {
		t.Fatalf("update of a missing dashboard: %v", err)
	}

	// Fields.
	name, desc, layout := "Renombrado", "descripción", domain.Layout21
	if err := f.repo.UpdateFields(ctx, d.ID, port.FieldsPatch{Name: &name, Description: &desc, Layout: &layout}, now); err != nil {
		t.Fatal(err)
	}
	got, _ = f.repo.Get(ctx, v(f.owner), d.ID)
	if got.Name != name || got.Description != desc || got.Layout != domain.Layout21 || !got.UpdatedAt.Equal(now.Truncate(time.Microsecond)) {
		t.Fatalf("fields: %+v", got)
	}
	if err := f.repo.UpdateFields(ctx, d.ID, port.FieldsPatch{Name: &[]string{"Dos"}[0]}, now); apperror.CodeOf(err) != domain.CodeNameTaken {
		t.Fatalf("rename to a used name: %v", err)
	}

	// Replacing gadgets keeps ids and swaps the rest.
	keep := got.Gadgets[0]
	stored, err := f.repo.ReplaceGadgets(ctx, d.ID, nil, []domain.Gadget{
		{ID: keep.ID, Type: keep.Type, Title: "Mía", Config: keep.Config, Position: 1, ColumnIndex: 0},
		gadget(domain.GadgetFavorites, 0, 0, `{"types":[],"limit":10}`),
	}, now)
	if err != nil || len(stored) != 2 || stored[0].ID != keep.ID || stored[1].ID == uuid.Nil {
		t.Fatalf("replace: %+v %v", stored, err)
	}
	got, _ = f.repo.Get(ctx, v(f.owner), d.ID)
	if len(got.Gadgets) != 2 || got.Gadgets[0].Type != domain.GadgetFavorites || got.Gadgets[1].ID != keep.ID || got.Gadgets[1].Title != "Mía" {
		t.Fatalf("after replace: %+v", got.Gadgets)
	}
	one := domain.Layout1
	if _, err := f.repo.ReplaceGadgets(ctx, d.ID, &one, nil, now); err != nil {
		t.Fatal(err)
	}
	if got, _ = f.repo.Get(ctx, v(f.owner), d.ID); len(got.Gadgets) != 0 || got.Layout != domain.Layout1 {
		t.Fatalf("empty replace: %+v", got)
	}
	if _, err := f.repo.ReplaceGadgets(ctx, uuid.New(), nil, nil, now); apperror.CodeOf(err) != domain.CodeNotFound {
		t.Fatalf("replace in a missing dashboard: %v", err)
	}
	// A failing insert leaves the previous gadgets untouched (atomic).
	f.exec(`INSERT INTO dashboard_gadgets (dashboard_id, type, title) VALUES ($1, 'text', 'previo')`, d.ID)
	if _, err := f.repo.ReplaceGadgets(ctx, d.ID, nil, []domain.Gadget{{Type: "x", Title: "ok", Config: json.RawMessage(`{}`)}, {Type: "x", Title: "bad", Config: json.RawMessage(`{}`), ColumnIndex: 9}}, now); err == nil {
		t.Fatal("invalid gadget accepted by the database")
	}
	if got, _ = f.repo.Get(ctx, v(f.owner), d.ID); len(got.Gadgets) != 1 || got.Gadgets[0].Title != "previo" {
		t.Fatalf("a failed replacement was not rolled back: %+v", got.Gadgets)
	}

	// Delete removes gadgets, shares and the stars that pointed at it.
	f.exec(`INSERT INTO favorites (user_id, entity_type, entity_id) VALUES ($1, 'dashboard', $2)`, f.owner, d.ID)
	if err := f.repo.Delete(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := f.db.Get(&left, `SELECT (SELECT count(*) FROM dashboard_gadgets WHERE dashboard_id = $1) + (SELECT count(*) FROM favorites WHERE entity_id = $1)`, d.ID); err != nil || left != 0 {
		t.Fatalf("leftovers: %d %v", left, err)
	}
	if err := f.repo.Delete(ctx, d.ID); apperror.CodeOf(err) != domain.CodeNotFound {
		t.Fatalf("second delete: %v", err)
	}
}

func TestDashboardListPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mine := f.create(f.owner, "Alfa", gadget(domain.GadgetText, 0, 0, `{}`))
	f.create(f.owner, "Beta 100%")
	f.create(f.owner, "Gamma")
	foreign := f.create(f.member, "Compartido por beto")
	f.create(f.member, "Privado de beto")
	if err := f.repo.ReplaceShares(ctx, foreign.ID, f.org, []domain.Share{{Type: domain.ShareUser, TargetID: f.owner, Permission: domain.PermissionView}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	f.exec(`INSERT INTO favorites (user_id, entity_type, entity_id) VALUES ($1, 'dashboard', $2)`, f.owner, mine.ID)

	list, total, err := f.repo.List(ctx, v(f.owner), port.ListQuery{Page: 1, PageSize: 10})
	if err != nil || total != 4 || len(list) != 4 {
		t.Fatalf("all: %d %d %v", total, len(list), err)
	}
	if !list[0].IsFavorite && list[0].ID == mine.ID {
		t.Fatal("favorite flag missing")
	}
	own, total, _ := f.repo.List(ctx, v(f.owner), port.ListQuery{Scope: "mine", Page: 1, PageSize: 10})
	if total != 3 || len(own) != 3 {
		t.Fatalf("mine: %d", total)
	}
	shared, total, _ := f.repo.List(ctx, v(f.owner), port.ListQuery{Scope: "shared", Page: 1, PageSize: 10})
	if total != 1 || shared[0].ID != foreign.ID || shared[0].OwnerName != "beto" {
		t.Fatalf("shared: %+v", shared)
	}
	if hits, total, _ := f.repo.List(ctx, v(f.owner), port.ListQuery{Query: "100%", Page: 1, PageSize: 10}); total != 1 || hits[0].Name != "Beta 100%" {
		t.Fatalf("search escaping: %d %+v", total, hits)
	}
	if hits, total, _ := f.repo.List(ctx, v(f.owner), port.ListQuery{Query: "%", Page: 1, PageSize: 10}); total != 1 || len(hits) != 1 {
		t.Fatalf("a bare wildcard matched %d dashboards", total)
	}
	page2, total, _ := f.repo.List(ctx, v(f.owner), port.ListQuery{Page: 2, PageSize: 3})
	if total != 4 || len(page2) != 1 {
		t.Fatalf("pagination: %d %d", total, len(page2))
	}
	// The org admin reads everything of the organization; the outsider nothing.
	if _, total, _ := f.repo.List(ctx, v(f.admin), port.ListQuery{Page: 1, PageSize: 50}); total != 5 {
		t.Fatalf("admin sees %d", total)
	}
	if _, total, _ := f.repo.List(ctx, v(f.other), port.ListQuery{Page: 1, PageSize: 50}); total != 0 {
		t.Fatalf("outsider sees %d", total)
	}
}

func TestDashboardReadStatementsDoNotGrowPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	measure := func() int32 {
		f.counter.Store(0)
		if _, _, err := f.repo.List(ctx, v(f.owner), port.ListQuery{Page: 1, PageSize: 100}); err != nil {
			t.Fatal(err)
		}
		return f.counter.Load()
	}
	f.create(f.owner, "D0", gadget(domain.GadgetText, 0, 0, `{}`))
	small := measure()
	for i := 0; i < 30; i++ {
		f.create(f.owner, "Carga "+uuid.NewString()[:8], gadget(domain.GadgetText, 0, 0, `{}`), gadget(domain.GadgetFavorites, 1, 0, `{}`))
	}
	if large := measure(); small != large || small != 2 {
		t.Fatalf("list statements: %d with 1 dashboard, %d with 31 (want 2)", small, large)
	}
	id := f.create(f.owner, "Detalle", gadget(domain.GadgetText, 0, 0, `{}`)).ID
	f.counter.Store(0)
	if _, err := f.repo.Get(ctx, v(f.owner), id); err != nil || f.counter.Load() != 2 {
		t.Fatalf("get statements = %d", f.counter.Load())
	}
}

func TestOrganizationForPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if id, err := f.repo.OrganizationFor(ctx, f.owner, uuid.Nil); err != nil || id != f.org {
		t.Fatalf("default org: %v %v", id, err)
	}
	if id, err := f.repo.OrganizationFor(ctx, f.other, f.otherOr); err != nil || id != f.otherOr {
		t.Fatalf("requested org: %v %v", id, err)
	}
	if _, err := f.repo.OrganizationFor(ctx, f.owner, f.otherOr); apperror.CodeOf(err) != domain.CodeNoOrganization {
		t.Fatalf("foreign org: %v", err)
	}
}

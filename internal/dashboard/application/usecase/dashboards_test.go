package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/dashboard/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

type memRepo struct {
	boards  map[uuid.UUID]*domain.Dashboard
	grants  map[uuid.UUID]map[uuid.UUID]domain.Permission // dashboard -> viewer -> permission
	admins  map[uuid.UUID]bool                            // viewer -> organization admin
	org     map[uuid.UUID]uuid.UUID                       // user -> organization
	shares  map[uuid.UUID][]domain.Share
	getHits int
}

func newMemRepo() *memRepo {
	return &memRepo{boards: map[uuid.UUID]*domain.Dashboard{}, grants: map[uuid.UUID]map[uuid.UUID]domain.Permission{}, admins: map[uuid.UUID]bool{},
		org: map[uuid.UUID]uuid.UUID{}, shares: map[uuid.UUID][]domain.Share{}}
}

func (m *memRepo) OrganizationFor(_ context.Context, u, _ uuid.UUID) (uuid.UUID, error) {
	if o, ok := m.org[u]; ok {
		return o, nil
	}
	return uuid.Nil, domain.ErrNoOrganization
}

func (m *memRepo) view(v port.Viewer, d *domain.Dashboard) (port.DashboardView, bool) {
	access := domain.ResolveAccess(d.OwnerID == v.UserID, m.grants[d.ID][v.UserID])
	if access == domain.AccessNone && m.admins[v.UserID] && m.org[v.UserID] == d.OrganizationID {
		access = domain.AccessView
	}
	if access == domain.AccessNone {
		return port.DashboardView{}, false
	}
	c := *d
	c.Gadgets = append([]domain.Gadget(nil), d.Gadgets...)
	return port.DashboardView{Dashboard: c, Access: access, Shared: len(m.shares[d.ID]) > 0}, true
}

func (m *memRepo) List(_ context.Context, v port.Viewer, q port.ListQuery) ([]port.DashboardView, int, error) {
	var out []port.DashboardView
	for _, d := range m.boards {
		if view, ok := m.view(v, d); ok {
			out = append(out, view)
		}
	}
	return out, len(out), nil
}

func (m *memRepo) Get(_ context.Context, v port.Viewer, id uuid.UUID) (*port.DashboardView, error) {
	m.getHits++
	d, ok := m.boards[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	view, ok := m.view(v, d)
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &view, nil
}

func (m *memRepo) FindDefault(_ context.Context, v port.Viewer) (*port.DashboardView, error) {
	for _, d := range m.boards {
		if d.OwnerID == v.UserID && d.IsDefault {
			view, _ := m.view(v, d)
			return &view, nil
		}
	}
	return nil, nil
}

func (m *memRepo) Create(_ context.Context, d *domain.Dashboard) error {
	for _, o := range m.boards {
		if o.OwnerID == d.OwnerID && strings.EqualFold(o.Name, d.Name) {
			return domain.ErrNameTaken
		}
	}
	d.ID = uuid.New()
	for i := range d.Gadgets {
		d.Gadgets[i].ID = uuid.New()
	}
	c := *d
	m.boards[d.ID] = &c
	return nil
}

func (m *memRepo) UpdateFields(_ context.Context, id uuid.UUID, p port.FieldsPatch, at time.Time) error {
	d := m.boards[id]
	if p.Name != nil {
		d.Name = *p.Name
	}
	if p.Description != nil {
		d.Description = *p.Description
	}
	if p.Layout != nil {
		d.Layout = *p.Layout
	}
	if p.IsDefault != nil {
		d.IsDefault = *p.IsDefault
	}
	d.UpdatedAt = at
	return nil
}

func (m *memRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.boards, id)
	return nil
}

func (m *memRepo) ReplaceGadgets(_ context.Context, id uuid.UUID, layout *domain.Layout, g []domain.Gadget, at time.Time) ([]domain.Gadget, error) {
	d := m.boards[id]
	if layout != nil {
		d.Layout = *layout
	}
	out := make([]domain.Gadget, len(g))
	for i, x := range g {
		if x.ID == uuid.Nil {
			x.ID = uuid.New()
		}
		out[i] = x
	}
	d.Gadgets, d.UpdatedAt = out, at
	return out, nil
}

func (m *memRepo) Shares(_ context.Context, id uuid.UUID) ([]port.ShareView, error) {
	var out []port.ShareView
	for _, s := range m.shares[id] {
		out = append(out, port.ShareView{Share: s})
	}
	return out, nil
}

func (m *memRepo) ReplaceShares(_ context.Context, id, _ uuid.UUID, shares []domain.Share, _ time.Time) error {
	m.shares[id] = shares
	m.grants[id] = map[uuid.UUID]domain.Permission{}
	for _, s := range shares {
		if s.Type == domain.ShareUser {
			m.grants[id][s.TargetID] = s.Permission
		}
	}
	return nil
}

type fakeData struct{ calls []string }

func (f *fakeData) Statistics(_ context.Context, v port.Viewer, c domain.StatsConfig) (port.StatsData, error) {
	f.calls = append(f.calls, "stats:"+c.GroupBy+":"+v.UserID.String())
	return port.StatsData{GroupBy: c.GroupBy, Groups: []port.StatGroup{{Key: "a", Label: "A", Count: 1}}}, nil
}
func (f *fakeData) TwoDimensional(context.Context, port.Viewer, domain.TwoDimensionalConfig) (port.MatrixData, error) {
	f.calls = append(f.calls, "matrix")
	return port.MatrixData{}, nil
}
func (f *fakeData) CreatedVsResolved(_ context.Context, _ port.Viewer, c domain.CreatedVsResolvedConfig, _ time.Time) (port.CreatedVsResolvedData, error) {
	f.calls = append(f.calls, "cvr")
	return port.CreatedVsResolvedData{Days: c.Days}, nil
}
func (f *fakeData) Burndown(context.Context, port.Viewer, domain.BurndownConfig, time.Time) (port.BurndownData, error) {
	f.calls = append(f.calls, "burndown")
	return port.BurndownData{}, nil
}
func (f *fakeData) Activity(context.Context, port.Viewer, domain.ActivityConfig) ([]port.ActivityItem, error) {
	f.calls = append(f.calls, "activity")
	return nil, nil
}
func (f *fakeData) RecentProjects(context.Context, port.Viewer, int) ([]port.ProjectItem, error) {
	f.calls = append(f.calls, "recent")
	return nil, nil
}
func (f *fakeData) Favorites(context.Context, port.Viewer, domain.FavoritesConfig) ([]port.FavoriteItem, error) {
	f.calls = append(f.calls, "favorites")
	return nil, nil
}

type fakeIssues struct {
	queries []string
	filters []uuid.UUID
	err     error
}

func (f *fakeIssues) RunQuery(_ context.Context, v port.Viewer, q string, limit int) (port.IssueList, error) {
	f.queries = append(f.queries, q)
	return port.IssueList{Total: 1, Query: q, Items: []port.IssueRow{{Key: "ABC-1", Summary: v.UserID.String()}}}, f.err
}
func (f *fakeIssues) RunFilter(_ context.Context, _ port.Viewer, id uuid.UUID, _ int) (port.IssueList, error) {
	f.filters = append(f.filters, id)
	return port.IssueList{}, f.err
}

type fakeUpcoming struct{ from, to time.Time }

func (f *fakeUpcoming) Upcoming(_ context.Context, _ port.Viewer, from, to time.Time, _ int) ([]port.UpcomingItem, error) {
	f.from, f.to = from, to
	return []port.UpcomingItem{{Kind: "event", Title: "x"}}, nil
}

type env struct {
	svc             *DashboardsService
	repo            *memRepo
	data            *fakeData
	issues          *fakeIssues
	up              *fakeUpcoming
	clk             *clock.FakeClock
	org             uuid.UUID
	owner, ed, vw   uuid.UUID
	admin, stranger uuid.UUID
}

func newEnv() *env {
	e := &env{repo: newMemRepo(), data: &fakeData{}, issues: &fakeIssues{}, up: &fakeUpcoming{}, org: uuid.New(),
		owner: uuid.New(), ed: uuid.New(), vw: uuid.New(), admin: uuid.New(), stranger: uuid.New(),
		clk: &clock.FakeClock{T: time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)}}
	for _, u := range []uuid.UUID{e.owner, e.ed, e.vw, e.admin} {
		e.repo.org[u] = e.org
	}
	e.repo.org[e.stranger] = uuid.New()
	e.repo.admins[e.admin] = true
	e.svc = NewDashboardsService(DashboardsDeps{Repository: e.repo, Data: e.data, Issues: e.issues, Upcoming: e.up, Clock: e.clk})
	return e
}

func v(id uuid.UUID) port.Viewer { return port.Viewer{UserID: id} }

func wantErr(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	if err == nil || apperror.KindOf(err) != kind || (code != "" && apperror.CodeOf(err) != code) {
		t.Fatalf("want %s/%s, got %v (%s/%s)", kind, code, err, apperror.KindOf(err), apperror.CodeOf(err))
	}
}

// shared creates a dashboard of the owner shared with an editor and a viewer.
func (e *env) shared(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	d, err := e.svc.Create(ctx, v(e.owner), port.CreateInput{Name: "Equipo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetShares(ctx, v(e.owner), d.ID, []domain.Share{
		{Type: domain.ShareUser, TargetID: e.ed, Permission: domain.PermissionEdit}, {Type: domain.ShareUser, TargetID: e.vw, Permission: domain.PermissionView},
	}); err != nil {
		t.Fatal(err)
	}
	return d.ID
}

func TestPermissionMatrix(t *testing.T) {
	ctx := context.Background()
	name := "Nuevo"
	gadgets := []port.GadgetInput{{Type: domain.GadgetText}}

	type action struct {
		name string
		run  func(e *env, id, u uuid.UUID) error
	}
	actions := []action{
		{"get", func(e *env, id, u uuid.UUID) error { _, err := e.svc.Get(ctx, v(u), id); return err }},
		{"update", func(e *env, id, u uuid.UUID) error {
			_, err := e.svc.Update(ctx, v(u), id, port.UpdateInput{Name: &name})
			return err
		}},
		{"gadgets", func(e *env, id, u uuid.UUID) error {
			_, err := e.svc.ReplaceGadgets(ctx, v(u), id, nil, gadgets)
			return err
		}},
		{"shares", func(e *env, id, u uuid.UUID) error { _, err := e.svc.SetShares(ctx, v(u), id, nil); return err }},
		{"make default", func(e *env, id, u uuid.UUID) error {
			t := true
			_, err := e.svc.Update(ctx, v(u), id, port.UpdateInput{IsDefault: &t})
			return err
		}},
		{"delete", func(e *env, id, u uuid.UUID) error { return e.svc.Delete(ctx, v(u), id) }},
	}
	// expectation per user: "" allowed, "403", "404"
	want := map[string]map[string]string{
		"get":          {"owner": "", "editor": "", "viewer": "", "org admin": "", "stranger": "404"},
		"update":       {"owner": "", "editor": "", "viewer": "403", "org admin": "403", "stranger": "404"},
		"gadgets":      {"owner": "", "editor": "", "viewer": "403", "org admin": "403", "stranger": "404"},
		"shares":       {"owner": "", "editor": "403", "viewer": "403", "org admin": "403", "stranger": "404"},
		"make default": {"owner": "", "editor": "403", "viewer": "403", "org admin": "403", "stranger": "404"},
		"delete":       {"owner": "", "editor": "403", "viewer": "403", "org admin": "403", "stranger": "404"},
	}
	users := []struct{ name string }{{"viewer"}, {"org admin"}, {"stranger"}, {"editor"}, {"owner"}}
	for _, a := range actions {
		for _, u := range users {
			e := newEnv()
			id := e.shared(t)
			err := a.run(e, id, e.userFor(u.name))
			switch want[a.name][u.name] {
			case "":
				if err != nil {
					t.Errorf("%s by %s: %v", a.name, u.name, err)
				}
			case "403":
				if apperror.KindOf(err) != apperror.KindForbidden || apperror.CodeOf(err) != domain.CodeForbidden {
					t.Errorf("%s by %s: %v, want 403", a.name, u.name, err)
				}
			case "404":
				if apperror.KindOf(err) != apperror.KindNotFound || apperror.CodeOf(err) != domain.CodeNotFound {
					t.Errorf("%s by %s: %v, want 404", a.name, u.name, err)
				}
			}
		}
	}
}

func (e *env) userFor(name string) uuid.UUID {
	switch name {
	case "viewer":
		return e.vw
	case "org admin":
		return e.admin
	case "stranger":
		return e.stranger
	case "editor":
		return e.ed
	}
	return e.owner
}

func TestOnlyTheOwnerSeesShares(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	id := e.shared(t)
	owner, _ := e.svc.Get(ctx, v(e.owner), id)
	editor, _ := e.svc.Get(ctx, v(e.ed), id)
	if len(owner.Shares) != 2 || len(editor.Shares) != 0 {
		t.Fatalf("shares: owner %d editor %d", len(owner.Shares), len(editor.Shares))
	}
}

func TestCreateUpdateValidation(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	_, err := e.svc.Create(ctx, v(e.owner), port.CreateInput{Name: " "})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidDashboard)
	_, err = e.svc.Create(ctx, v(e.owner), port.CreateInput{Name: "x", Layout: "9"})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidDashboard)
	_, err = e.svc.Create(ctx, v(e.owner), port.CreateInput{Name: "x", Description: strings.Repeat("d", 501)})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidDashboard)
	_, err = e.svc.Create(ctx, v(uuid.New()), port.CreateInput{Name: "x"})
	wantErr(t, err, apperror.KindValidation, domain.CodeNoOrganization)
	d, err := e.svc.Create(ctx, v(e.owner), port.CreateInput{Name: "  Mi panel ", Layout: "3"})
	if err != nil || d.Name != "Mi panel" || d.Layout != domain.Layout3 || d.Access != domain.AccessOwner {
		t.Fatalf("create: %+v %v", d, err)
	}
	_, err = e.svc.Create(ctx, v(e.owner), port.CreateInput{Name: "mi PANEL"})
	wantErr(t, err, apperror.KindConflict, domain.CodeNameTaken)

	// The layout cannot drop a column that still has gadgets.
	if _, err := e.svc.ReplaceGadgets(ctx, v(e.owner), d.ID, nil, []port.GadgetInput{{Type: domain.GadgetText, ColumnIndex: 2}}); err != nil {
		t.Fatal(err)
	}
	two := "2"
	_, err = e.svc.Update(ctx, v(e.owner), d.ID, port.UpdateInput{Layout: &two})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidDashboard)
	bad, long := "7", strings.Repeat("n", 121)
	_, err = e.svc.Update(ctx, v(e.owner), d.ID, port.UpdateInput{Layout: &bad})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidDashboard)
	_, err = e.svc.Update(ctx, v(e.owner), d.ID, port.UpdateInput{Name: &long})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidDashboard)
	desc := strings.Repeat("d", 501)
	_, err = e.svc.Update(ctx, v(e.owner), d.ID, port.UpdateInput{Description: &desc})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidDashboard)
	// Replacing the layout together with the gadgets works.
	if _, err := e.svc.ReplaceGadgets(ctx, v(e.owner), d.ID, &two, []port.GadgetInput{{Type: domain.GadgetText, ColumnIndex: 1}}); err != nil {
		t.Fatal(err)
	}
	// Invalid gadgets and layout in a replacement.
	_, err = e.svc.ReplaceGadgets(ctx, v(e.owner), d.ID, &bad, nil)
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidDashboard)
	_, err = e.svc.ReplaceGadgets(ctx, v(e.owner), d.ID, nil, []port.GadgetInput{{Type: "weather"}})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidGadget)
	ghost := uuid.New()
	_, err = e.svc.ReplaceGadgets(ctx, v(e.owner), d.ID, nil, []port.GadgetInput{{ID: &ghost, Type: domain.GadgetText}})
	wantErr(t, err, apperror.KindValidation, domain.CodeGadgetNotFound)
}

func TestListScope(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	e.shared(t)
	if _, _, err := e.svc.List(ctx, v(e.owner), port.ListQuery{Scope: "nope"}); err == nil {
		t.Fatal("bad scope accepted")
	}
	list, total, err := e.svc.List(ctx, v(e.vw), port.ListQuery{PageSize: 1000})
	if err != nil || total != 1 || len(list) != 1 || list[0].Access != domain.AccessView {
		t.Fatalf("list: %+v %v", list, err)
	}
	if list, _, _ := e.svc.List(ctx, v(e.stranger), port.ListQuery{}); len(list) != 0 {
		t.Fatalf("stranger sees %d dashboards", len(list))
	}
}

func TestDuplicate(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	id := e.shared(t)
	if _, err := e.svc.ReplaceGadgets(ctx, v(e.owner), id, nil, []port.GadgetInput{{Type: domain.GadgetText, Config: json.RawMessage(`{"markdown":"hola"}`)}, {Type: domain.GadgetFavorites, ColumnIndex: 1}}); err != nil {
		t.Fatal(err)
	}
	// A viewer can copy it; the copy belongs to them, has fresh ids and no shares.
	copy1, err := e.svc.Duplicate(ctx, v(e.vw), id, "")
	if err != nil || copy1.Name != "Copia de Equipo" || copy1.OwnerID != e.vw || len(copy1.Gadgets) != 2 || copy1.Access != domain.AccessOwner || len(copy1.Shares) != 0 {
		t.Fatalf("duplicate: %+v %v", copy1, err)
	}
	original, _ := e.svc.Get(ctx, v(e.owner), id)
	if copy1.Gadgets[0].ID == original.Gadgets[0].ID {
		t.Fatal("copy reuses gadget ids")
	}
	// The same name again gets a numeric suffix.
	copy2, err := e.svc.Duplicate(ctx, v(e.vw), id, "")
	if err != nil || copy2.Name != "Copia de Equipo (2)" {
		t.Fatalf("second copy: %v %v", copy2, err)
	}
	named, err := e.svc.Duplicate(ctx, v(e.vw), id, "  Mío ")
	if err != nil || named.Name != "Mío" {
		t.Fatalf("named copy: %v %v", named, err)
	}
	_, err = e.svc.Duplicate(ctx, v(e.stranger), id, "")
	wantErr(t, err, apperror.KindNotFound, domain.CodeNotFound)
}

func TestDefaultDashboard(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	first, err := e.svc.Default(ctx, v(e.owner))
	if err != nil || !first.IsDefault || first.Name != "Mi panel" || len(first.Gadgets) != 6 {
		t.Fatalf("first use: %+v %v", first, err)
	}
	for _, g := range first.Gadgets {
		cfg, err := domain.NormalizeConfig(g.Type, g.Config)
		if err != nil || string(cfg) != string(g.Config) {
			t.Errorf("default gadget %s is not canonical: %s vs %s (%v)", g.Type, g.Config, cfg, err)
		}
		if g.ColumnIndex > 1 {
			t.Errorf("default gadget outside the two columns")
		}
	}
	again, err := e.svc.Default(ctx, v(e.owner))
	if err != nil || again.ID != first.ID || len(e.repo.boards) != 1 {
		t.Fatalf("second use created another: %v %v", again, err)
	}
	// A user without organization cannot get one.
	_, err = e.svc.Default(ctx, v(uuid.New()))
	wantErr(t, err, apperror.KindValidation, domain.CodeNoOrganization)
	// The name is taken by a non-default dashboard.
	other := newEnv()
	if _, err := other.svc.Create(ctx, v(other.owner), port.CreateInput{Name: "Mi panel"}); err != nil {
		t.Fatal(err)
	}
	d, err := other.svc.Default(ctx, v(other.owner))
	if err != nil || d.Name != "Mi panel principal" || !d.IsDefault {
		t.Fatalf("name clash: %+v %v", d, err)
	}
}

func TestGadgetDataPerType(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	d, _ := e.svc.Create(ctx, v(e.owner), port.CreateInput{Name: "Datos"})
	fid := uuid.New()
	inputs := []port.GadgetInput{
		{Type: domain.GadgetAssignedToMe}, {Type: domain.GadgetCreatedByMe, Config: json.RawMessage(`{"include_done":true}`)},
		{Type: domain.GadgetFilterResults, Config: json.RawMessage(`{"query":"project = ABC"}`)},
		{Type: domain.GadgetFilterResults, Config: json.RawMessage(`{"filter_id":"` + fid.String() + `","columns":["key"]}`)},
		{Type: domain.GadgetIssueStatistics}, {Type: domain.GadgetPie, Config: json.RawMessage(`{"group_by":"assignee"}`)},
		{Type: domain.GadgetTwoDimensional}, {Type: domain.GadgetCreatedVsResolved}, {Type: domain.GadgetSprintBurndown},
		{Type: domain.GadgetActivityStream}, {Type: domain.GadgetRecentProjects}, {Type: domain.GadgetFavorites},
		{Type: domain.GadgetText, Config: json.RawMessage(`{"markdown":"**hola**"}`)}, {Type: domain.GadgetCalendarUpcoming, Config: json.RawMessage(`{"days":3}`)},
	}
	full, err := e.svc.ReplaceGadgets(ctx, v(e.owner), d.ID, nil, inputs)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{}
	for i, g := range full.Gadgets {
		out, err := e.svc.GadgetData(ctx, v(e.owner), d.ID, g.ID)
		if err != nil {
			t.Fatalf("%s: %v", g.Type, err)
		}
		if out.Type != g.Type || out.GadgetID != g.ID || !out.GeneratedAt.Equal(e.clk.T) || out.Data == nil {
			t.Fatalf("%s: %+v", g.Type, out)
		}
		data[g.Type+string(rune('a'+i))] = out.Data
	}
	wantQueries := []string{
		"assignee = currentUser() AND statusCategory != done ORDER BY updated DESC",
		"reporter = currentUser() ORDER BY updated DESC",
		"project = ABC",
	}
	if strings.Join(e.issues.queries, "|") != strings.Join(wantQueries, "|") || len(e.issues.filters) != 1 || e.issues.filters[0] != fid {
		t.Fatalf("queries = %v filters = %v", e.issues.queries, e.issues.filters)
	}
	if got := strings.Join(e.data.calls[:2], ","); !strings.HasPrefix(got, "stats:status:") || !strings.Contains(e.data.calls[1], "stats:assignee:") {
		t.Fatalf("stats calls = %v", e.data.calls)
	}
	if e.up.to.Sub(e.up.from) != 72*time.Hour || !e.up.from.Equal(e.clk.T) {
		t.Fatalf("upcoming window %v - %v", e.up.from, e.up.to)
	}
	list := data["assigned_to_mea"].(port.IssueListData)
	if list.Total != 1 || len(list.Columns) == 0 || list.Items[0].Summary != e.owner.String() {
		t.Fatalf("assigned data: %+v", list)
	}
	if text := data["texta"+""]; text != nil {
		t.Log("unexpected key")
	}
	// Unknown ids.
	_, err = e.svc.GadgetData(ctx, v(e.owner), d.ID, uuid.New())
	wantErr(t, err, apperror.KindNotFound, domain.CodeGadgetNotFound)
	_, err = e.svc.GadgetData(ctx, v(e.stranger), d.ID, full.Gadgets[0].ID)
	wantErr(t, err, apperror.KindNotFound, domain.CodeNotFound)
}

func TestGadgetDataIsComputedPerViewerAndCached(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	id := e.shared(t)
	full, _ := e.svc.ReplaceGadgets(ctx, v(e.owner), id, nil, []port.GadgetInput{{Type: domain.GadgetAssignedToMe}})
	gid := full.Gadgets[0].ID

	a, err := e.svc.GadgetData(ctx, v(e.owner), id, gid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.GadgetData(ctx, v(e.owner), id, gid); err != nil {
		t.Fatal(err)
	}
	if len(e.issues.queries) != 1 {
		t.Fatalf("second call within the TTL ran the query again: %d", len(e.issues.queries))
	}
	// Another viewer of the same shared dashboard gets their own data, never the cached one of the owner.
	b, err := e.svc.GadgetData(ctx, v(e.vw), id, gid)
	if err != nil || len(e.issues.queries) != 2 {
		t.Fatalf("viewer: %v, queries %d", err, len(e.issues.queries))
	}
	if a.Data.(port.IssueListData).Items[0].Summary == b.Data.(port.IssueListData).Items[0].Summary {
		t.Fatal("the viewer received the owner's data")
	}
	// The platform administrator's view is cached apart from the regular one.
	admin := port.Viewer{UserID: e.owner, PlatformAdmin: true}
	if _, err := e.svc.GadgetData(ctx, admin, id, gid); err != nil || len(e.issues.queries) != 3 {
		t.Fatalf("admin view reused the regular cache: %d", len(e.issues.queries))
	}
	// After the TTL it is computed again.
	e.clk.Advance(DataCacheTTL + time.Second)
	if _, err := e.svc.GadgetData(ctx, v(e.owner), id, gid); err != nil || len(e.issues.queries) != 4 {
		t.Fatalf("expired entry not recomputed: %d", len(e.issues.queries))
	}
	// Changing the dashboard invalidates the entries.
	name := "Otro"
	if _, err := e.svc.Update(ctx, v(e.owner), id, port.UpdateInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(time.Second)
	if _, err := e.svc.GadgetData(ctx, v(e.owner), id, gid); err != nil || len(e.issues.queries) != 5 {
		t.Fatalf("update did not invalidate the cache: %d", len(e.issues.queries))
	}
}

func TestGadgetDataSourceFailures(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	d, _ := e.svc.Create(ctx, v(e.owner), port.CreateInput{Name: "F"})
	full, _ := e.svc.ReplaceGadgets(ctx, v(e.owner), d.ID, nil, []port.GadgetInput{{Type: domain.GadgetAssignedToMe}, {Type: domain.GadgetCalendarUpcoming}, {Type: domain.GadgetFilterResults, Config: json.RawMessage(`{"query":"x"}`)}})
	boom := errors.New("boom")
	e.issues.err = boom
	if _, err := e.svc.GadgetData(ctx, v(e.owner), d.ID, full.Gadgets[0].ID); !errors.Is(err, boom) {
		t.Fatalf("runner error lost: %v", err)
	}
	// Without the collaborators the gadgets report the source as unavailable.
	bare := NewDashboardsService(DashboardsDeps{Repository: e.repo, Data: e.data, Clock: e.clk})
	for _, g := range full.Gadgets {
		_, err := bare.GadgetData(ctx, v(e.owner), d.ID, g.ID)
		wantErr(t, err, apperror.KindInternal, domain.CodeSourceFailed)
	}
	// A stored configuration that no longer decodes is an internal error, not a panic.
	e.repo.boards[d.ID].Gadgets[0].Config = json.RawMessage(`[1]`)
	e.clk.Advance(time.Hour)
	if _, err := e.svc.GadgetData(ctx, v(e.owner), d.ID, full.Gadgets[0].ID); apperror.KindOf(err) != apperror.KindInternal {
		t.Fatalf("corrupt config: %v", err)
	}
}

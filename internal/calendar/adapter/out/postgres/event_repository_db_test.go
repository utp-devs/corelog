package postgres

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/calendar/domain"
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

type calFixture struct {
	t       *testing.T
	db      *sqlx.DB
	repo    *CalendarRepository
	counter *atomic.Int32
	org     uuid.UUID
	owner   uuid.UUID // org member, lead of the open project
	member  uuid.UUID // org member without project role
	admin   uuid.UUID // org admin
	other   uuid.UUID // member of another organization
	open    uuid.UUID // open project
	secret  uuid.UUID // private project, only the owner is a member
}

func (f *calFixture) exec(q string, args ...any) {
	f.t.Helper()
	if _, err := f.db.Exec(q, args...); err != nil {
		f.t.Fatalf("%v\n%s", err, q)
	}
}

func (f *calFixture) user(name string) uuid.UUID {
	id := uuid.New()
	f.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, $2, $2, $3, 'x', 'user')`, id, name, name+"@example.test")
	return id
}

func newCalFixture(t *testing.T) *calFixture {
	t.Helper()
	db := testdb.New(t)
	counter := &atomic.Int32{}
	repo := NewCalendarRepository(db)
	repo.db = countingQuerier{DB: db, n: counter}
	f := &calFixture{t: t, db: db, repo: repo, counter: counter, org: uuid.MustParse(defaultOrg), open: uuid.New(), secret: uuid.New()}
	f.owner, f.member, f.admin, f.other = f.user("ana"), f.user("beto"), f.user("carla"), f.user("dani")
	f.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member'), ($1, $3, 'member'), ($1, $4, 'admin')`, f.org, f.owner, f.member, f.admin)
	otherOrg := uuid.New()
	f.exec(`INSERT INTO organizations (id, name, slug) VALUES ($1, 'Otra', 'otra')`, otherOrg)
	f.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'owner')`, otherOrg, f.other)
	f.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $3, 'OPN', 'Abierto', 'kanban', 'scrum', $4, 'open'), ($2, $3, 'SEC', 'Secreto', 'kanban', 'scrum', $4, 'private')`, f.open, f.secret, f.org, f.owner)
	f.exec(`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $3, 'lead'), ($2, $3, 'lead')`, f.open, f.secret, f.owner)
	return f
}

func v(id uuid.UUID) port.Viewer { return port.Viewer{UserID: id, GlobalRole: "user"} }

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func (f *calFixture) event(owner uuid.UUID, title string, vis domain.Visibility, project *uuid.UUID, start time.Time, attendees ...uuid.UUID) *domain.Event {
	f.t.Helper()
	e := &domain.Event{OwnerID: owner, OrganizationID: f.org, ProjectID: project, Title: title, StartsAt: start, EndsAt: start.Add(time.Hour), Timezone: "UTC",
		Visibility: vis, Color: "blue", CreatedAt: start, UpdatedAt: start}
	for _, a := range attendees {
		e.Attendees = append(e.Attendees, domain.Attendee{UserID: a, Response: domain.ResponseNeedsAction})
	}
	if err := e.Normalize(); err != nil {
		f.t.Fatal(err)
	}
	if err := f.repo.Create(context.Background(), e); err != nil {
		f.t.Fatal(err)
	}
	return e
}

func titles(events []domain.Event) map[string]bool {
	out := map[string]bool{}
	for _, e := range events {
		out[e.Title] = true
	}
	return out
}

func TestEventVisibilityMatrixPostgres(t *testing.T) {
	f := newCalFixture(t)
	ctx := context.Background()
	start := time.Date(2026, 12, 7, 9, 0, 0, 0, time.UTC)
	priv := f.event(f.owner, "privado", domain.VisibilityPrivate, nil, start)
	guest := f.event(f.owner, "con invitado", domain.VisibilityPrivate, nil, start, f.member)
	org := f.event(f.admin, "organizacion", domain.VisibilityOrganization, nil, start)
	openProject := f.event(f.owner, "proyecto abierto", domain.VisibilityProject, &f.open, start)
	secretProject := f.event(f.owner, "proyecto secreto", domain.VisibilityProject, &f.secret, start)

	cases := []struct {
		who  string
		id   uuid.UUID
		want []string
	}{
		{"owner", f.owner, []string{"privado", "con invitado", "organizacion", "proyecto abierto", "proyecto secreto"}},
		{"guest member", f.member, []string{"con invitado", "organizacion", "proyecto abierto"}},
		{"org admin", f.admin, []string{"organizacion", "proyecto abierto", "proyecto secreto"}},
		{"other organization", f.other, nil},
	}
	from, to := day(2026, 12, 1), day(2026, 12, 31)
	for _, c := range cases {
		t.Run(c.who, func(t *testing.T) {
			list, err := f.repo.ListVisible(ctx, v(c.id), from, to, nil)
			if err != nil {
				t.Fatal(err)
			}
			got := titles(list)
			if len(got) != len(c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
			for _, w := range c.want {
				if !got[w] {
					t.Fatalf("missing %q in %v", w, got)
				}
			}
			// FindVisible agrees with the list for every event.
			for _, e := range []*domain.Event{priv, guest, org, openProject, secretProject} {
				_, err := f.repo.FindVisible(ctx, v(c.id), e.ID)
				if got[e.Title] != (err == nil) {
					t.Fatalf("%s: list says %v, FindVisible err = %v", e.Title, got[e.Title], err)
				}
				if err != nil && apperror.CodeOf(err) != domain.CodeEventNotFound {
					t.Fatalf("hidden event error: %v", err)
				}
			}
		})
	}
	// The platform administrator sees everything but private events of others.
	admin := port.Viewer{UserID: f.other, GlobalRole: "admin"}
	list, _ := f.repo.ListVisible(ctx, admin, from, to, nil)
	if got := titles(list); len(got) != 3 || got["privado"] || got["con invitado"] {
		t.Fatalf("platform admin sees %v", got)
	}
}

func TestEventLifecyclePostgres(t *testing.T) {
	f := newCalFixture(t)
	ctx := context.Background()
	rule := "FREQ=WEEKLY;COUNT=3"
	e := f.event(f.owner, "semanal", domain.VisibilityOrganization, nil, time.Date(2026, 12, 7, 9, 0, 0, 0, time.UTC), f.member, f.admin)
	e.RecurrenceRule = &rule

	// The master is returned for a later window; a window before it, or a non-recurring one that ended, is not.
	if err := f.repo.Update(ctx, e, false); err != nil {
		t.Fatal(err)
	}
	if list, _ := f.repo.ListVisible(ctx, v(f.owner), day(2027, 1, 1), day(2027, 2, 1), nil); len(list) != 1 {
		t.Fatalf("recurring master missing: %v", list)
	}
	if list, _ := f.repo.ListVisible(ctx, v(f.owner), day(2026, 11, 1), day(2026, 12, 1), nil); len(list) != 0 {
		t.Fatalf("event before its start: %v", list)
	}
	f.event(f.owner, "pasado", domain.VisibilityPrivate, nil, time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC))
	if list, _ := f.repo.ListVisible(ctx, v(f.owner), day(2026, 7, 1), day(2026, 8, 1), nil); len(list) != 0 {
		t.Fatalf("ended single event returned: %v", list)
	}

	// Exceptions round trip, including an override with moved times.
	title, moved := "movida", time.Date(2026, 12, 14, 16, 0, 0, 0, time.UTC)
	e.Exceptions = []domain.Exception{
		{OriginalStart: time.Date(2026, 12, 7, 9, 0, 0, 0, time.UTC), Cancelled: true},
		{OriginalStart: time.Date(2026, 12, 14, 9, 0, 0, 0, time.UTC), Override: &domain.Override{Title: &title, StartsAt: &moved}},
	}
	e.UpdatedAt = e.UpdatedAt.Add(time.Minute)
	if err := f.repo.Update(ctx, e, false); err != nil {
		t.Fatal(err)
	}
	got, err := f.repo.FindVisible(ctx, v(f.owner), e.ID)
	if err != nil || len(got.Exceptions) != 2 || !got.Exceptions[0].Cancelled || *got.Exceptions[1].Override.Title != title || !got.Exceptions[1].Override.StartsAt.Equal(moved) {
		t.Fatalf("exceptions: %+v %v", got, err)
	}
	if len(got.Attendees) != 2 || got.Attendees[0].Name != "beto" {
		t.Fatalf("attendees: %+v", got.Attendees)
	}

	// Attendees are replaced only when asked.
	e.Attendees = []domain.Attendee{{UserID: f.member, Response: domain.ResponseNeedsAction}}
	if err := f.repo.Update(ctx, e, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.repo.FindVisible(ctx, v(f.owner), e.ID); len(got.Attendees) != 1 {
		t.Fatalf("attendees after replace: %+v", got.Attendees)
	}

	// RSVP.
	if ok, err := f.repo.SetResponse(ctx, e.ID, f.member, domain.ResponseAccepted); err != nil || !ok {
		t.Fatalf("rsvp: %v %v", ok, err)
	}
	if ok, _ := f.repo.SetResponse(ctx, e.ID, f.admin, domain.ResponseAccepted); ok {
		t.Fatal("a user that was removed from the guests answered")
	}
	if got, _ := f.repo.FindVisible(ctx, v(f.member), e.ID); got.Attendees[0].Response != domain.ResponseAccepted {
		t.Fatalf("response not stored: %+v", got.Attendees)
	}

	// Soft delete.
	if err := f.repo.Delete(ctx, e.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.FindVisible(ctx, v(f.owner), e.ID); apperror.CodeOf(err) != domain.CodeEventNotFound {
		t.Fatalf("deleted event found: %v", err)
	}
	if err := f.repo.Delete(ctx, e.ID, time.Now()); apperror.CodeOf(err) != domain.CodeEventNotFound {
		t.Fatalf("second delete: %v", err)
	}
	if err := f.repo.Update(ctx, e, false); apperror.CodeOf(err) != domain.CodeEventNotFound {
		t.Fatalf("update of a deleted event: %v", err)
	}
}

func TestEventProjectFilterAndReferencesPostgres(t *testing.T) {
	f := newCalFixture(t)
	ctx := context.Background()
	start := time.Date(2026, 12, 7, 9, 0, 0, 0, time.UTC)
	f.event(f.owner, "libre", domain.VisibilityOrganization, nil, start)
	f.event(f.owner, "abierto", domain.VisibilityProject, &f.open, start)
	f.event(f.owner, "secreto", domain.VisibilityPrivate, &f.secret, start)
	list, err := f.repo.ListVisible(ctx, v(f.owner), day(2026, 12, 1), day(2026, 12, 31), []uuid.UUID{f.open})
	if err != nil || len(list) != 2 || titles(list)["secreto"] {
		t.Fatalf("project filter: %v %v", titles(list), err)
	}
	bad := &domain.Event{OwnerID: f.owner, OrganizationID: f.org, Title: "x", StartsAt: start, EndsAt: start, Timezone: "UTC", Visibility: domain.VisibilityPrivate, Color: "blue",
		Attendees: []domain.Attendee{{UserID: uuid.New(), Response: domain.ResponseNeedsAction}}}
	if err := f.repo.Create(ctx, bad); apperror.CodeOf(err) != domain.CodeInvalidEvent {
		t.Fatalf("unknown attendee: %v", err)
	}
}

func TestEventOrganizationLookupsPostgres(t *testing.T) {
	f := newCalFixture(t)
	ctx := context.Background()
	if org, err := f.repo.OrganizationFor(ctx, f.owner, uuid.Nil); err != nil || org != f.org {
		t.Fatalf("OrganizationFor: %v %v", org, err)
	}
	if _, err := f.repo.OrganizationFor(ctx, f.owner, uuid.New()); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("foreign organization: %v", err)
	}
	if _, err := f.repo.OrganizationFor(ctx, uuid.New(), uuid.Nil); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("user without organization: %v", err)
	}
	if org, err := f.repo.ProjectOrganization(ctx, f.open); err != nil || org != f.org {
		t.Fatalf("ProjectOrganization: %v %v", org, err)
	}
	if _, err := f.repo.ProjectOrganization(ctx, uuid.New()); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("missing project: %v", err)
	}
	if role, _ := f.repo.OrgRole(ctx, f.admin, f.org); role != "admin" {
		t.Fatalf("role = %q", role)
	}
	if role, _ := f.repo.OrgRole(ctx, f.other, f.org); role != "" {
		t.Fatalf("outsider role = %q", role)
	}
	members, err := f.repo.MembersAmong(ctx, f.org, []uuid.UUID{f.owner, f.other, f.admin})
	if err != nil || len(members) != 2 {
		t.Fatalf("MembersAmong: %v %v", members, err)
	}
}

func TestDerivedDatesPostgres(t *testing.T) {
	f := newCalFixture(t)
	ctx := context.Background()
	typ := `(SELECT it.id FROM issue_types it WHERE it.organization_id = $2 AND it.is_default)`
	prio := `(SELECT pr.id FROM priorities pr WHERE pr.organization_id = $2 AND pr.is_default)`
	issue := func(project uuid.UUID, summary string, start, due any) {
		f.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, key_number, issue_type_id, priority_id, start_date, due_date)
			SELECT gen_random_uuid(), $1, $3, '', 'open', 'medium', $4, (SELECT COALESCE(max(key_number), 0) + 1 FROM issues WHERE project_id = $1), `+typ+`, `+prio+`, $5, $6`,
			project, f.org, summary, f.owner, start, due)
	}
	issue(f.open, "vence", nil, "2026-12-10")
	issue(f.open, "empieza", "2026-12-12", "2026-12-20")
	issue(f.open, "fuera", nil, "2027-03-01")
	issue(f.secret, "secreta", nil, "2026-12-11")
	f.exec(`INSERT INTO sprints (project_id, name, start_date, end_date) VALUES ($1, 'Sprint 9', '2026-12-08', '2026-12-18')`, f.open)
	f.exec(`INSERT INTO versions (project_id, name, release_date) VALUES ($1, 'v1.0', '2026-12-15')`, f.open)
	f.exec(`INSERT INTO versions (project_id, name, release_date, status, archived_at) VALUES ($1, 'v0.9', '2026-12-16', 'archived', now())`, f.open)

	all := []string{port.KindIssueDue, port.KindIssueStart, port.KindSprintStart, port.KindSprintEnd, port.KindVersionRelease}
	q := port.DerivedQuery{Viewer: v(f.owner), From: day(2026, 12, 1), To: day(2027, 1, 1), Kinds: all}
	got, err := f.repo.Derived(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	byKind := map[string][]string{}
	for _, e := range got {
		byKind[e.Kind] = append(byKind[e.Kind], e.Title)
		if e.ProjectKey == "" || e.ProjectID == uuid.Nil || e.Link == "" {
			t.Fatalf("incomplete item %+v", e)
		}
	}
	if len(byKind[port.KindIssueDue]) != 3 || len(byKind[port.KindIssueStart]) != 1 || len(byKind[port.KindSprintStart]) != 1 ||
		len(byKind[port.KindSprintEnd]) != 1 || len(byKind[port.KindVersionRelease]) != 1 || byKind[port.KindVersionRelease][0] != "v1.0" {
		t.Fatalf("derived = %v", byKind)
	}
	if got[0].Date.Day() != 8 {
		t.Fatalf("not ordered by date: %+v", got[0])
	}
	// The `to` bound is exclusive.
	q.To = day(2026, 12, 10)
	if got, _ := f.repo.Derived(ctx, q); len(got) != 1 {
		t.Fatalf("exclusive to: %+v", got)
	}
	q.To = day(2027, 1, 1)
	// Kind selection.
	q.Kinds = []string{port.KindSprintEnd}
	if got, _ := f.repo.Derived(ctx, q); len(got) != 1 || got[0].Kind != port.KindSprintEnd {
		t.Fatalf("kind filter: %+v", got)
	}
	q.Kinds = all
	// A member without role only sees the open project, and only when asking for it (no membership).
	q.Viewer = v(f.member)
	if got, _ := f.repo.Derived(ctx, q); len(got) != 0 {
		t.Fatalf("a non-member of any project got dates without asking: %+v", got)
	}
	q.Projects = []uuid.UUID{f.open, f.secret}
	got, _ = f.repo.Derived(ctx, q)
	for _, e := range got {
		if e.ProjectID == f.secret {
			t.Fatalf("private project leaked: %+v", e)
		}
	}
	if len(got) != 6 {
		t.Fatalf("open project dates for the member = %d", len(got))
	}
	// Another organization sees nothing even when asking.
	q.Viewer = v(f.other)
	if got, _ := f.repo.Derived(ctx, q); len(got) != 0 {
		t.Fatalf("cross-organization leak: %+v", got)
	}
	// The org admin sees the private project when asking for it.
	q.Viewer = v(f.admin)
	if got, _ := f.repo.Derived(ctx, q); len(got) != 7 {
		t.Fatalf("admin dates = %d", len(got))
	}
}

func TestFeedTokensPostgres(t *testing.T) {
	f := newCalFixture(t)
	ctx := context.Background()
	if at, err := f.repo.FeedInfo(ctx, f.owner); err != nil || at != nil {
		t.Fatalf("feed before creation: %v %v", at, err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err := f.repo.ReplaceFeedToken(ctx, f.owner, "hash-1", now); err != nil {
		t.Fatal(err)
	}
	if id, err := f.repo.UserByFeedToken(ctx, "hash-1"); err != nil || id != f.owner {
		t.Fatalf("lookup: %v %v", id, err)
	}
	if err := f.repo.ReplaceFeedToken(ctx, f.owner, "hash-2", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.UserByFeedToken(ctx, "hash-1"); apperror.CodeOf(err) != domain.CodeFeedNotFound {
		t.Fatalf("rotated token still valid: %v", err)
	}
	if at, _ := f.repo.FeedInfo(ctx, f.owner); at == nil || !at.Equal(now.Add(time.Hour)) {
		t.Fatalf("created_at = %v", at)
	}
	if ok, err := f.repo.RevokeFeedToken(ctx, f.owner, now); err != nil || !ok {
		t.Fatalf("revoke: %v %v", ok, err)
	}
	if ok, _ := f.repo.RevokeFeedToken(ctx, f.owner, now); ok {
		t.Fatal("second revoke reported success")
	}
	if _, err := f.repo.UserByFeedToken(ctx, "hash-2"); apperror.CodeOf(err) != domain.CodeFeedNotFound {
		t.Fatalf("revoked token valid: %v", err)
	}
	if at, _ := f.repo.FeedInfo(ctx, f.owner); at != nil {
		t.Fatal("revoked feed reported active")
	}
	// A new token after revocation works again.
	if err := f.repo.ReplaceFeedToken(ctx, f.owner, "hash-3", now); err != nil {
		t.Fatal(err)
	}
	if id, err := f.repo.UserByFeedToken(ctx, "hash-3"); err != nil || id != f.owner {
		t.Fatalf("token after revoke: %v %v", id, err)
	}
}

func TestEventListStatementsDoNotGrowPostgres(t *testing.T) {
	f := newCalFixture(t)
	ctx := context.Background()
	measure := func() int32 {
		f.counter.Store(0)
		if _, err := f.repo.ListVisible(ctx, v(f.owner), day(2026, 12, 1), day(2026, 12, 31), nil); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repo.Derived(ctx, port.DerivedQuery{Viewer: v(f.owner), From: day(2026, 12, 1), To: day(2026, 12, 31), Kinds: []string{port.KindIssueDue}}); err != nil {
			t.Fatal(err)
		}
		return f.counter.Load()
	}
	f.event(f.owner, "uno", domain.VisibilityPrivate, nil, time.Date(2026, 12, 7, 9, 0, 0, 0, time.UTC), f.member)
	small := measure()
	for i := 0; i < 40; i++ {
		f.event(f.owner, "carga", domain.VisibilityOrganization, nil, time.Date(2026, 12, 8, 9, 0, 0, 0, time.UTC), f.member, f.admin)
	}
	if large := measure(); small != large || small != 3 {
		t.Fatalf("statements: %d with 1 event, %d with 41 (want 3 in both)", small, large)
	}
}

package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/calendar/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

var errBoom = errors.New("boom")

// memRepo is an in-memory EventRepository. Visibility: the owner, the guests, and every member of the
// organization for non-private events.
type memRepo struct {
	events   map[uuid.UUID]*domain.Event
	orgs     map[uuid.UUID]uuid.UUID // user -> organization
	roles    map[uuid.UUID]string    // user -> role
	projects map[uuid.UUID]uuid.UUID // project -> organization
	tokens   map[string]uuid.UUID
	feedAt   map[uuid.UUID]time.Time
	derived  []port.Event
	lastDQ   port.DerivedQuery
	fail     bool
}

func newMemRepo() *memRepo {
	return &memRepo{events: map[uuid.UUID]*domain.Event{}, orgs: map[uuid.UUID]uuid.UUID{}, roles: map[uuid.UUID]string{},
		projects: map[uuid.UUID]uuid.UUID{}, tokens: map[string]uuid.UUID{}, feedAt: map[uuid.UUID]time.Time{}}
}

func (m *memRepo) OrganizationFor(_ context.Context, userID, requested uuid.UUID) (uuid.UUID, error) {
	org, ok := m.orgs[userID]
	if !ok || (requested != uuid.Nil && requested != org) {
		return uuid.Nil, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidEvent, "no perteneces a ninguna organización")
	}
	return org, nil
}
func (m *memRepo) ProjectOrganization(_ context.Context, p uuid.UUID) (uuid.UUID, error) {
	return m.projects[p], nil
}
func (m *memRepo) OrgRole(_ context.Context, u, o uuid.UUID) (string, error) {
	if m.orgs[u] != o {
		return "", nil
	}
	return m.roles[u], nil
}
func (m *memRepo) MembersAmong(_ context.Context, org uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, id := range ids {
		if m.orgs[id] == org {
			out = append(out, id)
		}
	}
	return out, nil
}
func (m *memRepo) Create(_ context.Context, e *domain.Event) error {
	if m.fail {
		return errBoom
	}
	e.ID = uuid.New()
	c := *e
	m.events[e.ID] = &c
	return nil
}
func (m *memRepo) Update(_ context.Context, e *domain.Event, replace bool) error {
	c := *e
	m.events[e.ID] = &c
	return nil
}
func (m *memRepo) Delete(_ context.Context, id uuid.UUID, _ time.Time) error {
	delete(m.events, id)
	return nil
}
func (m *memRepo) visible(v port.Viewer, e *domain.Event) bool {
	if e.OwnerID == v.UserID {
		return true
	}
	for _, a := range e.Attendees {
		if a.UserID == v.UserID {
			return true
		}
	}
	return e.Visibility != domain.VisibilityPrivate && m.orgs[v.UserID] == e.OrganizationID
}
func (m *memRepo) FindVisible(_ context.Context, v port.Viewer, id uuid.UUID) (*domain.Event, error) {
	e, ok := m.events[id]
	if !ok || !m.visible(v, e) {
		return nil, domain.ErrEventNotFound
	}
	c := *e
	return &c, nil
}
func (m *memRepo) ListVisible(_ context.Context, v port.Viewer, _, _ time.Time, projects []uuid.UUID) ([]domain.Event, error) {
	var out []domain.Event
	for _, e := range m.events {
		if !m.visible(v, e) {
			continue
		}
		if projects != nil && e.ProjectID != nil {
			found := false
			for _, p := range projects {
				found = found || p == *e.ProjectID
			}
			if !found {
				continue
			}
		}
		out = append(out, *e)
	}
	return out, nil
}
func (m *memRepo) SetResponse(_ context.Context, ev, u uuid.UUID, r domain.Response) (bool, error) {
	e := m.events[ev]
	for i := range e.Attendees {
		if e.Attendees[i].UserID == u {
			e.Attendees[i].Response = r
			return true, nil
		}
	}
	return false, nil
}
func (m *memRepo) Derived(_ context.Context, q port.DerivedQuery) ([]port.Event, error) {
	m.lastDQ = q
	return m.derived, nil
}
func (m *memRepo) FeedInfo(_ context.Context, u uuid.UUID) (*time.Time, error) {
	if at, ok := m.feedAt[u]; ok {
		return &at, nil
	}
	return nil, nil
}
func (m *memRepo) ReplaceFeedToken(_ context.Context, u uuid.UUID, hash string, at time.Time) error {
	for h, owner := range m.tokens {
		if owner == u {
			delete(m.tokens, h)
		}
	}
	m.tokens[hash] = u
	m.feedAt[u] = at
	return nil
}
func (m *memRepo) RevokeFeedToken(_ context.Context, u uuid.UUID, _ time.Time) (bool, error) {
	for h, owner := range m.tokens {
		if owner == u {
			delete(m.tokens, h)
			delete(m.feedAt, u)
			return true, nil
		}
	}
	return false, nil
}
func (m *memRepo) UserByFeedToken(_ context.Context, hash string) (uuid.UUID, error) {
	if u, ok := m.tokens[hash]; ok {
		return u, nil
	}
	return uuid.Nil, domain.ErrFeedNotFound
}

type fakeAuth struct {
	allowed map[string]bool // action + project
	calls   int
}

func (f *fakeAuth) Authorize(_ context.Context, action string, _ uuid.UUID, _ string, project uuid.UUID) error {
	f.calls++
	if f.allowed[action+project.String()] {
		return nil
	}
	if action == port.ActionViewProject {
		return apperror.New(apperror.KindNotFound, "project not found")
	}
	return apperror.New(apperror.KindForbidden, "forbidden")
}

type env struct {
	svc             *EventService
	repo            *memRepo
	auth            *fakeAuth
	org             uuid.UUID
	owner, member   uuid.UUID
	admin, outsider uuid.UUID
	project         uuid.UUID
	now             time.Time
}

func newEnv() *env {
	e := &env{repo: newMemRepo(), auth: &fakeAuth{allowed: map[string]bool{}}, org: uuid.New(), owner: uuid.New(), member: uuid.New(), admin: uuid.New(), outsider: uuid.New(), project: uuid.New(),
		now: time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)}
	other := uuid.New()
	e.repo.orgs[e.owner], e.repo.orgs[e.member], e.repo.orgs[e.admin], e.repo.orgs[e.outsider] = e.org, e.org, e.org, other
	e.repo.roles[e.owner], e.repo.roles[e.member], e.repo.roles[e.admin] = "member", "member", "admin"
	e.repo.projects[e.project] = e.org
	e.svc = NewEventService(e.repo, e.auth, clock.NewFixed(e.now))
	return e
}

func viewer(id uuid.UUID) port.Viewer { return port.Viewer{UserID: id, GlobalRole: "user"} }

func (e *env) input() port.EventInput {
	return port.EventInput{Title: "Reunión", StartsAt: time.Date(2026, 12, 7, 9, 0, 0, 0, time.UTC), EndsAt: time.Date(2026, 12, 7, 10, 0, 0, 0, time.UTC)}
}

func wantErr(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want %s/%s, got nil", kind, code)
	}
	if apperror.KindOf(err) != kind || (code != "" && apperror.CodeOf(err) != code) {
		t.Fatalf("want %s/%s, got %s/%s (%v)", kind, code, apperror.KindOf(err), apperror.CodeOf(err), err)
	}
}

func TestCreatePermissionMatrix(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	project := e.project
	e.auth.allowed[port.ActionViewProject+project.String()] = true

	cases := []struct {
		name    string
		user    uuid.UUID
		mutate  func(*port.EventInput)
		allowEd bool
		kind    apperror.Kind
		code    string
	}{
		{"private personal", e.member, func(*port.EventInput) {}, false, "", ""},
		{"private tagged to a visible project", e.member, func(in *port.EventInput) { in.ProjectID = &project }, false, "", ""},
		{"project event without edit rights", e.member, func(in *port.EventInput) { in.ProjectID = &project; in.Visibility = domain.VisibilityProject }, false, apperror.KindForbidden, ""},
		{"project event with edit rights", e.member, func(in *port.EventInput) { in.ProjectID = &project; in.Visibility = domain.VisibilityProject }, true, "", ""},
		{"project event without project", e.member, func(in *port.EventInput) { in.Visibility = domain.VisibilityProject }, false, apperror.KindValidation, domain.CodeInvalidEvent},
		{"organization event by a member", e.member, func(in *port.EventInput) { in.Visibility = domain.VisibilityOrganization }, false, apperror.KindForbidden, CodeForbidden},
		{"organization event by an admin", e.admin, func(in *port.EventInput) { in.Visibility = domain.VisibilityOrganization }, false, "", ""},
		{"user without organization", uuid.New(), func(*port.EventInput) {}, false, apperror.KindValidation, domain.CodeInvalidEvent},
		{"invalid title", e.member, func(in *port.EventInput) { in.Title = " " }, false, apperror.KindValidation, domain.CodeInvalidEvent},
		{"invalid recurrence", e.member, func(in *port.EventInput) { r := "FREQ=SOMETIMES"; in.RecurrenceRule = &r }, false, apperror.KindValidation, domain.CodeInvalidRecurrence},
		{"attendee outside the organization", e.member, func(in *port.EventInput) { in.Attendees = []uuid.UUID{e.outsider} }, false, apperror.KindValidation, domain.CodeInvalidEvent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e.auth.allowed[port.ActionEditIssue+project.String()] = c.allowEd
			in := e.input()
			c.mutate(&in)
			view, err := e.svc.Create(ctx, viewer(c.user), in)
			if c.kind != "" {
				wantErr(t, err, c.kind, c.code)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if view.Event.OwnerID != c.user || view.Event.OrganizationID != e.org || !view.CanEdit || !view.Event.CreatedAt.Equal(e.now) {
				t.Fatalf("unexpected view: %+v", view)
			}
		})
	}
}

func TestCreateWithAttendeesAndProjectOfAnotherOrganizationVisibility(t *testing.T) {
	e := newEnv()
	in := e.input()
	in.Attendees = []uuid.UUID{e.member, e.member, e.admin}
	view, err := e.svc.Create(context.Background(), viewer(e.owner), in)
	if err != nil || len(view.Event.Attendees) != 2 || view.Event.Attendees[0].Response != domain.ResponseNeedsAction {
		t.Fatalf("attendees: %+v %v", view, err)
	}
	// A guest sees the private event and cannot edit it.
	got, err := e.svc.Get(context.Background(), viewer(e.member), view.Event.ID)
	if err != nil || got.CanEdit || got.MyResponse != domain.ResponseNeedsAction {
		t.Fatalf("guest view: %+v %v", got, err)
	}
	// Someone else gets 404, not 403.
	_, err = e.svc.Get(context.Background(), viewer(e.outsider), view.Event.ID)
	wantErr(t, err, apperror.KindNotFound, domain.CodeEventNotFound)
	_, err = e.svc.Get(context.Background(), viewer(e.admin), uuid.New())
	wantErr(t, err, apperror.KindNotFound, domain.CodeEventNotFound)
}

func TestUpdatePermissionsAndExceptionsReset(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	e.auth.allowed[port.ActionViewProject+e.project.String()] = true
	e.auth.allowed[port.ActionEditIssue+e.project.String()] = true
	in := e.input()
	in.ProjectID, in.Visibility = &e.project, domain.VisibilityProject
	rule := "FREQ=DAILY;COUNT=5"
	in.RecurrenceRule = &rule
	created, err := e.svc.Create(ctx, viewer(e.owner), in)
	if err != nil {
		t.Fatal(err)
	}
	id := created.Event.ID
	title := "Nuevo"

	// A project member with edit rights can edit; one without gets 403 (the event is visible to them).
	view, err := e.svc.Update(ctx, viewer(e.member), id, nil, port.EventPatch{Title: &title})
	if err != nil || view.Event.Title != title || !view.CanEdit {
		t.Fatalf("edit with rights: %+v %v", view, err)
	}
	e.auth.allowed[port.ActionEditIssue+e.project.String()] = false
	_, err = e.svc.Update(ctx, viewer(e.member), id, nil, port.EventPatch{Title: &title})
	wantErr(t, err, apperror.KindForbidden, CodeForbidden)
	if err := e.svc.Delete(ctx, viewer(e.member), id, nil); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("delete without rights: %v", err)
	}
	// The owner always can.
	occ := time.Date(2026, 12, 8, 9, 0, 0, 0, time.UTC)
	if err := e.svc.Delete(ctx, viewer(e.owner), id, &occ); err != nil {
		t.Fatalf("cancel occurrence: %v", err)
	}
	if got := e.repo.events[id].Exceptions; len(got) != 1 || !got[0].Cancelled {
		t.Fatalf("exceptions: %+v", got)
	}
	// Changing the rule drops the exceptions that pointed at the old series.
	newRule := "FREQ=WEEKLY"
	view, err = e.svc.Update(ctx, viewer(e.owner), id, nil, port.EventPatch{RecurrenceRule: port.OptString{Set: true, Value: &newRule}})
	if err != nil || len(view.Event.Exceptions) != 0 || *view.Event.RecurrenceRule != "FREQ=WEEKLY" {
		t.Fatalf("rule change: %+v %v", view, err)
	}
	// Clearing the rule.
	view, err = e.svc.Update(ctx, viewer(e.owner), id, nil, port.EventPatch{RecurrenceRule: port.OptString{Set: true}})
	if err != nil || view.Event.Recurring() {
		t.Fatalf("clear rule: %+v %v", view, err)
	}
	// Organization visibility needs an organization admin.
	org := domain.VisibilityOrganization
	_, err = e.svc.Update(ctx, viewer(e.owner), id, nil, port.EventPatch{Visibility: &org, ProjectID: port.OptUUID{Set: true}})
	wantErr(t, err, apperror.KindForbidden, CodeForbidden)
	// Attendees are replaced and keep the responses of those that stay.
	guests := []uuid.UUID{e.member, e.admin}
	if _, err := e.svc.Update(ctx, viewer(e.owner), id, nil, port.EventPatch{Attendees: &guests}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Respond(ctx, viewer(e.member), id, domain.ResponseAccepted); err != nil {
		t.Fatal(err)
	}
	guests = []uuid.UUID{e.member}
	view, err = e.svc.Update(ctx, viewer(e.owner), id, nil, port.EventPatch{Attendees: &guests})
	if err != nil || len(view.Event.Attendees) != 1 || view.Event.Attendees[0].Response != domain.ResponseAccepted {
		t.Fatalf("attendees kept: %+v %v", view.Event.Attendees, err)
	}
}

func TestOccurrenceOverrides(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	in := e.input()
	rule := "FREQ=WEEKLY;BYDAY=MO"
	in.RecurrenceRule = &rule
	created, _ := e.svc.Create(ctx, viewer(e.owner), in)
	id := created.Event.ID
	occ := time.Date(2026, 12, 14, 9, 0, 0, 0, time.UTC)
	title, moved := "Esta vez en la sala B", time.Date(2026, 12, 14, 15, 0, 0, 0, time.UTC)

	view, err := e.svc.Update(ctx, viewer(e.owner), id, &occ, port.EventPatch{Title: &title, StartsAt: &moved})
	if err != nil || len(view.Event.Exceptions) != 1 || *view.Event.Exceptions[0].Override.Title != title {
		t.Fatalf("override: %+v %v", view.Event.Exceptions, err)
	}
	// A second patch on the same occurrence merges with the first.
	loc := "Sala B"
	view, err = e.svc.Update(ctx, viewer(e.owner), id, &occ, port.EventPatch{Location: &loc})
	if err != nil || *view.Event.Exceptions[0].Override.Title != title || *view.Event.Exceptions[0].Override.Location != loc {
		t.Fatalf("merge: %+v %v", view.Event.Exceptions, err)
	}
	// Not an occurrence of the series, a series-level field, an invalid title and a non-recurring event.
	notOcc := time.Date(2026, 12, 15, 9, 0, 0, 0, time.UTC)
	_, err = e.svc.Update(ctx, viewer(e.owner), id, &notOcc, port.EventPatch{Title: &title})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidOccurrence)
	tz := "Europe/Madrid"
	_, err = e.svc.Update(ctx, viewer(e.owner), id, &occ, port.EventPatch{Timezone: &tz})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidOccurrence)
	blank := " "
	_, err = e.svc.Update(ctx, viewer(e.owner), id, &occ, port.EventPatch{Title: &blank})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidEvent)
	early := time.Date(2026, 12, 14, 8, 0, 0, 0, time.UTC)
	_, err = e.svc.Update(ctx, viewer(e.owner), id, &occ, port.EventPatch{EndsAt: &early})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidEvent)
	wantErr(t, e.svc.Delete(ctx, viewer(e.owner), id, &notOcc), apperror.KindValidation, domain.CodeInvalidOccurrence)
	single, _ := e.svc.Create(ctx, viewer(e.owner), e.input())
	_, err = e.svc.Update(ctx, viewer(e.owner), single.Event.ID, &occ, port.EventPatch{Title: &title})
	wantErr(t, err, apperror.KindValidation, domain.CodeInvalidOccurrence)

	// The expanded calendar shows the override and keeps the original start for later edits.
	res, err := e.svc.Items(ctx, viewer(e.owner), port.ItemsQuery{From: time.Date(2026, 12, 14, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 12, 22, 0, 0, 0, 0, time.UTC), Kinds: []string{"events"}})
	if err != nil || len(res.Items) != 2 {
		t.Fatalf("items: %+v %v", res, err)
	}
	if res.Items[0].Title != title || !res.Items[0].StartsAt.Equal(moved) || res.Items[0].OccurrenceStart == nil || !res.Items[0].OccurrenceStart.Equal(occ) || res.Items[0].Location != loc {
		t.Fatalf("first item: %+v", res.Items[0])
	}
	if !res.Items[1].Recurring || res.Items[1].Title != "Reunión" {
		t.Fatalf("second item: %+v", res.Items[1])
	}
}

func TestItemsMergesDerivedAndValidates(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	e.auth.allowed[port.ActionViewProject+e.project.String()] = true
	e.repo.derived = []port.Event{{Kind: port.KindIssueDue, Date: time.Date(2026, 12, 7, 0, 0, 0, 0, time.UTC), Title: "Entrega", SourceID: uuid.New(), ProjectID: e.project, ProjectKey: "ABC", Link: "/issues/x"}}
	if _, err := e.svc.Create(ctx, viewer(e.owner), e.input()); err != nil {
		t.Fatal(err)
	}
	q := port.ItemsQuery{From: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)}
	res, err := e.svc.Items(ctx, viewer(e.owner), q)
	if err != nil || len(res.Items) != 2 {
		t.Fatalf("items: %+v %v", res, err)
	}
	if res.Items[0].Kind != port.KindEvent && res.Items[0].Kind != port.KindIssueDue {
		t.Fatalf("kinds: %+v", res.Items)
	}
	var due port.Item
	for _, it := range res.Items {
		if it.Kind == port.KindIssueDue {
			due = it
		}
	}
	if !due.AllDay || !due.EndsAt.Equal(due.StartsAt.AddDate(0, 0, 1)) || due.ProjectKey != "ABC" || due.Color != "red" {
		t.Fatalf("derived item: %+v", due)
	}
	if len(e.repo.lastDQ.Kinds) != 5 {
		t.Fatalf("derived kinds asked: %v", e.repo.lastDQ.Kinds)
	}
	// Only events: the derived reader is not called.
	e.repo.lastDQ = port.DerivedQuery{}
	q.Kinds = []string{"event"}
	if res, _ := e.svc.Items(ctx, viewer(e.owner), q); len(res.Items) != 1 || e.repo.lastDQ.Kinds != nil {
		t.Fatalf("event-only: %+v dq=%+v", res, e.repo.lastDQ)
	}
	// Only sprints and versions.
	q.Kinds = []string{"sprints", "version_release"}
	if _, err := e.svc.Items(ctx, viewer(e.owner), q); err != nil || strings.Join(e.repo.lastDQ.Kinds, ",") != "sprint_start,sprint_end,version_release" {
		t.Fatalf("kinds: %v %v", e.repo.lastDQ.Kinds, err)
	}
	// Projects are authorized one by one: a hidden project is 404.
	q.Projects = []uuid.UUID{uuid.New()}
	_, err = e.svc.Items(ctx, viewer(e.owner), q)
	wantErr(t, err, apperror.KindNotFound, "")
	q.Projects = []uuid.UUID{e.project}
	if _, err := e.svc.Items(ctx, viewer(e.owner), q); err != nil || len(e.repo.lastDQ.Projects) != 1 {
		t.Fatalf("project filter: %v %+v", err, e.repo.lastDQ)
	}
	// Invalid window and invalid kinds.
	_, err = e.svc.Items(ctx, viewer(e.owner), port.ItemsQuery{From: q.To, To: q.From})
	wantErr(t, err, apperror.KindValidation, "calendar.invalid_range")
	_, err = e.svc.Items(ctx, viewer(e.owner), port.ItemsQuery{From: q.From, To: q.To, Kinds: []string{"birthdays"}})
	wantErr(t, err, apperror.KindValidation, "calendar.invalid_types")
	// Someone outside the organization sees none of the organization's events.
	res, _ = e.svc.Items(ctx, viewer(e.outsider), port.ItemsQuery{From: q.From, To: q.To, Kinds: []string{"events"}})
	if len(res.Items) != 0 {
		t.Fatalf("leak: %+v", res.Items)
	}
}

func TestRespond(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	in := e.input()
	in.Visibility = domain.VisibilityOrganization
	in.Attendees = []uuid.UUID{e.member}
	created, err := e.svc.Create(ctx, viewer(e.admin), in)
	if err != nil {
		t.Fatal(err)
	}
	id := created.Event.ID
	if err := e.svc.Respond(ctx, viewer(e.member), id, domain.ResponseTentative); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.svc.Get(ctx, viewer(e.member), id); got.MyResponse != domain.ResponseTentative {
		t.Fatalf("my response = %q", got.MyResponse)
	}
	wantErr(t, e.svc.Respond(ctx, viewer(e.member), id, "maybe"), apperror.KindValidation, domain.CodeInvalidEvent)
	// Visible to the organization but not invited: 403.
	wantErr(t, e.svc.Respond(ctx, viewer(e.owner), id, domain.ResponseAccepted), apperror.KindForbidden, domain.CodeNotAttendee)
	// Not visible: 404.
	wantErr(t, e.svc.Respond(ctx, viewer(e.outsider), id, domain.ResponseAccepted), apperror.KindNotFound, domain.CodeEventNotFound)
}

func TestFeedTokenLifecycle(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	n := 0
	e.svc.WithTokenGenerator(func() (string, error) {
		n++
		return strings.Repeat(string(rune('a'+n)), 43), nil
	})
	if at, _ := e.svc.FeedInfo(ctx, e.owner); at != nil {
		t.Fatal("feed active before creation")
	}
	if _, err := e.svc.Create(ctx, viewer(e.owner), e.input()); err != nil {
		t.Fatal(err)
	}
	first, err := e.svc.CreateFeed(ctx, e.owner)
	if err != nil || first.Token == "" || !first.CreatedAt.Equal(e.now) {
		t.Fatalf("create feed: %+v %v", first, err)
	}
	if _, stored := e.repo.tokens[first.Token]; stored {
		t.Fatal("the raw token must not be stored")
	}
	body, err := e.svc.RenderFeed(ctx, first.Token)
	if err != nil || !strings.HasPrefix(body, "BEGIN:VCALENDAR\r\n") || !strings.Contains(body, "SUMMARY:Reunión") {
		t.Fatalf("feed: %v\n%s", err, body)
	}
	// Rotation invalidates the previous token.
	second, _ := e.svc.CreateFeed(ctx, e.owner)
	_, err = e.svc.RenderFeed(ctx, first.Token)
	wantErr(t, err, apperror.KindNotFound, domain.CodeFeedNotFound)
	if _, err := e.svc.RenderFeed(ctx, second.Token); err != nil {
		t.Fatal(err)
	}
	// Garbage and revoked tokens are 404.
	_, err = e.svc.RenderFeed(ctx, "short")
	wantErr(t, err, apperror.KindNotFound, domain.CodeFeedNotFound)
	if err := e.svc.RevokeFeed(ctx, e.owner); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.RenderFeed(ctx, second.Token)
	wantErr(t, err, apperror.KindNotFound, domain.CodeFeedNotFound)
	wantErr(t, e.svc.RevokeFeed(ctx, e.owner), apperror.KindNotFound, domain.CodeFeedNotFound)
	// The token generator failing is an internal error.
	e.svc.WithTokenGenerator(func() (string, error) { return "", errBoom })
	if _, err := e.svc.CreateFeed(ctx, e.owner); apperror.KindOf(err) != apperror.KindInternal {
		t.Fatalf("generator failure: %v", err)
	}
	if tok, err := randomToken(); err != nil || len(tok) != 43 {
		t.Fatalf("randomToken: %q %v", tok, err)
	}
}

func TestParseKinds(t *testing.T) {
	all, err := ParseKinds(nil)
	if err != nil || len(all) != 6 {
		t.Fatalf("all: %v %v", all, err)
	}
	set, err := ParseKinds([]string{"issues", " sprint_end ", "events"})
	if err != nil || len(set) != 4 || !set[port.KindIssueDue] || !set[port.KindIssueStart] || !set[port.KindSprintEnd] || !set[port.KindEvent] {
		t.Fatalf("set: %v %v", set, err)
	}
	if _, err := ParseKinds([]string{"nope"}); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestCreatePropagatesRepositoryFailure(t *testing.T) {
	e := newEnv()
	e.repo.fail = true
	if _, err := e.svc.Create(context.Background(), viewer(e.owner), e.input()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
}

package postgres_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/search/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/application/usecase"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

type filterEnv struct {
	*searchEnv
	clock *clock.FakeClock
	svc   *usecase.FilterService
	repo  *postgres.FilterRepository
}

func newFilterEnv(t *testing.T) *filterEnv {
	t.Helper()
	e := newSearchEnv(t)
	clk := &clock.FakeClock{T: testNow}
	jql := usecase.NewJQLService(e.repo, e.repo, e.repo, clk)
	repo := postgres.NewFilterRepository(e.db)
	return &filterEnv{searchEnv: e, clock: clk, svc: usecase.NewFilterService(repo, jql, clk), repo: repo}
}

func (f *filterEnv) create(owner uuid.UUID, name, query string) uuid.UUID {
	f.t.Helper()
	d, err := f.svc.Create(f.ctx, f.viewer(owner), uuid.Nil, domain.FilterFields{Name: name, QueryText: query})
	if err != nil {
		f.t.Fatalf("create %q: %v", name, err)
	}
	return d.ID
}

func (f *filterEnv) share(owner uuid.UUID, id uuid.UUID, shares ...domain.Share) {
	f.t.Helper()
	if _, err := f.svc.SetShares(f.ctx, owner, id, shares); err != nil {
		f.t.Fatalf("share: %v", err)
	}
}

func (f *filterEnv) run(user, id uuid.UUID) ([]string, error) {
	out, err := f.svc.Run(f.ctx, f.viewer(user), id, 1, 100, true)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	for _, it := range out.Page.Items {
		keys = append(keys, it.Key)
	}
	return keys, nil
}

func TestFilterCreateAndValidation(t *testing.T) {
	f := newFilterEnv(t)
	d, err := f.svc.Create(f.ctx, f.viewer(f.alice), uuid.Nil, domain.FilterFields{
		Name: "  Mis bugs  ", Description: "errores abiertos", QueryText: "type = Error AND assignee = currentUser()",
		Columns: []string{"key", "summary", "key"}, Sort: "updated DESC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Mis bugs" || d.Access != domain.AccessOwner || d.OwnerID != f.alice || d.OrganizationID.String() != testOrg ||
		len(d.Columns) != 2 || d.Sort != "updated DESC" || d.OwnerName != "alice" {
		t.Fatalf("created: %+v", d)
	}
	// Invalid queries are rejected with located problems and nothing is stored.
	for _, q := range []string{"type = ", "projct = A", "timeSpent > 1", "statusCategory = nope"} {
		_, err := f.svc.Create(f.ctx, f.viewer(f.alice), uuid.Nil, domain.FilterFields{Name: "x" + q, QueryText: q})
		var qe *domain.QueryError
		if !errors.As(err, &qe) || apperror.KindOf(err) != apperror.KindValidation {
			t.Fatalf("%q: %v", q, err)
		}
	}
	if _, err := f.svc.Create(f.ctx, f.viewer(f.alice), uuid.Nil, domain.FilterFields{Name: "ok", QueryText: "labels = a", Sort: "labels"}); apperror.CodeOf(err) != domain.CodeInvalidQuery {
		t.Fatalf("invalid sort: %v", err)
	}
	if _, err := f.svc.Create(f.ctx, f.viewer(f.alice), uuid.Nil, domain.FilterFields{Name: "ok", QueryText: "labels = a", Sort: "created ASC garbage"}); err == nil {
		t.Fatal("malformed sort accepted")
	}
	if _, err := f.svc.Create(f.ctx, f.viewer(f.alice), uuid.Nil, domain.FilterFields{Name: ""}); apperror.CodeOf(err) != domain.CodeFilterInvalid {
		t.Fatalf("empty name: %v", err)
	}
	// Names are unique per owner, ignoring case; other owners may reuse them.
	if _, err := f.svc.Create(f.ctx, f.viewer(f.alice), uuid.Nil, domain.FilterFields{Name: "MIS BUGS"}); apperror.CodeOf(err) != domain.CodeFilterNameTaken || apperror.KindOf(err) != apperror.KindConflict {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := f.svc.Create(f.ctx, f.viewer(f.bob), uuid.Nil, domain.FilterFields{Name: "Mis bugs"}); err != nil {
		t.Fatalf("another owner: %v", err)
	}
	// A user outside any organization cannot own a filter; an organization the user is not in is refused.
	if _, err := f.svc.Create(f.ctx, f.viewer(f.outsider), uuid.Nil, domain.FilterFields{Name: "x"}); apperror.CodeOf(err) != domain.CodeFilterInvalid {
		t.Fatalf("outsider: %v", err)
	}
	if _, err := f.svc.Create(f.ctx, f.viewer(f.alice), f.org2, domain.FilterFields{Name: "x"}); apperror.CodeOf(err) != domain.CodeFilterInvalid {
		t.Fatalf("foreign organization: %v", err)
	}
	// An empty query is a valid filter ("everything I can browse").
	all := f.create(f.alice, "Todo", "")
	if keys, err := f.run(f.alice, all); err != nil || len(keys) != 6 {
		t.Fatalf("empty filter: %v %v", keys, err)
	}
}

func TestFilterRunsWithStoredOrderAndPaging(t *testing.T) {
	f := newFilterEnv(t)
	d, err := f.svc.Create(f.ctx, f.viewer(f.alice), uuid.Nil, domain.FilterFields{Name: "PUB por clave", QueryText: "project = PUB ORDER BY created ASC", Sort: "key DESC"})
	if err != nil {
		t.Fatal(err)
	}
	keys, err := f.run(f.alice, d.ID)
	if err != nil || strings.Join(keys, ",") != "PUB-5,PUB-4,PUB-3,PUB-2,PUB-1" {
		t.Fatalf("stored sort overrides the query: %v %v", keys, err)
	}
	out, err := f.svc.Run(f.ctx, f.viewer(f.alice), d.ID, 2, 2, true)
	if err != nil || out.Page.Total != 5 || len(out.Page.Items) != 2 || out.Page.Items[0].Key != "PUB-3" {
		t.Fatalf("paging: %v %+v", err, out.Page)
	}
}

// TestFilterAccessMatrix checks who can read, run, edit, delete and share a filter, for every kind of share.
func TestFilterAccessMatrix(t *testing.T) {
	f := newFilterEnv(t)
	id := f.create(f.alice, "Compartido", "project = PUB OR project = PRIV")

	// Private: only the owner.
	for name, user := range map[string]uuid.UUID{"bob": f.bob, "carol": f.carol, "dave": f.dave, "outsider": f.outsider, "owner": f.owner, "admin": f.admin} {
		if _, err := f.svc.Get(f.ctx, user, id); !errors.Is(err, domain.ErrFilterNotFound) {
			t.Fatalf("%s must not see a private filter: %v", name, err)
		}
	}
	if d, err := f.svc.Get(f.ctx, f.alice, id); err != nil || d.Access != domain.AccessOwner {
		t.Fatalf("owner: %v %+v", err, d)
	}

	type expect struct {
		user   string
		access domain.Access
	}
	matrix := []struct {
		name    string
		shares  []domain.Share
		expects []expect
	}{
		{"user view", []domain.Share{{Type: domain.ShareUser, TargetID: f.bob, Permission: domain.PermissionView}},
			[]expect{{"bob", domain.AccessView}, {"carol", domain.AccessNone}, {"outsider", domain.AccessNone}}},
		{"user edit", []domain.Share{{Type: domain.ShareUser, TargetID: f.bob, Permission: domain.PermissionEdit}},
			[]expect{{"bob", domain.AccessEdit}, {"carol", domain.AccessNone}}},
		{"team view", []domain.Share{{Type: domain.ShareTeam, TargetID: f.teamBackend, Permission: domain.PermissionView}},
			[]expect{{"bob", domain.AccessView}, {"carol", domain.AccessNone}}},
		{"other team", []domain.Share{{Type: domain.ShareTeam, TargetID: f.teamOther, Permission: domain.PermissionView}},
			[]expect{{"bob", domain.AccessNone}, {"carol", domain.AccessView}}},
		{"project", []domain.Share{{Type: domain.ShareProject, TargetID: f.pub}},
			[]expect{{"bob", domain.AccessView}, {"carol", domain.AccessNone}, {"dave", domain.AccessNone}}},
		{"private project", []domain.Share{{Type: domain.ShareProject, TargetID: f.priv}},
			[]expect{{"bob", domain.AccessNone}, {"owner", domain.AccessView}}},
		{"organization", []domain.Share{{Type: domain.ShareOrganization, TargetID: uuid.MustParse(testOrg)}},
			[]expect{{"bob", domain.AccessView}, {"carol", domain.AccessView}, {"owner", domain.AccessView}, {"dave", domain.AccessNone}, {"outsider", domain.AccessNone}}},
		{"best permission wins", []domain.Share{
			{Type: domain.ShareOrganization, TargetID: uuid.MustParse(testOrg), Permission: domain.PermissionView},
			{Type: domain.ShareUser, TargetID: f.bob, Permission: domain.PermissionEdit}},
			[]expect{{"bob", domain.AccessEdit}, {"carol", domain.AccessView}}},
		{"unshared", nil, []expect{{"bob", domain.AccessNone}, {"carol", domain.AccessNone}}},
	}
	users := map[string]uuid.UUID{"bob": f.bob, "carol": f.carol, "dave": f.dave, "outsider": f.outsider, "owner": f.owner}
	for _, row := range matrix {
		f.share(f.alice, id, row.shares...)
		for _, ex := range row.expects {
			d, err := f.svc.Get(f.ctx, users[ex.user], id)
			if ex.access == domain.AccessNone {
				if !errors.Is(err, domain.ErrFilterNotFound) {
					t.Errorf("%s/%s: want not found, got %v %+v", row.name, ex.user, err, d)
				}
				continue
			}
			if err != nil || d.Access != ex.access {
				t.Errorf("%s/%s: want %v, got %v %+v", row.name, ex.user, ex.access, err, d)
				continue
			}
			if len(d.Shares) != 0 {
				t.Errorf("%s/%s: only the owner receives the shares", row.name, ex.user)
			}
		}
	}

	// Permissions per action.
	f.share(f.alice, id,
		domain.Share{Type: domain.ShareUser, TargetID: f.bob, Permission: domain.PermissionEdit},
		domain.Share{Type: domain.ShareUser, TargetID: f.carol, Permission: domain.PermissionView})
	newName := "Renombrado"
	if d, err := f.svc.Update(f.ctx, f.viewer(f.bob), id, port.FilterPatch{Name: &newName}); err != nil || d.Name != newName {
		t.Fatalf("editor update: %v", err)
	}
	if _, err := f.svc.Update(f.ctx, f.viewer(f.carol), id, port.FilterPatch{Name: &newName}); !errors.Is(err, domain.ErrFilterForbidden) || apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer update: %v", err)
	}
	if _, err := f.svc.Update(f.ctx, f.viewer(f.outsider), id, port.FilterPatch{Name: &newName}); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("outsider update: %v", err)
	}
	for _, user := range []uuid.UUID{f.bob, f.carol} {
		if err := f.svc.Delete(f.ctx, user, id); !errors.Is(err, domain.ErrFilterForbidden) {
			t.Fatalf("delete by a non-owner: %v", err)
		}
		if _, err := f.svc.SetShares(f.ctx, user, id, nil); !errors.Is(err, domain.ErrFilterForbidden) {
			t.Fatalf("share by a non-owner: %v", err)
		}
	}
	// Viewers and editors run it.
	if _, err := f.run(f.carol, id); err != nil {
		t.Fatalf("viewer run: %v", err)
	}
	// An editor keeps the same query validation.
	bad := "projct ="
	if _, err := f.svc.Update(f.ctx, f.viewer(f.bob), id, port.FilterPatch{QueryText: &bad}); err == nil {
		t.Fatal("editor stored an invalid query")
	}
	// Leaving the organization removes the access.
	f.exec(`DELETE FROM organization_members WHERE user_id = $1`, f.bob)
	if _, err := f.svc.Get(f.ctx, f.bob, id); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("former member keeps access: %v", err)
	}
	// Delete by the owner is logical: the filter, its shares and subscriptions stay stored and hidden; favorites go.
	f.exec(`INSERT INTO favorites (user_id, entity_type, entity_id) VALUES ($1, 'filter', $2)`, f.carol, id)
	if _, err := f.svc.Subscribe(f.ctx, f.viewer(f.carol), id, domain.Schedule{Frequency: domain.FrequencyDaily, Hour: 9}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Delete(f.ctx, f.alice, id); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := f.db.Get(&n, `SELECT count(*) FROM favorites WHERE entity_type = 'filter'`); err != nil || n != 0 {
		t.Fatalf("favorites left = %d (%v)", n, err)
	}
	if err := f.db.Get(&n, `SELECT count(*) FROM filters WHERE id = $1 AND deleted_at IS NOT NULL`, id); err != nil || n != 1 {
		t.Fatalf("filter row must be kept with deleted_at set: %d (%v)", n, err)
	}
	if err := f.db.Get(&n, `SELECT count(*) FROM filter_shares WHERE filter_id = $1`, id); err != nil || n == 0 {
		t.Fatalf("shares must be kept for history: %d (%v)", n, err)
	}
	if due, err := f.repo.ListSubscriptions(f.ctx, uuid.Nil, 10); err != nil || len(due) != 0 {
		t.Fatalf("subscriptions of a deleted filter must be hidden: %d (%v)", len(due), err)
	}
	if err := f.svc.Delete(f.ctx, f.alice, id); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestFilterShareTargetsMustBelongToTheOrganization(t *testing.T) {
	f := newFilterEnv(t)
	id := f.create(f.alice, "F", "")
	bad := [][]domain.Share{
		{{Type: domain.ShareUser, TargetID: f.dave}},                     // member of another organization
		{{Type: domain.ShareUser, TargetID: f.outsider}},                 // not a member of any
		{{Type: domain.ShareProject, TargetID: f.oth}},                   // project of another organization
		{{Type: domain.ShareOrganization, TargetID: f.org2}},             // another organization
		{{Type: domain.ShareTeam, TargetID: uuid.New()}},                 // unknown team
		{{Type: domain.ShareType("group"), TargetID: f.bob}},             // unknown type
		{{Type: domain.ShareUser, TargetID: uuid.Nil}},                   // no target
		{{Type: domain.ShareUser, TargetID: f.bob, Permission: "admin"}}, // unknown permission
		{{Type: domain.ShareUser, TargetID: f.bob}, {Type: domain.ShareUser, TargetID: f.dave}},
	}
	for i, shares := range bad {
		if _, err := f.svc.SetShares(f.ctx, f.alice, id, shares); apperror.KindOf(err) != apperror.KindValidation {
			t.Errorf("case %d accepted: %v", i, err)
		}
	}
	// A rejected list leaves the previous shares untouched.
	f.share(f.alice, id, domain.Share{Type: domain.ShareUser, TargetID: f.bob})
	if _, err := f.svc.SetShares(f.ctx, f.alice, id, bad[0]); err == nil {
		t.Fatal("expected a failure")
	}
	d, _ := f.svc.Get(f.ctx, f.alice, id)
	if len(d.Shares) != 1 || d.Shares[0].TargetName != "bob" || d.Shares[0].Permission != domain.PermissionView {
		t.Fatalf("shares after a failed replace: %+v", d.Shares)
	}
	// Duplicates collapse; more than the cap is refused.
	f.share(f.alice, id, domain.Share{Type: domain.ShareUser, TargetID: f.bob}, domain.Share{Type: domain.ShareUser, TargetID: f.bob, Permission: domain.PermissionEdit})
	if d, _ = f.svc.Get(f.ctx, f.alice, id); len(d.Shares) != 1 {
		t.Fatalf("duplicates: %+v", d.Shares)
	}
	many := make([]domain.Share, 101)
	for i := range many {
		many[i] = domain.Share{Type: domain.ShareUser, TargetID: uuid.New()}
	}
	if _, err := f.svc.SetShares(f.ctx, f.alice, id, many); apperror.CodeOf(err) != domain.CodeShareInvalid {
		t.Fatalf("too many shares: %v", err)
	}
	// Names of every target kind resolve.
	f.share(f.alice, id,
		domain.Share{Type: domain.ShareTeam, TargetID: f.teamBackend}, domain.Share{Type: domain.ShareProject, TargetID: f.pub},
		domain.Share{Type: domain.ShareOrganization, TargetID: uuid.MustParse(testOrg)}, domain.Share{Type: domain.ShareUser, TargetID: f.carol})
	d, _ = f.svc.Get(f.ctx, f.alice, id)
	names := map[string]string{}
	for _, s := range d.Shares {
		names[string(s.Type)] = s.TargetName
	}
	if names["team"] != "Backend" || names["project"] != "Publico" || names["organization"] == "" || names["user"] != "carol" {
		t.Fatalf("share names: %+v", names)
	}
}

// TestFilterNeverRevealsWhatTheViewerCannotBrowse shares a filter over a private project: whoever runs it gets
// only what they can browse themselves.
func TestFilterNeverRevealsWhatTheViewerCannotBrowse(t *testing.T) {
	f := newFilterEnv(t)
	id := f.create(f.alice, "Todo incluido privado", "project = PRIV OR project = PUB OR text ~ secreto")
	f.share(f.alice, id, domain.Share{Type: domain.ShareOrganization, TargetID: uuid.MustParse(testOrg)})
	own, err := f.run(f.alice, id)
	if err != nil || len(own) != 6 {
		t.Fatalf("owner run: %v %v", own, err)
	}
	for _, user := range []uuid.UUID{f.bob, f.carol} {
		keys, err := f.run(user, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range keys {
			if strings.HasPrefix(k, "PRIV") {
				t.Fatalf("shared filter leaked %s", k)
			}
		}
	}
	if keys, _ := f.run(f.bob, id); len(keys) != 5 {
		t.Fatalf("bob should get the public project only: %v", keys)
	}
	if keys, _ := f.run(f.carol, id); len(keys) != 0 {
		t.Fatalf("a guest sees nothing: %v", keys)
	}
}

func TestFilterListScopesAndSearch(t *testing.T) {
	f := newFilterEnv(t)
	mine1 := f.create(f.alice, "Alfa propio", "labels = bug")
	f.create(f.alice, "Beta propio", "")
	fromBob := f.create(f.bob, "De bob compartido", "")
	f.create(f.bob, "De bob privado", "")
	f.share(f.bob, fromBob, domain.Share{Type: domain.ShareUser, TargetID: f.alice})
	f.exec(`INSERT INTO favorites (user_id, entity_type, entity_id) VALUES ($1, 'filter', $2)`, f.alice, fromBob)

	names := func(q port.FilterListQuery) ([]string, int) {
		views, total, err := f.svc.List(f.ctx, f.alice, q)
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, v := range views {
			out = append(out, v.Name)
		}
		return out, total
	}
	if got, total := names(port.FilterListQuery{}); strings.Join(got, "|") != "Alfa propio|Beta propio|De bob compartido" || total != 3 {
		t.Fatalf("all: %v %d", got, total)
	}
	if got, _ := names(port.FilterListQuery{Scope: port.ScopeMine}); strings.Join(got, "|") != "Alfa propio|Beta propio" {
		t.Fatalf("mine: %v", got)
	}
	if got, _ := names(port.FilterListQuery{Scope: port.ScopeShared}); strings.Join(got, "|") != "De bob compartido" {
		t.Fatalf("shared: %v", got)
	}
	if got, _ := names(port.FilterListQuery{Scope: port.ScopeFavorite}); strings.Join(got, "|") != "De bob compartido" {
		t.Fatalf("favorite: %v", got)
	}
	if got, _ := names(port.FilterListQuery{Text: "BETA"}); strings.Join(got, "|") != "Beta propio" {
		t.Fatalf("text: %v", got)
	}
	if got, _ := names(port.FilterListQuery{Text: "%"}); len(got) != 0 {
		t.Fatalf("wildcards are literal: %v", got)
	}
	got, total := names(port.FilterListQuery{PageSize: 2, Page: 2})
	if len(got) != 1 || total != 3 {
		t.Fatalf("page 2: %v %d", got, total)
	}
	views, _, _ := f.svc.List(f.ctx, f.alice, port.FilterListQuery{Scope: port.ScopeFavorite})
	if len(views) != 1 || !views[0].IsFavorite || views[0].Access != domain.AccessView || views[0].OwnerName != "bob" {
		t.Fatalf("favorite view: %+v", views)
	}
	if _, _, err := f.svc.List(f.ctx, f.alice, port.FilterListQuery{Scope: "nope"}); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("bad scope: %v", err)
	}
	// Strangers see nothing of it.
	if views, total, _ := f.svc.List(f.ctx, f.dave, port.FilterListQuery{}); len(views) != 0 || total != 0 {
		t.Fatalf("dave: %v", views)
	}
	_ = mine1
	// Updating and reading back keeps the details.
	desc := "con descripcion"
	cols := []string{"key"}
	d, err := f.svc.Update(f.ctx, f.viewer(f.alice), mine1, port.FilterPatch{Description: &desc, Columns: &cols})
	if err != nil || d.Description != desc || len(d.Columns) != 1 || d.QueryText != "labels = bug" {
		t.Fatalf("update: %v %+v", err, d)
	}
	dup := "Beta propio"
	if _, err := f.svc.Update(f.ctx, f.viewer(f.alice), mine1, port.FilterPatch{Name: &dup}); apperror.CodeOf(err) != domain.CodeFilterNameTaken {
		t.Fatalf("rename clash: %v", err)
	}
}

func TestFilterSubscriptionsAndDueLogic(t *testing.T) {
	f := newFilterEnv(t)
	id := f.create(f.alice, "Digest", "project = PUB")
	f.share(f.alice, id, domain.Share{Type: domain.ShareUser, TargetID: f.bob})

	// Subscribing needs visibility, a valid schedule, and defaults to the user's time zone.
	if _, err := f.svc.Subscribe(f.ctx, f.viewer(f.carol), id, domain.Schedule{Frequency: domain.FrequencyDaily, Hour: 9}); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("hidden filter: %v", err)
	}
	for _, bad := range []domain.Schedule{
		{Frequency: "hourly"}, {Frequency: domain.FrequencyDaily, Hour: 24}, {Frequency: domain.FrequencyWeekly, Weekday: 7},
		{Frequency: domain.FrequencyDaily, Timezone: "Mars/Olympus"},
	} {
		if _, err := f.svc.Subscribe(f.ctx, f.viewer(f.alice), id, bad); apperror.CodeOf(err) != domain.CodeSubscriptionInvalid {
			t.Fatalf("%+v accepted: %v", bad, err)
		}
	}
	sub, err := f.svc.Subscribe(f.ctx, f.viewer(f.alice), id, domain.Schedule{Frequency: domain.FrequencyDaily, Hour: 9})
	if err != nil || sub.Schedule.Timezone != "UTC" || sub.LastSentAt != nil {
		t.Fatalf("subscribe: %v %+v", err, sub)
	}
	bobSub, err := f.svc.Subscribe(f.ctx, f.viewer(f.bob), id, domain.Schedule{Frequency: domain.FrequencyWeekly, Hour: 8, Weekday: 1, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := f.svc.Get(f.ctx, f.alice, id)
	if !d.IsFavorite && (!d.HasSubscription || d.Subscription == nil || d.Subscription.ID != sub.ID) {
		t.Fatalf("detail subscription: %+v", d)
	}

	due := func() map[uuid.UUID]bool {
		list, err := f.svc.DueSubscriptions(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := map[uuid.UUID]bool{}
		for _, item := range list {
			out[item.Subscription.UserID] = true
			if item.Filter.ID != id || item.Filter.Name != "Digest" {
				t.Fatalf("due item filter: %+v", item.Filter)
			}
		}
		return out
	}
	// testNow is Friday 15:30 and both were created now: nothing is due until the next slot.
	if got := due(); len(got) != 0 {
		t.Fatalf("a new subscription must wait for its slot: %v", got)
	}
	f.clock.T = time.Date(2026, 11, 28, 9, 5, 0, 0, time.UTC) // Saturday 09:05: alice's daily slot passed
	if got := due(); !got[f.alice] || got[f.bob] || len(got) != 1 {
		t.Fatalf("saturday: %v", got)
	}
	// Delivering clears it until the next slot.
	if err := f.svc.MarkDelivered(f.ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	if got := due(); len(got) != 0 {
		t.Fatalf("after delivery: %v", got)
	}
	f.clock.T = time.Date(2026, 11, 29, 9, 0, 0, 0, time.UTC) // Sunday 09:00
	if got := due(); !got[f.alice] || got[f.bob] {
		t.Fatalf("sunday: %v", got)
	}
	f.clock.T = time.Date(2026, 11, 30, 8, 0, 0, 0, time.UTC) // Monday 08:00: bob's weekly slot
	if got := due(); !got[f.bob] || !got[f.alice] {
		t.Fatalf("monday: %v", got)
	}
	// A subscriber who lost access is skipped.
	f.exec(`DELETE FROM organization_members WHERE user_id = $1`, f.bob)
	if got := due(); got[f.bob] {
		t.Fatalf("lost access: %v", got)
	}
	_ = bobSub
	if err := f.svc.MarkDelivered(f.ctx, uuid.New()); !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Fatalf("unknown subscription: %v", err)
	}

	// Re-subscribing restarts the baseline; unsubscribing removes it.
	if _, err := f.svc.Subscribe(f.ctx, f.viewer(f.alice), id, domain.Schedule{Frequency: domain.FrequencyDaily, Hour: 7}); err != nil {
		t.Fatal(err)
	}
	if got := due(); got[f.alice] {
		t.Fatalf("resubscribing restarts the baseline: %v", got)
	}
	if err := f.svc.Unsubscribe(f.ctx, f.alice, id); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Unsubscribe(f.ctx, f.alice, id); !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Fatalf("second unsubscribe: %v", err)
	}
	if err := f.svc.Unsubscribe(f.ctx, f.dave, id); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("hidden unsubscribe: %v", err)
	}
}

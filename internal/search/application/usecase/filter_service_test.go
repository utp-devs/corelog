package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

// memFilterRepo is an in-memory FilterRepository. Access is decided by the owner and a map of shared
// permissions per viewer, which is all the use case relies on.
type memFilterRepo struct {
	filters map[uuid.UUID]*domain.Filter
	shared  map[uuid.UUID]map[uuid.UUID]domain.Permission // filter -> viewer -> permission
	shares  map[uuid.UUID][]domain.Share
	subs    map[uuid.UUID]*domain.Subscription // keyed by filter id (one subscriber per filter in these tests)
	orgFor  uuid.UUID
	failOn  string
}

func newMemRepo() *memFilterRepo {
	return &memFilterRepo{
		filters: map[uuid.UUID]*domain.Filter{}, shared: map[uuid.UUID]map[uuid.UUID]domain.Permission{},
		shares: map[uuid.UUID][]domain.Share{}, subs: map[uuid.UUID]*domain.Subscription{}, orgFor: uuid.New(),
	}
}

func (m *memFilterRepo) fail(op string) error {
	if m.failOn == op {
		return errors.New("boom " + op)
	}
	return nil
}

func (m *memFilterRepo) OrganizationFor(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return m.orgFor, m.fail("org")
}

func (m *memFilterRepo) Create(_ context.Context, owner, org uuid.UUID, f domain.FilterFields, now time.Time) (uuid.UUID, error) {
	if err := m.fail("create"); err != nil {
		return uuid.Nil, err
	}
	for _, x := range m.filters {
		if x.OwnerID == owner && strings.EqualFold(x.Name, f.Name) {
			return uuid.Nil, domain.ErrFilterNameTaken
		}
	}
	id := uuid.New()
	m.filters[id] = &domain.Filter{ID: id, OwnerID: owner, OrganizationID: org, Name: f.Name, Description: f.Description, QueryText: f.QueryText, Columns: f.Columns, Sort: f.Sort, CreatedAt: now, UpdatedAt: now}
	return id, nil
}

func (m *memFilterRepo) Get(_ context.Context, viewer, id uuid.UUID) (*port.FilterView, error) {
	if err := m.fail("get"); err != nil {
		return nil, err
	}
	f, ok := m.filters[id]
	if !ok {
		return nil, domain.ErrFilterNotFound
	}
	access := domain.ResolveAccess(f.OwnerID == viewer, m.shared[id][viewer])
	if !access.CanView() {
		return nil, domain.ErrFilterNotFound
	}
	s, subscribed := m.subs[id]
	return &port.FilterView{Filter: *f, Access: access, HasSubscription: subscribed && s.UserID == viewer}, nil
}

func (m *memFilterRepo) List(_ context.Context, viewer uuid.UUID, _ port.FilterListQuery) ([]port.FilterView, int, error) {
	var out []port.FilterView
	for id := range m.filters {
		if v, err := m.Get(context.Background(), viewer, id); err == nil {
			out = append(out, *v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, len(out), m.fail("list")
}

func (m *memFilterRepo) Update(_ context.Context, id uuid.UUID, f domain.FilterFields, now time.Time) error {
	if err := m.fail("update"); err != nil {
		return err
	}
	x := m.filters[id]
	x.Name, x.Description, x.QueryText, x.Columns, x.Sort, x.UpdatedAt = f.Name, f.Description, f.QueryText, f.Columns, f.Sort, now
	return nil
}

func (m *memFilterRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.filters, id)
	return m.fail("delete")
}

func (m *memFilterRepo) Shares(_ context.Context, id uuid.UUID) ([]port.ShareView, error) {
	var out []port.ShareView
	for _, s := range m.shares[id] {
		out = append(out, port.ShareView{Share: s})
	}
	return out, m.fail("shares")
}

func (m *memFilterRepo) ReplaceShares(_ context.Context, id, _ uuid.UUID, shares []domain.Share, _ time.Time) error {
	if err := m.fail("replace"); err != nil {
		return err
	}
	m.shares[id] = shares
	return nil
}

func (m *memFilterRepo) GetSubscription(_ context.Context, id, user uuid.UUID) (*domain.Subscription, error) {
	if err := m.fail("getsub"); err != nil {
		return nil, err
	}
	if s, ok := m.subs[id]; ok && s.UserID == user {
		return s, nil
	}
	return nil, nil
}

func (m *memFilterRepo) UpsertSubscription(_ context.Context, id, user uuid.UUID, sch domain.Schedule, now time.Time) (domain.Subscription, error) {
	s := &domain.Subscription{ID: uuid.New(), FilterID: id, UserID: user, Schedule: sch, CreatedAt: now}
	m.subs[id] = s
	return *s, m.fail("upsert")
}

func (m *memFilterRepo) DeleteSubscription(_ context.Context, id, user uuid.UUID) error {
	if s, ok := m.subs[id]; !ok || s.UserID != user {
		return domain.ErrSubscriptionNotFound
	}
	delete(m.subs, id)
	return nil
}

func (m *memFilterRepo) MarkSubscriptionSent(_ context.Context, id uuid.UUID, at time.Time) error {
	for _, s := range m.subs {
		if s.ID == id {
			s.LastSentAt = &at
			return nil
		}
	}
	return domain.ErrSubscriptionNotFound
}

func (m *memFilterRepo) ListSubscriptions(_ context.Context, after uuid.UUID, limit int) ([]domain.Subscription, error) {
	if err := m.fail("listsubs"); err != nil {
		return nil, err
	}
	var all []domain.Subscription
	for _, s := range m.subs {
		if strings.Compare(s.ID.String(), after.String()) > 0 {
			all = append(all, *s)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID.String() < all[j].ID.String() })
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func newFilterSvc() (*FilterService, *memFilterRepo, *clock.FakeClock, *fakeSearcher) {
	repo := newMemRepo()
	clk := &clock.FakeClock{T: time.Date(2026, 11, 27, 15, 30, 0, 0, time.UTC)}
	searcher := &fakeSearcher{}
	jql := NewJQLService(searcher, nil, fakeViewers{tz: "America/Bogota"}, clk)
	return NewFilterService(repo, jql, clk), repo, clk, searcher
}

var (
	ownerV = port.Viewer{UserID: uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")}
	otherV = port.Viewer{UserID: uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")}
)

func TestFilterServiceLifecycle(t *testing.T) {
	svc, repo, clk, searcher := newFilterSvc()
	ctx := context.Background()
	d, err := svc.Create(ctx, ownerV, uuid.Nil, domain.FilterFields{Name: " Mi filtro ", QueryText: "project = a", Sort: "key DESC"})
	if err != nil || d.Name != "Mi filtro" || d.Access != domain.AccessOwner || d.OrganizationID != repo.orgFor || !searcher.last.DryRun {
		t.Fatalf("%v %+v", err, d)
	}
	// Invalid input never reaches the repository.
	for _, bad := range []domain.FilterFields{{Name: ""}, {Name: "x", QueryText: "projct ="}, {Name: "x", Sort: "nosuch"}} {
		if _, err := svc.Create(ctx, ownerV, uuid.Nil, bad); apperror.KindOf(err) != apperror.KindValidation {
			t.Errorf("%+v accepted: %v", bad, err)
		}
	}
	if len(repo.filters) != 1 {
		t.Fatalf("invalid filters were stored: %d", len(repo.filters))
	}
	if _, err := svc.Create(ctx, ownerV, uuid.Nil, domain.FilterFields{Name: "mi filtro"}); !errors.Is(err, domain.ErrFilterNameTaken) {
		t.Fatalf("duplicate: %v", err)
	}
	// Run uses the stored query and order.
	if _, err := svc.Run(ctx, ownerV, d.ID, 2, 10, true); err != nil {
		t.Fatal(err)
	}
	if searcher.last.Query.OrderBy[0].Field.Name != "key" || searcher.last.Page != 2 || !searcher.last.WithTotal || searcher.last.Viewer.UserID != ownerV.UserID {
		t.Fatalf("run query: %+v", searcher.last)
	}
	if _, err := svc.Run(ctx, otherV, d.ID, 1, 10, true); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("hidden run: %v", err)
	}

	// Update by the owner; an invalid new query is rejected; unknown ids are not found.
	clk.Advance(time.Hour)
	name := "Nuevo"
	q := "labels = bug"
	u, err := svc.Update(ctx, ownerV, d.ID, port.FilterPatch{Name: &name, QueryText: &q})
	if err != nil || u.Name != "Nuevo" || u.QueryText != "labels = bug" || u.Sort != "key DESC" || !u.UpdatedAt.Equal(clk.T) {
		t.Fatalf("%v %+v", err, u)
	}
	desc, cols, sortV := "d", []string{"key"}, ""
	if u, err = svc.Update(ctx, ownerV, d.ID, port.FilterPatch{Description: &desc, Columns: &cols, Sort: &sortV}); err != nil || u.Description != "d" || u.Sort != "" || len(u.Columns) != 1 {
		t.Fatalf("%v %+v", err, u)
	}
	badQ := "projct ="
	if _, err := svc.Update(ctx, ownerV, d.ID, port.FilterPatch{QueryText: &badQ}); err == nil {
		t.Fatal("invalid update accepted")
	}
	emptyName := ""
	if _, err := svc.Update(ctx, ownerV, d.ID, port.FilterPatch{Name: &emptyName}); apperror.CodeOf(err) != domain.CodeFilterInvalid {
		t.Fatalf("empty name: %v", err)
	}
	if _, err := svc.Update(ctx, ownerV, uuid.New(), port.FilterPatch{}); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("unknown: %v", err)
	}

	// Lists.
	views, total, err := svc.List(ctx, ownerV.UserID, port.FilterListQuery{})
	if err != nil || total != 1 || len(views) != 1 {
		t.Fatalf("%v %v", err, views)
	}
	for _, scope := range []port.FilterScope{port.ScopeMine, port.ScopeShared, port.ScopeFavorite, port.ScopeAll, ""} {
		if _, _, err := svc.List(ctx, ownerV.UserID, port.FilterListQuery{Scope: scope, PageSize: 1000, Page: -1}); err != nil {
			t.Errorf("scope %q: %v", scope, err)
		}
	}
	if _, _, err := svc.List(ctx, ownerV.UserID, port.FilterListQuery{Scope: "x"}); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("bad scope: %v", err)
	}

	// Delete: owner only.
	if err := svc.Delete(ctx, otherV.UserID, d.ID); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("hidden delete: %v", err)
	}
	if err := svc.Delete(ctx, ownerV.UserID, d.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, ownerV.UserID, d.ID); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestFilterServicePermissions(t *testing.T) {
	svc, repo, _, _ := newFilterSvc()
	ctx := context.Background()
	d, _ := svc.Create(ctx, ownerV, uuid.Nil, domain.FilterFields{Name: "F"})
	repo.shared[d.ID] = map[uuid.UUID]domain.Permission{otherV.UserID: domain.PermissionView}
	name := "x"
	if _, err := svc.Update(ctx, otherV, d.ID, port.FilterPatch{Name: &name}); !errors.Is(err, domain.ErrFilterForbidden) {
		t.Fatalf("viewer update: %v", err)
	}
	if err := svc.Delete(ctx, otherV.UserID, d.ID); !errors.Is(err, domain.ErrFilterForbidden) {
		t.Fatalf("viewer delete: %v", err)
	}
	if _, err := svc.SetShares(ctx, otherV.UserID, d.ID, nil); !errors.Is(err, domain.ErrFilterForbidden) {
		t.Fatalf("viewer share: %v", err)
	}
	repo.shared[d.ID][otherV.UserID] = domain.PermissionEdit
	if _, err := svc.Update(ctx, otherV, d.ID, port.FilterPatch{Name: &name}); err != nil {
		t.Fatalf("editor update: %v", err)
	}
	if err := svc.Delete(ctx, otherV.UserID, d.ID); !errors.Is(err, domain.ErrFilterForbidden) {
		t.Fatalf("editor delete: %v", err)
	}
	// Only the owner receives the shares.
	target := uuid.New()
	views, err := svc.SetShares(ctx, ownerV.UserID, d.ID, []domain.Share{{Type: domain.ShareUser, TargetID: target}})
	if err != nil || len(views) != 1 || views[0].Permission != domain.PermissionView {
		t.Fatalf("%v %+v", err, views)
	}
	if got, _ := svc.Get(ctx, ownerV.UserID, d.ID); len(got.Shares) != 1 {
		t.Fatalf("owner shares: %+v", got.Shares)
	}
	if got, _ := svc.Get(ctx, otherV.UserID, d.ID); len(got.Shares) != 0 {
		t.Fatalf("editor shares: %+v", got.Shares)
	}
	if _, err := svc.SetShares(ctx, ownerV.UserID, d.ID, []domain.Share{{Type: "x", TargetID: target}}); apperror.CodeOf(err) != domain.CodeShareInvalid {
		t.Fatalf("invalid share: %v", err)
	}
}

func TestFilterServiceSubscriptionsAndDue(t *testing.T) {
	svc, repo, clk, _ := newFilterSvc()
	ctx := context.Background()
	d, _ := svc.Create(ctx, ownerV, uuid.Nil, domain.FilterFields{Name: "F"})
	repo.shared[d.ID] = map[uuid.UUID]domain.Permission{otherV.UserID: domain.PermissionView}

	if _, err := svc.Subscribe(ctx, otherV, uuid.New(), domain.Schedule{Frequency: domain.FrequencyDaily}); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("unknown filter: %v", err)
	}
	if _, err := svc.Subscribe(ctx, ownerV, d.ID, domain.Schedule{Frequency: "x"}); apperror.CodeOf(err) != domain.CodeSubscriptionInvalid {
		t.Fatalf("invalid: %v", err)
	}
	sub, err := svc.Subscribe(ctx, ownerV, d.ID, domain.Schedule{Frequency: domain.FrequencyDaily, Hour: 9})
	if err != nil || sub.Schedule.Timezone != "America/Bogota" {
		t.Fatalf("the user's time zone is the default: %v %+v", err, sub)
	}
	if got, _ := svc.Get(ctx, ownerV.UserID, d.ID); got.Subscription == nil || got.Subscription.ID != sub.ID {
		t.Fatalf("detail: %+v", got)
	}
	if due, err := svc.DueSubscriptions(ctx); err != nil || len(due) != 0 {
		t.Fatalf("new subscription is not due: %v %v", due, err)
	}
	clk.T = time.Date(2026, 11, 28, 15, 0, 0, 0, time.UTC) // after 09:00 Bogota (14:00 UTC)
	due, err := svc.DueSubscriptions(ctx)
	if err != nil || len(due) != 1 || due[0].Filter.ID != d.ID || due[0].Subscription.ID != sub.ID {
		t.Fatalf("%v %+v", err, due)
	}
	if err := svc.MarkDelivered(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	if due, _ := svc.DueSubscriptions(ctx); len(due) != 0 {
		t.Fatalf("delivered: %v", due)
	}
	if err := svc.MarkDelivered(ctx, uuid.New()); !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	// A subscriber who lost access is skipped.
	repo.subs[d.ID] = &domain.Subscription{ID: uuid.New(), FilterID: d.ID, UserID: otherV.UserID,
		Schedule: domain.Schedule{Frequency: domain.FrequencyDaily, Hour: 9, Timezone: "UTC"}, CreatedAt: clk.T.Add(-72 * time.Hour)}
	repo.shared[d.ID] = nil
	if due, err := svc.DueSubscriptions(ctx); err != nil || len(due) != 0 {
		t.Fatalf("lost access: %v %v", due, err)
	}
	if err := svc.Unsubscribe(ctx, otherV.UserID, d.ID); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("hidden unsubscribe: %v", err)
	}
	if err := svc.Unsubscribe(ctx, ownerV.UserID, d.ID); !errors.Is(err, domain.ErrSubscriptionNotFound) {
		t.Fatalf("not the subscriber: %v", err)
	}
}

func TestFilterServiceDuePagesThroughSubscriptions(t *testing.T) {
	svc, repo, clk, _ := newFilterSvc()
	ctx := context.Background()
	base := clk.T.Add(-72 * time.Hour)
	for i := 0; i < 501; i++ {
		fid := uuid.New()
		repo.filters[fid] = &domain.Filter{ID: fid, OwnerID: ownerV.UserID, Name: "n" + fid.String()}
		repo.subs[fid] = &domain.Subscription{ID: uuid.New(), FilterID: fid, UserID: ownerV.UserID,
			Schedule: domain.Schedule{Frequency: domain.FrequencyDaily, Hour: 9, Timezone: "UTC"}, CreatedAt: base}
	}
	due, err := svc.DueSubscriptions(ctx)
	if err != nil || len(due) != 501 {
		t.Fatalf("%v %d", err, len(due))
	}
}

func TestFilterServiceInfrastructureErrors(t *testing.T) {
	svc, repo, _, _ := newFilterSvc()
	ctx := context.Background()
	d, _ := svc.Create(ctx, ownerV, uuid.Nil, domain.FilterFields{Name: "F"})
	if _, err := svc.Subscribe(ctx, ownerV, d.ID, domain.Schedule{Frequency: domain.FrequencyDaily}); err != nil {
		t.Fatal(err)
	}
	name := "x"
	calls := map[string]func() error{
		"org":     func() error { _, err := svc.Create(ctx, ownerV, uuid.Nil, domain.FilterFields{Name: "n"}); return err },
		"create":  func() error { _, err := svc.Create(ctx, ownerV, uuid.Nil, domain.FilterFields{Name: "n"}); return err },
		"get":     func() error { _, err := svc.Get(ctx, ownerV.UserID, d.ID); return err },
		"shares":  func() error { _, err := svc.Get(ctx, ownerV.UserID, d.ID); return err },
		"getsub":  func() error { _, err := svc.Get(ctx, ownerV.UserID, d.ID); return err },
		"list":    func() error { _, _, err := svc.List(ctx, ownerV.UserID, port.FilterListQuery{}); return err },
		"update":  func() error { _, err := svc.Update(ctx, ownerV, d.ID, port.FilterPatch{Name: &name}); return err },
		"replace": func() error { _, err := svc.SetShares(ctx, ownerV.UserID, d.ID, nil); return err },
		"upsert": func() error {
			_, err := svc.Subscribe(ctx, ownerV, d.ID, domain.Schedule{Frequency: domain.FrequencyDaily})
			return err
		},
		"listsubs": func() error { _, err := svc.DueSubscriptions(ctx); return err },
	}
	for op, call := range calls {
		repo.failOn = op
		if err := call(); err == nil || !strings.Contains(err.Error(), "boom") {
			t.Errorf("%s: %v", op, err)
		}
	}
	repo.failOn = "delete"
	if err := svc.Delete(ctx, ownerV.UserID, d.ID); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("delete: %v", err)
	}
	repo.failOn = ""
	// Time zone lookup failures surface when subscribing.
	svc2 := NewFilterService(repo, NewJQLService(&fakeSearcher{}, nil, fakeViewers{err: errors.New("tz")}, nil), nil)
	if _, err := svc2.Subscribe(ctx, ownerV, d.ID, domain.Schedule{Frequency: domain.FrequencyDaily}); err == nil {
		t.Fatal("expected the time zone error")
	}
}

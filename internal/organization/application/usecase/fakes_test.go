package usecase

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

var fixedNow = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

type memberKey struct {
	org  uuid.UUID
	user uuid.UUID
}

// fakeRepo is an in-memory port.Repository. It assigns sequential IDs on Create.
type fakeRepo struct {
	nextID  int
	orgs    map[uuid.UUID]*domain.Organization
	members map[memberKey]*domain.Member
	users   map[uuid.UUID]fakeUser
	invites map[uuid.UUID]*domain.Invitation
}

// fakeUser is the account data the member read model needs.
type fakeUser struct {
	name  string
	email string
}

var _ port.Repository = (*fakeRepo)(nil)

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		orgs:    map[uuid.UUID]*domain.Organization{},
		members: map[memberKey]*domain.Member{},
		users:   map[uuid.UUID]fakeUser{},
		invites: map[uuid.UUID]*domain.Invitation{},
	}
}

func (r *fakeRepo) Create(_ context.Context, org *domain.Organization) error {
	for _, o := range r.orgs {
		if o.Slug == org.Slug {
			return domain.ErrSlugTaken
		}
	}
	r.nextID++
	org.ID = uuid.UUID{15: byte(r.nextID)}
	stored := *org
	r.orgs[org.ID] = &stored
	return nil
}

func (r *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Organization, error) {
	o, ok := r.orgs[id]
	if !ok || o.IsDeleted() {
		return nil, domain.ErrNotFound
	}
	c := *o
	return &c, nil
}

func (r *fakeRepo) GetBySlug(_ context.Context, slug string) (*domain.Organization, error) {
	for _, o := range r.orgs {
		if o.Slug == slug {
			c := *o
			return &c, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]port.Membership, error) {
	var out []port.Membership
	for k, m := range r.members {
		if k.user != userID {
			continue
		}
		o := r.orgs[k.org]
		if o.IsDeleted() {
			continue
		}
		c := *o
		out = append(out, port.Membership{Organization: &c, Role: m.Role})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Organization.Slug < out[j].Organization.Slug })
	return out, nil
}

func (r *fakeRepo) Update(_ context.Context, org *domain.Organization) error {
	if _, ok := r.orgs[org.ID]; !ok {
		return domain.ErrNotFound
	}
	stored := *org
	r.orgs[org.ID] = &stored
	return nil
}

func (r *fakeRepo) SoftDelete(_ context.Context, id uuid.UUID, at time.Time) error {
	o, ok := r.orgs[id]
	if !ok {
		return domain.ErrNotFound
	}
	o.MarkDeleted(at)
	return nil
}

func (r *fakeRepo) ListMembers(_ context.Context, orgID uuid.UUID) ([]*domain.Member, error) {
	var out []*domain.Member
	for k, m := range r.members {
		if k.org == orgID {
			c := *m
			out = append(out, &c)
		}
	}
	return out, nil
}

func (r *fakeRepo) GetMember(_ context.Context, orgID, userID uuid.UUID) (*domain.Member, error) {
	m, ok := r.members[memberKey{orgID, userID}]
	if !ok {
		return nil, domain.ErrMemberNotFound
	}
	c := *m
	return &c, nil
}

func (r *fakeRepo) AddMember(_ context.Context, m *domain.Member) error {
	k := memberKey{m.OrganizationID, m.UserID}
	if _, ok := r.members[k]; ok {
		return domain.ErrMemberExists
	}
	c := *m
	r.members[k] = &c
	return nil
}

func (r *fakeRepo) UpdateMemberRole(_ context.Context, orgID, userID uuid.UUID, role domain.Role) error {
	m, ok := r.members[memberKey{orgID, userID}]
	if !ok {
		return domain.ErrMemberNotFound
	}
	m.Role = role
	return nil
}

func (r *fakeRepo) RemoveMember(_ context.Context, orgID, userID uuid.UUID) error {
	k := memberKey{orgID, userID}
	if _, ok := r.members[k]; !ok {
		return domain.ErrMemberNotFound
	}
	delete(r.members, k)
	return nil
}

func (r *fakeRepo) CountOwners(_ context.Context, orgID uuid.UUID) (int, error) {
	n := 0
	for k, m := range r.members {
		if k.org == orgID && m.Role == domain.RoleOwner {
			n++
		}
	}
	return n, nil
}

// failingPublisher fails every publish, to check that errors inside the transaction surface.
type failingPublisher struct{ err error }

func (p failingPublisher) Publish(context.Context, events.Event) error { return p.err }

// fixture bundles a service with its fakes and a fixed set of users.
type fixture struct {
	svc   *OrganizationService
	repo  *fakeRepo
	pub   *events.MemoryPublisher
	tx    *database.FakeTxManager
	clk   *clock.FakeClock
	org   uuid.UUID
	slug  string
	owner uuid.UUID
	admin uuid.UUID
	membr uuid.UUID
	guest uuid.UUID
	out   uuid.UUID
}

// Account emails of the fixture users. Invitations for them are created with these addresses.
const (
	ownerEmail = "owner@acme.test"
	adminEmail = "admin@acme.test"
	membrEmail = "member@acme.test"
	guestEmail = "guest@acme.test"
	outEmail   = "outsider@other.test"
)

// newFixture creates the organization "acme" with one user per role and an outsider.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		repo:  newFakeRepo(),
		pub:   events.NewMemoryPublisher(),
		tx:    &database.FakeTxManager{},
		owner: uuid.New(),
		admin: uuid.New(),
		membr: uuid.New(),
		guest: uuid.New(),
		out:   uuid.New(),
		slug:  "acme",
	}
	f.clk = &clock.FakeClock{T: fixedNow}
	f.svc = NewOrganizationService(f.repo, f.tx, f.pub, f.clk)
	f.repo.users[f.owner] = fakeUser{name: "Ana Owner", email: ownerEmail}
	f.repo.users[f.admin] = fakeUser{name: "Ana Admin", email: adminEmail}
	f.repo.users[f.membr] = fakeUser{name: "Mario Member", email: membrEmail}
	f.repo.users[f.guest] = fakeUser{name: "Gina Guest", email: guestEmail}
	f.repo.users[f.out] = fakeUser{name: "Olga Outsider", email: outEmail}
	org, err := f.svc.CreateOrganization(context.Background(), f.owner, port.CreateInput{Name: "Acme", Slug: f.slug})
	if err != nil {
		t.Fatalf("setup create: %v", err)
	}
	f.org = org.ID
	f.repo.members[memberKey{f.org, f.admin}] = &domain.Member{OrganizationID: f.org, UserID: f.admin, Role: domain.RoleAdmin, JoinedAt: fixedNow}
	f.repo.members[memberKey{f.org, f.membr}] = &domain.Member{OrganizationID: f.org, UserID: f.membr, Role: domain.RoleMember, JoinedAt: fixedNow}
	f.repo.members[memberKey{f.org, f.guest}] = &domain.Member{OrganizationID: f.org, UserID: f.guest, Role: domain.RoleGuest, JoinedAt: fixedNow}
	// Start each test with an empty event log: setup events are discarded.
	f.pub = events.NewMemoryPublisher()
	f.svc.pub = f.pub
	return f
}

// eventTypes returns the published event types in order.
func eventTypes(pub *events.MemoryPublisher) []string {
	var out []string
	for _, e := range pub.Events() {
		out = append(out, e.Type)
	}
	return out
}

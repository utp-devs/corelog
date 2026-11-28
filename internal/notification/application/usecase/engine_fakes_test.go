package usecase

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
)

type memSchemes struct {
	mu       sync.Mutex
	stored   map[string]port.StoredScheme
	projects map[uuid.UUID]uuid.UUID
}

func newMemSchemes() *memSchemes {
	return &memSchemes{stored: map[string]port.StoredScheme{}, projects: map[uuid.UUID]uuid.UUID{}}
}

func scopeKey(org uuid.UUID, project *uuid.UUID) string {
	if project != nil {
		return "p:" + project.String()
	}
	return "o:" + org.String()
}

func (m *memSchemes) Get(_ context.Context, org uuid.UUID, project *uuid.UUID) (*port.StoredScheme, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.stored[scopeKey(org, project)]; ok {
		return &s, nil
	}
	return nil, nil
}

func (m *memSchemes) Save(_ context.Context, org uuid.UUID, project *uuid.UUID, s domain.Scheme, _ uuid.UUID, now time.Time) (port.StoredScheme, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored := port.StoredScheme{Scheme: s, UpdatedAt: now}
	m.stored[scopeKey(org, project)] = stored
	return stored, nil
}

func (m *memSchemes) Delete(_ context.Context, org uuid.UUID, project *uuid.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.stored[scopeKey(org, project)]
	delete(m.stored, scopeKey(org, project))
	return ok, nil
}

func (m *memSchemes) ProjectOrg(_ context.Context, project uuid.UUID) (uuid.UUID, error) {
	return m.projects[project], nil
}

type memFacts struct {
	issues   map[uuid.UUID]*port.IssueFacts
	projects map[uuid.UUID]*port.ProjectFacts
	teams    map[uuid.UUID][]uuid.UUID
	roles    map[string][]uuid.UUID
	contacts map[uuid.UUID]port.Contact
}

func newMemFacts() *memFacts {
	return &memFacts{
		issues: map[uuid.UUID]*port.IssueFacts{}, projects: map[uuid.UUID]*port.ProjectFacts{},
		teams: map[uuid.UUID][]uuid.UUID{}, roles: map[string][]uuid.UUID{}, contacts: map[uuid.UUID]port.Contact{},
	}
}

func (f *memFacts) person(name string) uuid.UUID {
	id := uuid.New()
	f.contacts[id] = port.Contact{ID: id, Name: name, Email: name + "@example.test", Active: true}
	return id
}

func (f *memFacts) Issue(_ context.Context, id uuid.UUID) (*port.IssueFacts, error) {
	return f.issues[id], nil
}
func (f *memFacts) Project(_ context.Context, id uuid.UUID) (*port.ProjectFacts, error) {
	return f.projects[id], nil
}
func (f *memFacts) TeamMembers(_ context.Context, ids []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out := map[uuid.UUID][]uuid.UUID{}
	for _, id := range ids {
		out[id] = f.teams[id]
	}
	return out, nil
}
func (f *memFacts) RoleMembers(_ context.Context, _ uuid.UUID, roles []string) (map[string][]uuid.UUID, error) {
	out := map[string][]uuid.UUID{}
	for _, r := range roles {
		out[r] = f.roles[r]
	}
	return out, nil
}
func (f *memFacts) Contacts(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]port.Contact, error) {
	out := map[uuid.UUID]port.Contact{}
	for _, id := range ids {
		if c, ok := f.contacts[id]; ok {
			out[id] = c
		}
	}
	return out, nil
}

type memDelivery struct {
	claims map[string]bool
	items  []port.DigestItem
	nextID int64
}

func newMemDelivery() *memDelivery { return &memDelivery{claims: map[string]bool{}} }

func (d *memDelivery) Claim(_ context.Context, key string, user uuid.UUID, _ time.Time) (bool, error) {
	k := key + "|" + user.String()
	if d.claims[k] {
		return false, nil
	}
	d.claims[k] = true
	return true, nil
}
func (d *memDelivery) AddDigestItem(_ context.Context, item port.DigestItem) error {
	d.nextID++
	item.ID = d.nextID
	d.items = append(d.items, item)
	return nil
}
func (d *memDelivery) UsersWithDigest(context.Context, int) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]bool{}
	var out []uuid.UUID
	for _, i := range d.items {
		if !seen[i.UserID] {
			seen[i.UserID] = true
			out = append(out, i.UserID)
		}
	}
	return out, nil
}
func (d *memDelivery) DigestItems(_ context.Context, user uuid.UUID, _ int) ([]port.DigestItem, error) {
	var out []port.DigestItem
	for _, i := range d.items {
		if i.UserID == user {
			out = append(out, i)
		}
	}
	return out, nil
}
func (d *memDelivery) MarkDigested(_ context.Context, ids []int64, _ time.Time) error {
	drop := map[int64]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	kept := d.items[:0:0]
	for _, i := range d.items {
		if !drop[i.ID] {
			kept = append(kept, i)
		}
	}
	d.items = kept
	return nil
}
func (d *memDelivery) PurgeDeliveries(context.Context, time.Time) (int64, error) { return 0, nil }

type memOutbox struct {
	mu      sync.Mutex
	entries []*port.OutboxEntry
	keys    map[string]bool
}

func newMemOutbox() *memOutbox { return &memOutbox{keys: map[string]bool{}} }

func (o *memOutbox) Enqueue(_ context.Context, msg port.EmailMessage, now time.Time) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if msg.IdempotencyKey != "" {
		if o.keys[msg.IdempotencyKey] {
			return false, nil
		}
		o.keys[msg.IdempotencyKey] = true
	}
	var user *uuid.UUID
	if msg.UserID != uuid.Nil {
		u := msg.UserID
		user = &u
	}
	o.entries = append(o.entries, &port.OutboxEntry{
		ID: uuid.New(), UserID: user, To: msg.To, Kind: msg.Kind, Subject: msg.Subject, Text: msg.Text, HTML: msg.HTML,
		Status: port.OutboxPending, NextAttemptAt: now, CreatedAt: now,
	})
	return true, nil
}

func (o *memOutbox) ClaimDue(_ context.Context, now time.Time, limit int, lease time.Duration) ([]port.OutboxEntry, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []port.OutboxEntry
	for _, e := range o.entries {
		if len(out) == limit {
			break
		}
		if e.Status == port.OutboxPending && !e.NextAttemptAt.After(now) {
			e.NextAttemptAt = now.Add(lease)
			out = append(out, *e)
		}
	}
	return out, nil
}

func (o *memOutbox) find(id uuid.UUID) *port.OutboxEntry {
	for _, e := range o.entries {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func (o *memOutbox) MarkSent(_ context.Context, id uuid.UUID, now time.Time) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	e := o.find(id)
	e.Status, e.SentAt = port.OutboxSent, &now
	return nil
}

func (o *memOutbox) MarkFailed(_ context.Context, id uuid.UUID, attempts int, next *time.Time, lastError string, now time.Time) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	e := o.find(id)
	e.Attempts, e.LastError = attempts, lastError
	if next == nil {
		e.Status = port.OutboxDead
	} else {
		e.NextAttemptAt = *next
	}
	return nil
}

func (o *memOutbox) List(_ context.Context, status string, limit, offset int) ([]port.OutboxEntry, int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []port.OutboxEntry
	for _, e := range o.entries {
		if status == "" || e.Status == status {
			out = append(out, *e)
		}
	}
	total := len(out)
	sort.SliceStable(out, func(a, b int) bool { return out[a].CreatedAt.After(out[b].CreatedAt) })
	if offset > len(out) {
		offset = len(out)
	}
	out = out[offset:]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, total, nil
}

func (o *memOutbox) Retry(_ context.Context, id uuid.UUID, now time.Time) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	e := o.find(id)
	if e == nil || e.Status != port.OutboxDead {
		return false, nil
	}
	e.Status, e.Attempts, e.NextAttemptAt = port.OutboxPending, 0, now
	return true, nil
}

func (o *memOutbox) pending() []*port.OutboxEntry {
	var out []*port.OutboxEntry
	for _, e := range o.entries {
		if e.Status == port.OutboxPending {
			out = append(out, e)
		}
	}
	return out
}

type fakeRenderer struct{}

func (fakeRenderer) Notice(e port.Email) port.Rendered {
	return port.Rendered{Subject: string(e.Event) + " " + e.IssueKey, Text: e.Link, HTML: "<p>" + e.Link + "</p>"}
}
func (fakeRenderer) Digest(e port.Email) port.Rendered {
	return port.Rendered{Subject: "digest", Text: "digest", HTML: "digest"}
}
func (fakeRenderer) FilterSubscription(e port.Email) port.Rendered {
	return port.Rendered{Subject: "filter " + e.FilterName, Text: "filter", HTML: "filter"}
}

type allowAll struct{ denied map[uuid.UUID]bool }

func (a allowAll) CanBrowse(_ context.Context, user, _ uuid.UUID) (bool, error) {
	return !a.denied[user], nil
}

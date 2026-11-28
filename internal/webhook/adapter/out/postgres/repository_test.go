package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/webhook/application/port"
	"github.com/jhonsferg/corelog/internal/webhook/domain"
)

const orgID = "00000000-0000-4000-8000-000000000002"

func TestRepositoryWebhooksAndDeliveries(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	org := uuid.MustParse(orgID)
	user, project := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'u', 'Ana Perez', 'u@example.test', 'x', 'user')`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'WHK', 'Hooks', 'kanban', 'kanban', $3, 'open')`, project, org, user); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	filter := "status = Open"
	hook := port.Stored{SecretEncrypted: []byte{1, 2, 3}, Webhook: domain.Webhook{OrgID: org, Name: "all", URL: "https://example.com/a",
		Events: []string{"issue.created", "issue.updated"}, FilterQuery: &filter, Enabled: true, CreatedBy: &user, CreatedAt: now, UpdatedAt: now}}
	if err := repo.Create(ctx, &hook); err != nil || hook.ID == uuid.Nil {
		t.Fatalf("create %v", err)
	}
	scoped := port.Stored{SecretEncrypted: []byte{4}, Webhook: domain.Webhook{OrgID: org, ProjectID: &project, Name: "project", URL: "https://example.com/b",
		Events: []string{"*"}, Enabled: true, CreatedAt: now, UpdatedAt: now}}
	if err := repo.Create(ctx, &scoped); err != nil {
		t.Fatal(err)
	}
	off := port.Stored{SecretEncrypted: []byte{5}, Webhook: domain.Webhook{OrgID: org, Name: "off", URL: "https://example.com/c", Events: []string{"*"}, CreatedAt: now, UpdatedAt: now}}
	if err := repo.Create(ctx, &off); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx, org, hook.ID)
	if err != nil || got.Name != "all" || len(got.Events) != 2 || got.FilterQuery == nil || string(got.SecretEncrypted) != "\x01\x02\x03" || got.CreatedBy == nil {
		t.Fatalf("get %+v %v", got, err)
	}
	if _, err := repo.Get(ctx, uuid.New(), hook.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-org get: %v", err)
	}
	if _, err := repo.StoredByID(ctx, hook.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.StoredByID(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	if all, _ := repo.List(ctx, org, nil); len(all) != 3 {
		t.Fatalf("list %d", len(all))
	}
	if byProject, _ := repo.List(ctx, org, &project); len(byProject) != 1 || byProject[0].Name != "project" {
		t.Fatalf("list by project %v", byProject)
	}
	if other, _ := repo.List(ctx, uuid.New(), nil); len(other) != 0 {
		t.Fatal("other organization sees webhooks")
	}
	if ok, _ := repo.ProjectInOrg(ctx, org, project); !ok {
		t.Fatal("project in org")
	}
	if ok, _ := repo.ProjectInOrg(ctx, uuid.New(), project); ok {
		t.Fatal("project in another org")
	}

	// Matching: enabled, subscribed and in scope.
	m, err := repo.Matching(ctx, org, &project, "issue.created")
	if err != nil || len(m) != 2 {
		t.Fatalf("matching in project: %v %v", m, err)
	}
	if m, _ = repo.Matching(ctx, org, &project, "sprint.started"); len(m) != 1 || m[0].Name != "project" {
		t.Fatalf("matching wildcard: %v", m)
	}
	otherProject := uuid.New()
	if m, _ = repo.Matching(ctx, org, &otherProject, "issue.created"); len(m) != 1 || m[0].Name != "all" {
		t.Fatalf("matching other project: %v", m)
	}

	hook.Name, hook.Enabled, hook.FilterQuery, hook.UpdatedAt = "renamed", false, nil, now.Add(time.Second)
	if err := repo.Update(ctx, hook.Webhook); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.Get(ctx, org, hook.ID); got.Name != "renamed" || got.Enabled || got.FilterQuery != nil {
		t.Fatalf("after update %+v", got)
	}
	if err := repo.Update(ctx, domain.Webhook{ID: uuid.New(), OrgID: org, Events: []string{}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("update unknown: %v", err)
	}
	if err := repo.UpdateSecret(ctx, org, hook.ID, []byte{9}, now); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.Get(ctx, org, hook.ID); string(got.SecretEncrypted) != "\x09" {
		t.Fatal("secret not rotated")
	}

	// Deliveries.
	event := int64(99)
	d := domain.Delivery{WebhookID: scoped.ID, EventID: &event, EventType: "issue.created", Payload: []byte(`{"a":1}`), Status: domain.StatusPending, NextAttemptAt: &now, CreatedAt: now}
	if ok, err := repo.InsertDelivery(ctx, &d); err != nil || !ok || d.ID == uuid.Nil {
		t.Fatalf("insert %v %v", ok, err)
	}
	dup := d
	dup.ID = uuid.Nil
	if ok, err := repo.InsertDelivery(ctx, &dup); err != nil || ok {
		t.Fatalf("duplicate event must be ignored: %v %v", ok, err)
	}
	ping := domain.Delivery{WebhookID: scoped.ID, EventType: "ping", Payload: []byte(`{}`), Status: domain.StatusPending, NextAttemptAt: &now, CreatedAt: now.Add(time.Second)}
	_, _ = repo.InsertDelivery(ctx, &ping)
	future := now.Add(time.Hour)
	later := domain.Delivery{WebhookID: scoped.ID, EventType: "ping", Payload: []byte(`{}`), Status: domain.StatusFailed, NextAttemptAt: &future, CreatedAt: now.Add(2 * time.Second)}
	_, _ = repo.InsertDelivery(ctx, &later)
	disabled := domain.Delivery{WebhookID: off.ID, EventType: "ping", Payload: []byte(`{}`), Status: domain.StatusPending, NextAttemptAt: &now, CreatedAt: now}
	_, _ = repo.InsertDelivery(ctx, &disabled)

	list, err := repo.ListDeliveries(ctx, scoped.ID, 10)
	if err != nil || len(list) != 3 || list[0].ID != later.ID {
		t.Fatalf("list %v %v", list, err)
	}
	if got, err := repo.GetDelivery(ctx, scoped.ID, d.ID); err != nil || got.EventID == nil || *got.EventID != 99 {
		t.Fatalf("get delivery %+v %v", got, err)
	}
	if _, err := repo.GetDelivery(ctx, hook.ID, d.ID); !errors.Is(err, domain.ErrDeliveryNotFound) {
		t.Fatalf("delivery of another webhook: %v", err)
	}

	due, err := repo.ClaimDue(ctx, now.Add(time.Second), 10, 2*time.Minute)
	if err != nil || len(due) != 2 || due[0].URL != "https://example.com/b" || string(due[0].SecretEncrypted) != "\x04" {
		t.Fatalf("claim %v %v", due, err)
	}
	if again, _ := repo.ClaimDue(ctx, now.Add(time.Second), 10, 2*time.Minute); len(again) != 0 {
		t.Fatalf("a leased delivery was claimed twice: %d", len(again))
	}
	code := 500
	next := now.Add(time.Minute)
	if err := repo.RecordResult(ctx, d.ID, port.Result{Status: domain.StatusFailed, Attempts: 1, NextAttemptAt: &next, ResponseStatus: &code, Excerpt: "oops", DurationMS: 12}); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetDelivery(ctx, scoped.ID, d.ID); got.Status != domain.StatusFailed || got.Attempts != 1 || *got.ResponseStatus != 500 || got.ResponseExcerpt != "oops" {
		t.Fatalf("after result %+v", got)
	}

	if err := repo.Delete(ctx, org, scoped.ID); err != nil {
		t.Fatal(err)
	}
	var kept, stamped int
	if err := db.Get(&kept, `SELECT count(*) FROM webhook_deliveries WHERE webhook_id = $1`, scoped.ID); err != nil || kept == 0 {
		t.Fatalf("deliveries must be kept for history: %d %v", kept, err)
	}
	if err := db.Get(&stamped, `SELECT count(*) FROM webhooks WHERE id = $1 AND deleted_at IS NOT NULL AND NOT enabled`, scoped.ID); err != nil || stamped != 1 {
		t.Fatalf("the row must be kept with deleted_at set: %d %v", stamped, err)
	}
	if _, err := repo.Get(ctx, org, scoped.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted webhook visible through Get: %v", err)
	}
	if list, _ := repo.List(ctx, org, nil); len(list) != 0 {
		for _, w := range list {
			if w.ID == scoped.ID {
				t.Fatal("deleted webhook listed")
			}
		}
	}
	if due, _ := repo.ClaimDue(ctx, now.Add(time.Hour), 10, time.Minute); len(due) != 0 {
		t.Fatalf("deliveries of a deleted webhook were claimed: %d", len(due))
	}
	if err := repo.Delete(ctx, org, scoped.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestDescriberResolvesProjectIssueAndActor(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	org := uuid.MustParse(orgID)
	user, project, issue := uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'u', 'Ana Perez', 'u@example.test', 'x', 'user')`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'DSC', 'Describe', 'kanban', 'kanban', $3, 'open')`, project, org, user); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`SELECT corelog_ensure_workflow($1)`, project); err != nil {
		t.Fatal(err)
	}
	var typeID, prioID, statusID uuid.UUID
	_ = db.Get(&typeID, `SELECT id FROM issue_types WHERE organization_id = $1 AND legacy_key = 'task'`, org)
	_ = db.Get(&prioID, `SELECT id FROM priorities WHERE organization_id = $1 AND legacy_key = 'high'`, org)
	_ = db.Get(&statusID, `SELECT id FROM statuses WHERE organization_id = $1 ORDER BY position LIMIT 1`, org)
	if _, err := db.Exec(`INSERT INTO issues (id, summary, description, status, priority, reporter_id, project_id, issue_type, rank, key_number, issue_type_id, priority_id, status_id)
		VALUES ($1, 'Titulo', 'd', 'open', 'high', $2, $3, 'task', 'm', 1, $4, $5, $6)`, issue, user, project, typeID, prioID, statusID); err != nil {
		t.Fatal(err)
	}
	d := NewDescriber(db)
	info, ok, err := d.Describe(ctx, events.Event{ProjectID: &project, EntityID: &issue, EntityType: "issue", ActorID: &user, Type: "issue.created"})
	if err != nil || !ok || info.OrgID != org || info.Project.Key != "DSC" || info.Issue == nil || info.Issue.Key != "DSC-1" || info.Issue.Summary != "Titulo" || info.ActorName != "Ana Perez" {
		t.Fatalf("issue event: %+v %v %v", info, ok, err)
	}
	// Comment events reference the issue in their payload.
	comment := uuid.New()
	info, ok, _ = d.Describe(ctx, events.Event{ProjectID: &project, EntityID: &comment, EntityType: "comment", Type: "comment.created", Payload: map[string]any{"issue_id": issue.String()}})
	if !ok || info.Issue == nil || info.Issue.ID != issue {
		t.Fatalf("comment event: %+v", info)
	}
	// Events with an organization only, with an unknown project and with nothing.
	if info, ok, _ = d.Describe(ctx, events.Event{OrgID: &org, Type: "version.released"}); !ok || info.OrgID != org || info.Project != nil {
		t.Fatalf("org event: %+v %v", info, ok)
	}
	missing := uuid.New()
	if _, ok, _ = d.Describe(ctx, events.Event{ProjectID: &missing}); ok {
		t.Fatal("unknown project")
	}
	if _, ok, _ = d.Describe(ctx, events.Event{}); ok {
		t.Fatal("event without scope")
	}
}

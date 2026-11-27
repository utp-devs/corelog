package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// insertFeedComment stores a comment with a fixed timestamp.
func insertFeedComment(t *testing.T, e v2Env, issueID uuid.UUID, at time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.db.QueryRow(`
		INSERT INTO comments (issue_id, author_id, body, created_at, updated_at)
		VALUES ($1, $2, 'cuerpo', $3, $3) RETURNING id`, issueID, e.lead, at).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// insertFeedActivity stores one activity row. batch may be uuid.Nil for a row without batch.
func insertFeedActivity(t *testing.T, e v2Env, issueID, batch uuid.UUID, field string, at time.Time) {
	t.Helper()
	var batchArg any
	if batch != uuid.Nil {
		batchArg = batch
	}
	if _, err := e.db.Exec(`
		INSERT INTO activities (issue_id, actor_id, action, field_name, old_value, new_value, created_at, batch_id)
		VALUES ($1, $2, 'field_changed', $3, 'antes', 'despues', $4, $5)`,
		issueID, e.lead, field, at, batchArg); err != nil {
		t.Fatal(err)
	}
}

func TestActivityFeedGroupsBatchesAndPagesWithEqualTimestamps(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	issue := e.newIssue(t, "Feed")
	reader := NewActivityFeedRepository(e.db)
	at := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)

	// One PATCH with three changes is one history entry; a standalone row is one entry of its own.
	batch := uuid.New()
	insertFeedActivity(t, e, issue.ID, batch, "summary", at)
	insertFeedActivity(t, e, issue.ID, batch, "description", at)
	insertFeedActivity(t, e, issue.ID, batch, "priority", at)
	insertFeedActivity(t, e, issue.ID, uuid.Nil, "assignee_id", at)
	insertFeedComment(t, e, issue.ID, at)
	insertFeedComment(t, e, issue.ID, at)
	// A deleted comment stays as a placeholder entry.
	deleted := insertFeedComment(t, e, issue.ID, at)
	if _, err := e.db.Exec(`UPDATE comments SET deleted_at = $1 WHERE id = $2`, at, deleted); err != nil {
		t.Fatal(err)
	}

	seen := map[uuid.UUID]bool{}
	var cursor *domain.ActivityCursor
	pages := 0
	for {
		refs, err := reader.FeedRefs(ctx, issue.ID, domain.ActivityFilterAll, domain.CommentLevelLead, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, ref := range refs {
			if seen[ref.ID] {
				t.Fatalf("entry %s repeated across pages", ref.ID)
			}
			seen[ref.ID] = true
		}
		pages++
		if len(refs) < 2 || pages > 10 {
			break
		}
		last := refs[len(refs)-1]
		cursor = &domain.ActivityCursor{At: last.At, Kind: last.Kind, ID: last.ID}
	}
	// 2 comments + 1 deleted comment + 1 batch + 1 standalone row = 5 entries.
	if len(seen) != 5 {
		t.Fatalf("entries = %d, want 5 (batch grouped, deleted comment as placeholder)", len(seen))
	}
	if !seen[batch] {
		t.Error("the batch must be one entry keyed by its batch id")
	}

	refs, err := reader.FeedRefs(ctx, issue.ID, domain.ActivityFilterHistory, domain.CommentLevelLead, nil, 10)
	if err != nil || len(refs) != 2 {
		t.Fatalf("history filter: refs=%d err=%v, want 2", len(refs), err)
	}
	content, err := reader.FeedContent(ctx, issue.ID, refs)
	if err != nil {
		t.Fatal(err)
	}
	if len(content.Activities[batch]) != 3 {
		t.Errorf("batch rows = %d, want 3", len(content.Activities[batch]))
	}
}

func TestActivityFeedStatementCountDoesNotGrowWithEntries(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	issue := e.newIssue(t, "Conteo")
	at := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	pool, counter := countingDB(t, e.db)
	reader := NewActivityFeedRepository(pool)

	measure := func() int64 {
		counter.n.Store(0)
		refs, err := reader.FeedRefs(ctx, issue.ID, domain.ActivityFilterAll, domain.CommentLevelLead, nil, 100)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := reader.FeedContent(ctx, issue.ID, refs); err != nil {
			t.Fatal(err)
		}
		return counter.n.Load()
	}
	// Both measurements must load comments and batches, so the statement count can only differ if it depends on
	// the number of entries.
	insertFeedComment(t, e, issue.ID, at)
	insertFeedActivity(t, e, issue.ID, uuid.New(), "summary", at)
	small := measure()
	for i := 0; i < 25; i++ {
		insertFeedComment(t, e, issue.ID, at.Add(time.Duration(i)*time.Second))
		insertFeedActivity(t, e, issue.ID, uuid.New(), "summary", at.Add(time.Duration(i)*time.Second))
	}
	large := measure()
	if small != large {
		t.Fatalf("statements: %d for one entry, %d for 50 entries", small, large)
	}
}

func TestActivityFeedReferenceNamesFollowRenames(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	reader := NewActivityFeedRepository(e.db)
	labels := NewIssueLabelRepository(e.db)
	issue := e.newIssue(t, "Etiquetas")
	ids, err := labels.ResolveLabelNames(ctx, e.project, []string{"bug", "ux"})
	if err != nil || len(ids) != 2 {
		t.Fatalf("resolve: %v", err)
	}
	again, err := labels.ResolveLabelNames(ctx, e.project, []string{"ux", "bug"})
	if err != nil || again[0] != ids[1] || again[1] != ids[0] {
		t.Fatalf("resolving existing names must return the same ids in input order: %v", err)
	}
	if err := labels.ReplaceIssueLabels(ctx, issue.ID, ids); err != nil {
		t.Fatal(err)
	}
	current, err := labels.IssueLabelIDs(ctx, issue.ID)
	if err != nil || len(current) != 2 {
		t.Fatalf("issue labels = %v, %v", current, err)
	}
	if _, err := e.db.Exec(`UPDATE labels SET name = 'defecto' WHERE id = $1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	names, err := reader.ReferenceNames(ctx, port.ReferenceLabel, ids)
	if err != nil {
		t.Fatal(err)
	}
	if names[ids[0]] != "defecto" {
		t.Errorf("renamed label name = %q, want current name", names[ids[0]])
	}
	if _, err := reader.ReferenceNames(ctx, "unknown", ids); err == nil {
		t.Error("unknown reference kind must fail")
	}
}

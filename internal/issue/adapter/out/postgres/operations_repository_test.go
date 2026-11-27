package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

func TestOperationsRelocateKeepsAliasAndDropsForeignLabels(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	repo := NewOperationsRepository(e.db)
	target := keyTestProject(t, e.db, e.org, e.lead, "DEST")
	issue := e.newIssue(t, "Para mover")

	label := uuid.New()
	if _, err := e.db.Exec(`INSERT INTO labels (id, project_id, name) VALUES ($1, $2, 'deuda')`, label, e.project); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddIssueLabels(ctx, issue.ID, []uuid.UUID{label}); err != nil {
		t.Fatal(err)
	}

	res, err := repo.Relocate(ctx, issue.ID, e.project, issue.KeyNumber, target)
	if err != nil {
		t.Fatalf("Relocate: %v", err)
	}
	if res.ProjectKey != "DEST" || res.KeyNumber != 1 || res.DroppedLabels != 1 {
		t.Fatalf("relocate result = %+v", res)
	}
	moved, err := e.repo.FindByID(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ProjectID != target || moved.Key() != "DEST-1" {
		t.Fatalf("moved issue = project %s key %s", moved.ProjectID, moved.Key())
	}

	ids, err := repo.FindAliasIssues(ctx, issue.ProjectKey, issue.KeyNumber, uuid.Nil)
	if err != nil || len(ids) != 1 || ids[0] != issue.ID {
		t.Fatalf("alias of the old key = %v, %v", ids, err)
	}
	if none, _ := repo.FindAliasIssues(ctx, issue.ProjectKey, issue.KeyNumber+5, uuid.Nil); len(none) != 0 {
		t.Fatalf("unknown number resolved: %v", none)
	}
	if none, _ := repo.FindAliasIssues(ctx, issue.ProjectKey, issue.KeyNumber, uuid.New()); len(none) != 0 {
		t.Fatalf("alias leaked to another organization: %v", none)
	}
	candidates, err := e.repo.FindKeyCandidates(ctx, issue.ProjectKey, issue.KeyNumber, uuid.Nil, e.lead, true)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("old key still matches the issue directly: %v, %v", candidates, err)
	}
	if names, _ := repo.IssueLabelNames(ctx, issue.ID); len(names) != 0 {
		t.Fatalf("labels of the source project kept: %v", names)
	}
}

func TestOperationsLabelsAreProjectScopedAndIdempotent(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	repo := NewOperationsRepository(e.db)
	issue := e.newIssue(t, "Etiquetas")
	mine, other := uuid.New(), uuid.New()
	if _, err := e.db.Exec(`INSERT INTO labels (id, project_id, name) VALUES ($1, $2, 'backend'), ($3, $4, 'ajena')`,
		mine, e.project, other, keyTestProject(t, e.db, e.org, e.lead, "OTRA")); err != nil {
		t.Fatal(err)
	}

	valid, err := repo.ProjectLabelIDs(ctx, e.project, []uuid.UUID{mine, other})
	if err != nil || len(valid) != 1 || valid[mine] != "backend" {
		t.Fatalf("project labels = %v, %v", valid, err)
	}
	for i := 0; i < 2; i++ {
		if err := repo.AddIssueLabels(ctx, issue.ID, []uuid.UUID{mine}); err != nil {
			t.Fatalf("add #%d: %v", i, err)
		}
	}
	names, _ := repo.IssueLabelNames(ctx, issue.ID)
	if len(names) != 1 || names[0] != "backend" {
		t.Fatalf("names after double add = %v", names)
	}
	byIssue, err := repo.LabelNamesByIssue(ctx, []uuid.UUID{issue.ID})
	if err != nil || len(byIssue[issue.ID]) != 1 {
		t.Fatalf("labels by issue = %v, %v", byIssue, err)
	}
	if err := repo.RemoveIssueLabels(ctx, issue.ID, []uuid.UUID{mine, other}); err != nil {
		t.Fatal(err)
	}
	if names, _ := repo.IssueLabelNames(ctx, issue.ID); len(names) != 0 {
		t.Fatalf("labels after remove = %v", names)
	}
}

func TestOperationsCopyLinksKeepsKindAndSkipsClones(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	repo := NewOperationsRepository(e.db)
	source := e.newIssue(t, "Origen")
	dest := e.newIssue(t, "Copia")
	blocked := e.newIssue(t, "Bloqueada")
	saveLink(t, NewIssueLinkRepository(e.db), source.ID, blocked.ID, domain.IssueLinkBlocks, e.lead)
	saveLink(t, NewIssueLinkRepository(e.db), blocked.ID, source.ID, domain.LinkKindClones, e.lead)

	if err := repo.CopyLinks(ctx, source.ID, dest.ID, e.lead); err != nil {
		t.Fatalf("CopyLinks: %v", err)
	}
	links, err := NewIssueLinkRepository(e.db).ListLinks(ctx, dest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Kind != domain.IssueLinkBlocks || !links[0].Outward || links[0].Other.ID != blocked.ID {
		t.Fatalf("copied links = %+v, want only the blocks link with its direction", links)
	}
}

package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

func linkRepoOf(e v2Env) *IssueLinkRepository {
	return NewIssueLinkRepository(e.db)
}

func saveLink(t *testing.T, repo *IssueLinkRepository, source, target uuid.UUID, kind domain.IssueLinkKind, actor uuid.UUID) *domain.IssueLink {
	t.Helper()
	link, err := domain.NewIssueLink(source, target, actor, kind, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), link); err != nil {
		t.Fatalf("Save %s: %v", kind, err)
	}
	return link
}

func TestIssueLinkRepositoryRulesAndListing(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	repo := linkRepoOf(e)
	actor := keyTestUser(t, e.db, e.org)
	a, b, c := e.newIssue(t, "Origen"), e.newIssue(t, "Destino"), e.newIssue(t, "Tercera")

	link := saveLink(t, repo, a.ID, b.ID, domain.IssueLinkBlocks, actor)
	if err := repo.Save(ctx, &domain.IssueLink{SourceIssueID: a.ID, TargetIssueID: b.ID, Kind: domain.IssueLinkBlocks, CreatedBy: actor, CreatedAt: time.Now()}); !errors.Is(err, domain.ErrDuplicateLink) {
		t.Fatalf("same kind twice = %v, want duplicate", err)
	}
	exists, err := repo.PairExists(ctx, b.ID, a.ID, domain.IssueLinkBlocks)
	if err != nil || !exists {
		t.Fatalf("PairExists inverse = %v, %v; want true", exists, err)
	}
	reach, err := repo.BlocksReach(ctx, b.ID, a.ID)
	if err != nil || reach {
		t.Fatalf("BlocksReach from target = %v, %v; want false", reach, err)
	}
	reach, err = repo.BlocksReach(ctx, a.ID, b.ID)
	if err != nil || !reach {
		t.Fatalf("BlocksReach along the link = %v, %v; want true", reach, err)
	}
	saveLink(t, repo, b.ID, c.ID, domain.IssueLinkBlocks, actor)
	if reach, _ := repo.BlocksReach(ctx, a.ID, c.ID); !reach {
		t.Fatal("two-step blocking chain not followed")
	}

	outward, err := repo.ListLinks(ctx, a.ID)
	if err != nil || len(outward) != 1 || !outward[0].Outward || outward[0].Other.Key != b.Key() || outward[0].Kind != domain.IssueLinkBlocks {
		t.Fatalf("ListLinks(a) = %+v, %v", outward, err)
	}
	inward, err := repo.ListLinks(ctx, b.ID)
	if err != nil || len(inward) != 2 || inward[0].Outward || inward[0].Other.Key != a.Key() {
		t.Fatalf("ListLinks(b) = %+v, %v", inward, err)
	}
	if summary, err := repo.FindLinkable(ctx, a.ID); err != nil || summary.OrganizationID != e.org || summary.Key != a.Key() {
		t.Fatalf("FindLinkable = %+v, %v", summary, err)
	}

	if _, err := repo.FindLinkForIssue(ctx, c.ID, link.ID); !errors.Is(err, domain.ErrIssueLinkNotFound) {
		t.Fatalf("link seen from an unrelated issue = %v", err)
	}
	if err := repo.Delete(ctx, link.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, link.ID); !errors.Is(err, domain.ErrIssueLinkNotFound) {
		t.Fatalf("second delete = %v", err)
	}
}

func TestIssueLinkRepositoryHidesSoftDeletedEnds(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	repo := linkRepoOf(e)
	actor := keyTestUser(t, e.db, e.org)
	a, b := e.newIssue(t, "Vivo"), e.newIssue(t, "Borrado")
	saveLink(t, repo, a.ID, b.ID, domain.IssueLinkRelates, actor)

	if err := e.repo.SoftDelete(ctx, b.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	listed, err := repo.ListLinks(ctx, a.ID)
	if err != nil || len(listed) != 0 {
		t.Fatalf("ListLinks with a deleted end = %+v, %v; want none", listed, err)
	}
	if _, err := repo.FindLinkable(ctx, b.ID); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("FindLinkable of a deleted issue = %v", err)
	}
}

func TestChildrenListingExcludesDeletedChildren(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	parent := e.newIssue(t, "Epica")
	done := newKeyTestIssue(e.project, e.lead, "Hecha")
	done.ParentID = &parent.ID
	done.Status = domain.StatusClosed
	if err := e.repo.Save(ctx, done); err != nil {
		t.Fatal(err)
	}
	open := newKeyTestIssue(e.project, e.lead, "Abierta")
	open.ParentID = &parent.ID
	if err := e.repo.Save(ctx, open); err != nil {
		t.Fatal(err)
	}
	gone := newKeyTestIssue(e.project, e.lead, "Borrada")
	gone.ParentID = &parent.ID
	if err := e.repo.Save(ctx, gone); err != nil {
		t.Fatal(err)
	}
	if err := e.repo.SoftDelete(ctx, gone.ID, time.Now()); err != nil {
		t.Fatal(err)
	}

	children, err := e.repo.ListChildren(ctx, parent.ID)
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("children = %d, want 2 live ones", len(children))
	}
	progress := domain.ComputeChildProgress(children)
	if progress.Total != 2 || progress.Done != 1 || progress.Percent != 50 {
		t.Fatalf("progress = %+v", progress)
	}
}

func TestVotesStayConsistentUnderConcurrency(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	issue := e.newIssue(t, "Muy pedida")
	tx := database.NewTxManager(e.db)

	const voters = 12
	users := make([]uuid.UUID, voters)
	for i := range users {
		users[i] = keyTestUser(t, e.db, e.org)
	}
	castAll := func(ids []uuid.UUID) {
		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Add(1)
			go func(user uuid.UUID) {
				defer wg.Done()
				if err := tx.InTx(ctx, func(ctx context.Context) error {
					_, err := e.repo.CastVote(ctx, issue.ID, user)
					return err
				}); err != nil {
					t.Errorf("CastVote: %v", err)
				}
			}(id)
		}
		wg.Wait()
	}
	assertCounter := func(want int) {
		t.Helper()
		var rows, counter int
		if err := e.db.QueryRow(`SELECT COUNT(*) FROM issue_votes WHERE issue_id = $1`, issue.ID).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if err := e.db.QueryRow(`SELECT vote_count FROM issues WHERE id = $1`, issue.ID).Scan(&counter); err != nil {
			t.Fatal(err)
		}
		if rows != want || counter != want {
			t.Fatalf("votes = %d rows, counter %d; want %d", rows, counter, want)
		}
	}

	castAll(users)
	assertCounter(voters)

	// Repeating a vote changes nothing.
	changed, err := e.repo.CastVote(ctx, issue.ID, users[0])
	if err != nil || changed {
		t.Fatalf("repeated vote = %v, %v; want no change", changed, err)
	}
	assertCounter(voters)

	var wg sync.WaitGroup
	for _, id := range users[:voters/2] {
		wg.Add(1)
		go func(user uuid.UUID) {
			defer wg.Done()
			if _, err := e.repo.WithdrawVote(ctx, issue.ID, user); err != nil {
				t.Errorf("WithdrawVote: %v", err)
			}
		}(id)
	}
	wg.Wait()
	assertCounter(voters / 2)

	voted, err := e.repo.HasVoted(ctx, issue.ID, users[voters-1])
	if err != nil || !voted {
		t.Fatalf("HasVoted for a remaining voter = %v, %v", voted, err)
	}
	withdrawn, err := e.repo.WithdrawVote(ctx, issue.ID, users[0])
	if err != nil || withdrawn {
		t.Fatalf("withdraw without a vote = %v, %v; want no change", withdrawn, err)
	}
	assertCounter(voters / 2)
}

func TestVotesRejectSoftDeletedIssue(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	issue := e.newIssue(t, "Borrada")
	if err := e.repo.SoftDelete(ctx, issue.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	voter := keyTestUser(t, e.db, e.org)
	changed, err := e.repo.CastVote(ctx, issue.ID, voter)
	if err != nil || changed {
		t.Fatalf("vote on a deleted issue = %v, %v; want no change", changed, err)
	}
}

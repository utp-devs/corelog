// Package usecase implements the note use cases. Notes are private: every operation is scoped to the actor,
// administrators included.
package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/note/application/port"
	"github.com/jhonsferg/corelog/internal/note/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/pagination"
)

const (
	DefaultListLimit   = 25
	MaxListLimit       = 100
	DefaultSearchLimit = 10
	MaxSearchLimit     = 50
)

// Service implements port.Service.
type Service struct {
	repo  port.Repository
	links port.LinkAccess
	clock clock.Clock
}

var _ port.Service = (*Service)(nil)

func NewService(repo port.Repository, links port.LinkAccess, clk clock.Clock) *Service {
	return &Service{repo: repo, links: links, clock: clk}
}

func (s *Service) Create(ctx context.Context, actorID uuid.UUID, fields domain.Fields) (domain.Note, error) {
	n, err := domain.NewNote(actorID, fields, s.clock.Now())
	if err != nil {
		return domain.Note{}, err
	}
	if err := s.ensureLinks(ctx, actorID, n.IssueID, n.ProjectID); err != nil {
		return domain.Note{}, err
	}
	created, err := s.repo.Insert(ctx, n)
	if err != nil {
		return domain.Note{}, fmt.Errorf("note: create: %w", err)
	}
	return created, nil
}

func (s *Service) Update(ctx context.Context, actorID, id uuid.UUID, patch domain.Patch) (domain.Note, error) {
	n, err := s.repo.Get(ctx, actorID, id)
	if err != nil {
		return domain.Note{}, err
	}
	// Only a link that changes is checked: keeping an existing link does not depend on the current permission.
	var checkIssue, checkProject *uuid.UUID
	if n.IssueLinkChanged(patch) && patch.IssueID != nil {
		checkIssue = patch.IssueID
	}
	if n.ProjectLinkChanged(patch) && patch.ProjectID != nil {
		checkProject = patch.ProjectID
	}
	if err := n.ApplyPatch(patch, s.clock.Now()); err != nil {
		return domain.Note{}, err
	}
	if err := s.ensureLinks(ctx, actorID, checkIssue, checkProject); err != nil {
		return domain.Note{}, err
	}
	if err := s.repo.Update(ctx, n); err != nil {
		return domain.Note{}, fmt.Errorf("note: update: %w", err)
	}
	return n, nil
}

func (s *Service) Delete(ctx context.Context, actorID, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, actorID, id, s.clock.Now())
}

func (s *Service) Get(ctx context.Context, actorID, id uuid.UUID) (domain.Note, error) {
	return s.repo.Get(ctx, actorID, id)
}

func (s *Service) SetPinned(ctx context.Context, actorID, id uuid.UUID, pinned bool) (domain.Note, error) {
	n, err := s.repo.Get(ctx, actorID, id)
	if err != nil {
		return domain.Note{}, err
	}
	if n.Pinned == pinned {
		return n, nil
	}
	n.SetPinned(pinned, s.clock.Now())
	if err := s.repo.Update(ctx, n); err != nil {
		return domain.Note{}, fmt.Errorf("note: pin: %w", err)
	}
	return n, nil
}

func (s *Service) List(ctx context.Context, actorID uuid.UUID, query port.ListQuery) (port.ListPage, error) {
	if query.Limit == 0 {
		query.Limit = DefaultListLimit
	}
	if query.Limit < 1 || query.Limit > MaxListLimit {
		return port.ListPage{}, apperror.WithCode(apperror.KindValidation, "note.invalid", "el limite debe estar entre 1 y 100")
	}
	query.Query = strings.TrimSpace(query.Query)
	if tag := strings.TrimSpace(query.Tag); tag != "" {
		tags, err := domain.NormalizeTags([]string{tag})
		if err != nil {
			return port.ListPage{}, err
		}
		query.Tag = tags[0]
	}
	rows, err := s.repo.List(ctx, actorID, query)
	if err != nil {
		return port.ListPage{}, err
	}
	page := port.ListPage{Items: rows}
	if len(rows) > query.Limit {
		page.Items = rows[:query.Limit]
		last := page.Items[len(page.Items)-1]
		cursor, err := pagination.EncodeCursor(port.Position{Pinned: last.Pinned, UpdatedAt: last.UpdatedAt, ID: last.ID})
		if err != nil {
			return port.ListPage{}, fmt.Errorf("note: encode cursor: %w", err)
		}
		page.NextCursor = cursor
	}
	return page, nil
}

func (s *Service) ListTags(ctx context.Context, actorID uuid.UUID) ([]port.TagCount, error) {
	return s.repo.ListTags(ctx, actorID)
}

func (s *Service) SearchOwn(ctx context.Context, userID uuid.UUID, query string, limit int) ([]port.SearchHit, error) {
	q := strings.TrimSpace(query)
	if q == "" || userID == uuid.Nil {
		return []port.SearchHit{}, nil
	}
	switch {
	case limit <= 0:
		limit = DefaultSearchLimit
	case limit > MaxSearchLimit:
		limit = MaxSearchLimit
	}
	return s.repo.SearchOwn(ctx, userID, q, limit)
}

// ensureLinks checks that the actor can browse every issue and project the note points at.
func (s *Service) ensureLinks(ctx context.Context, actorID uuid.UUID, issueID, projectID *uuid.UUID) error {
	if issueID != nil {
		ok, err := s.links.CanBrowseIssue(ctx, actorID, *issueID)
		if err != nil {
			return fmt.Errorf("note: check issue link: %w", err)
		}
		if !ok {
			return domain.ErrLinkForbidden
		}
	}
	if projectID != nil {
		ok, err := s.links.CanBrowseProject(ctx, actorID, *projectID)
		if err != nil {
			return fmt.Errorf("note: check project link: %w", err)
		}
		if !ok {
			return domain.ErrLinkForbidden
		}
	}
	return nil
}

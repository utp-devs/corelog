// Package usecase implements the versions and release notes of a project.
package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/release/application/port"
	"github.com/jhonsferg/corelog/internal/release/domain"
)

// Move modes of the release action.
const (
	MoveNone    = "none"
	MoveVersion = "version"
	MoveNew     = "new"
)

// VersionService implements port.Service. Reads need browse_project; writes need manage_versions. A caller who
// cannot browse the project gets not found, so hidden projects do not reveal their versions.
type VersionService struct {
	repo       port.Repository
	authorizer port.Authorizer
	tx         port.TxRunner
	events     port.EventPublisher
	clock      clock.Clock
}

// NewVersionService wires the service. events may be nil in tests that do not check events.
func NewVersionService(repo port.Repository, authorizer port.Authorizer, tx port.TxRunner, events port.EventPublisher, clk clock.Clock) *VersionService {
	return &VersionService{repo: repo, authorizer: authorizer, tx: tx, events: events, clock: clk}
}

var _ port.Service = (*VersionService)(nil)

// requireBrowse lets the actor see the project's versions. Without browse access the project looks missing.
func (s *VersionService) requireBrowse(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID)
	if apperror.KindOf(err) == apperror.KindForbidden {
		return domain.ErrVersionNotFound
	}
	return err
}

// requireManage lets the actor change versions. Visible but denied actors get forbidden.
func (s *VersionService) requireManage(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	return s.authorizer.Authorize(ctx, port.ActionManageVersions, actorID, globalRole, projectID)
}

// authorizeRead checks browse access to the project of v.
func (s *VersionService) authorizeRead(ctx context.Context, actorID uuid.UUID, globalRole string, v *domain.Version) error {
	return s.requireBrowse(ctx, actorID, globalRole, v.ProjectID)
}

// withProgress fills the progress of every version with one grouped query.
func (s *VersionService) withProgress(ctx context.Context, versions []*domain.Version) error {
	if len(versions) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(versions))
	for _, v := range versions {
		ids = append(ids, v.ID)
	}
	rows, err := s.repo.Progress(ctx, ids)
	if err != nil {
		return err
	}
	for _, v := range versions {
		v.Progress = domain.NewProgress(rows[v.ID])
	}
	return nil
}

func (s *VersionService) List(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, filter port.ListFilter) ([]*domain.Version, error) {
	if err := s.requireBrowse(ctx, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	if filter.Status != nil && !filter.Status.Valid() {
		return nil, domain.ErrVersionStatusInvalid
	}
	versions, err := s.repo.List(ctx, projectID, filter.Status)
	if err != nil {
		return nil, err
	}
	if err := s.withProgress(ctx, versions); err != nil {
		return nil, err
	}
	return versions, nil
}

func (s *VersionService) Create(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, in port.CreateInput) (*domain.Version, error) {
	if err := s.requireBrowse(ctx, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	v, err := domain.NewVersion(projectID, in.Name, in.Description, in.StartDate, in.ReleaseDate, s.clock.Now())
	if err != nil {
		return nil, err
	}
	taken, err := s.repo.NameTaken(ctx, projectID, v.Name, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, domain.ErrVersionNameTaken
	}
	if err := s.repo.Create(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *VersionService) Get(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Version, error) {
	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeRead(ctx, actorID, globalRole, v); err != nil {
		return nil, err
	}
	if err := s.withProgress(ctx, []*domain.Version{v}); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *VersionService) Update(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, patch domain.VersionPatch) (*domain.Version, error) {
	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeRead(ctx, actorID, globalRole, v); err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, actorID, globalRole, v.ProjectID); err != nil {
		return nil, err
	}
	if err := v.Update(patch, s.clock.Now()); err != nil {
		return nil, err
	}
	if patch.Name != nil {
		taken, err := s.repo.NameTaken(ctx, v.ProjectID, v.Name, v.ID)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, domain.ErrVersionNameTaken
		}
	}
	if err := s.repo.Save(ctx, v); err != nil {
		return nil, err
	}
	if err := s.withProgress(ctx, []*domain.Version{v}); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *VersionService) Delete(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, moveTo *uuid.UUID) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		v, err := s.repo.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if err := s.authorizeRead(ctx, actorID, globalRole, v); err != nil {
			return err
		}
		if err := s.requireManage(ctx, actorID, globalRole, v.ProjectID); err != nil {
			return err
		}
		if moveTo != nil {
			if err := s.checkMoveTarget(ctx, v, *moveTo); err != nil {
				return err
			}
			if err := s.repo.MoveLinks(ctx, v.ID, *moveTo); err != nil {
				return err
			}
		}
		return s.repo.Delete(ctx, v.ID)
	})
}

// checkMoveTarget validates a destination version: same project, different from source, still unreleased.
func (s *VersionService) checkMoveTarget(ctx context.Context, source *domain.Version, targetID uuid.UUID) error {
	target, err := s.repo.FindByID(ctx, targetID)
	if err != nil {
		if apperror.CodeOf(err) == domain.ErrVersionNotFound.Code {
			return domain.ErrVersionMoveTarget
		}
		return err
	}
	if target.ID == source.ID || target.ProjectID != source.ProjectID || !target.AcceptsMoves() {
		return domain.ErrVersionMoveTarget
	}
	return nil
}

func (s *VersionService) Archive(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Version, error) {
	var out *domain.Version
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		v, err := s.repo.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if err := s.authorizeRead(ctx, actorID, globalRole, v); err != nil {
			return err
		}
		if err := s.requireManage(ctx, actorID, globalRole, v.ProjectID); err != nil {
			return err
		}
		if err := v.Archive(s.clock.Now()); err != nil {
			return err
		}
		if err := s.repo.Save(ctx, v); err != nil {
			return err
		}
		out = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.withProgress(ctx, []*domain.Version{out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *VersionService) Release(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, in port.ReleaseInput) (*port.ReleaseResult, error) {
	result := &port.ReleaseResult{}
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		v, err := s.repo.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if err := s.authorizeRead(ctx, actorID, globalRole, v); err != nil {
			return err
		}
		if err := s.requireManage(ctx, actorID, globalRole, v.ProjectID); err != nil {
			return err
		}
		if err := v.Release(in.ReleaseDate, s.clock.Now()); err != nil {
			return err
		}
		unresolved, err := s.repo.UnresolvedFixCount(ctx, v.ID)
		if err != nil {
			return err
		}
		result.UnresolvedIssues = unresolved
		switch in.Move.Mode {
		case "", MoveNone:
		case MoveVersion:
			if err := s.checkMoveTarget(ctx, v, in.Move.VersionID); err != nil {
				return err
			}
			if result.MovedIssues, err = s.repo.MoveUnresolvedFixes(ctx, v.ID, in.Move.VersionID); err != nil {
				return err
			}
		case MoveNew:
			next, err := s.createNextVersion(ctx, v)
			if err != nil {
				return err
			}
			result.NextVersion = next
			if result.MovedIssues, err = s.repo.MoveUnresolvedFixes(ctx, v.ID, next.ID); err != nil {
				return err
			}
		default:
			return domain.ErrVersionMoveTarget
		}
		result.UnresolvedIssues -= result.MovedIssues
		if err := s.repo.Save(ctx, v); err != nil {
			return err
		}
		result.Version = v
		return s.publishReleased(ctx, v, actorID, result)
	})
	if err != nil {
		return nil, err
	}
	if err := s.withProgress(ctx, []*domain.Version{result.Version}); err != nil {
		return nil, err
	}
	return result, nil
}

// createNextVersion creates the unreleased version that receives the unresolved issues. Its name derives from the
// released one and gets a numeric suffix when the name is taken.
func (s *VersionService) createNextVersion(ctx context.Context, released *domain.Version) (*domain.Version, error) {
	base := []rune("Siguiente a " + released.Name)
	if len(base) > domain.MaxVersionNameLength-6 {
		base = base[:domain.MaxVersionNameLength-6]
	}
	stem := strings.TrimSpace(string(base))
	for attempt := 1; attempt <= 9; attempt++ {
		name := stem
		if attempt > 1 {
			name = fmt.Sprintf("%s (%d)", stem, attempt)
		}
		taken, err := s.repo.NameTaken(ctx, released.ProjectID, name, uuid.Nil)
		if err != nil {
			return nil, err
		}
		if taken {
			continue
		}
		next, err := domain.NewVersion(released.ProjectID, name, "", nil, nil, s.clock.Now())
		if err != nil {
			return nil, err
		}
		if err := s.repo.Create(ctx, next); err != nil {
			return nil, err
		}
		return next, nil
	}
	return nil, domain.ErrVersionNameTaken
}

func (s *VersionService) publishReleased(ctx context.Context, v *domain.Version, actorID uuid.UUID, result *port.ReleaseResult) error {
	if s.events == nil {
		return nil
	}
	versionID := v.ID
	projectID := v.ProjectID
	actor := actorID
	payload := map[string]any{
		"version_id":        v.ID.String(),
		"name":              v.Name,
		"release_date":      domain.FormatDate(v.ReleaseDate),
		"moved_issues":      result.MovedIssues,
		"unresolved_issues": result.UnresolvedIssues,
	}
	if result.NextVersion != nil {
		payload["next_version_id"] = result.NextVersion.ID.String()
	}
	return s.events.Publish(ctx, events.Event{
		Type: "version.released", EntityType: "version", EntityID: &versionID,
		ProjectID: &projectID, ActorID: &actor, Payload: payload, CreatedAt: s.clock.Now(),
	})
}

func (s *VersionService) Notes(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.ReleaseNotes, error) {
	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeRead(ctx, actorID, globalRole, v); err != nil {
		return nil, err
	}
	issues, err := s.repo.NotesIssues(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	notes := domain.BuildReleaseNotes(v, issues)
	return &notes, nil
}

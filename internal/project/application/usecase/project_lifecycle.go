package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// Event types published by the project lifecycle operations.
const (
	EventProjectArchived = "project.archived"
	EventProjectRestored = "project.restored"
	EventProjectDeleted  = "project.deleted"
)

const (
	defaultPageSize = 25
	maxPageSize     = 100
)

var _ port.ProjectManager = (*ProjectService)(nil)

// CreateProject creates a project from a template. The creator becomes lead. Initial members must
// belong to the organization; team grants are validated inside the creation transaction.
func (s *ProjectService) CreateProject(ctx context.Context, actorID, orgID uuid.UUID, input port.CreateInput) (*domain.Project, error) {
	orgRole, err := s.orgRoleOf(ctx, orgID, actorID)
	if err != nil {
		return nil, err
	}
	if orgRole == "" {
		return nil, domain.ErrOrganizationNotFound
	}
	policy, defaultVisibility, err := s.orgs.ProjectCreationPolicy(ctx, orgID)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read organization project policy", err)
	}
	if !domain.CanCreateProject(orgRole, policy) {
		return nil, domain.ErrProjectCreationNotAllowed
	}
	visibility := input.Visibility
	if visibility == "" {
		visibility = domain.DefaultVisibility(defaultVisibility)
	}
	project, err := domain.NewProjectFromInput(domain.NewProjectInput{
		OrganizationID:        orgID,
		Key:                   input.Key,
		Name:                  input.Name,
		Description:           input.Description,
		Template:              input.Template,
		Visibility:            visibility,
		LeadID:                actorID,
		Color:                 input.Color,
		Icon:                  input.Icon,
		DefaultAssigneePolicy: input.DefaultAssigneePolicy,
	})
	if err != nil {
		return nil, err
	}
	available, err := s.repo.KeyAvailable(ctx, orgID, project.Key)
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, domain.ErrProjectKeyTaken
	}

	members, err := s.validInitialMembers(ctx, orgID, actorID, input.MemberIDs)
	if err != nil {
		return nil, err
	}
	teams := dedupeUUIDs(input.TeamIDs)
	now := time.Now().UTC()
	lead := port.Member{UserID: actorID, Role: "lead", CreatedAt: now}

	if creator, ok := s.repo.(port.ProjectCreationRepository); ok {
		if err := creator.CreateWithMembers(ctx, project, lead, members, teams); err != nil {
			return nil, err
		}
		return project, nil
	}
	if len(members) > 0 || len(teams) > 0 {
		return nil, domain.ErrInvalidProject
	}
	if err := s.repo.Save(ctx, project); err != nil {
		return nil, err
	}
	lead.ProjectID = project.ID
	if err := s.memberRepo.Add(ctx, lead); err != nil {
		return nil, err
	}
	return project, nil
}

// ListProjects returns one page of the projects the actor may see in the organization.
func (s *ProjectService) ListProjects(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID, filter port.ProjectFilter) (port.ProjectPage, error) {
	orgRole, err := s.orgRoleOf(ctx, orgID, actorID)
	if err != nil {
		return port.ProjectPage{}, err
	}
	if orgRole == "" && globalRole != platformAdmin {
		return port.ProjectPage{}, domain.ErrOrganizationNotFound
	}
	if filter.Template != "" {
		template, err := domain.ParseTemplate(string(filter.Template))
		if err != nil {
			return port.ProjectPage{}, err
		}
		filter.Template = template
	}
	if filter.Archived, err = normalizeArchiveFilter(filter.Archived); err != nil {
		return port.ProjectPage{}, err
	}
	if filter.Sort, err = normalizeSortField(filter.Sort); err != nil {
		return port.ProjectPage{}, err
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	switch {
	case filter.PageSize <= 0:
		filter.PageSize = defaultPageSize
	case filter.PageSize > maxPageSize:
		filter.PageSize = maxPageSize
	}
	filter.Q = strings.TrimSpace(filter.Q)
	filter.OrganizationID = orgID
	filter.Scope = s.scopeFor(actorID, globalRole, orgRole)
	return s.repo.ListPage(ctx, filter)
}

// Archive makes the project read-only. Requires project management rights.
func (s *ProjectService) Archive(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Project, error) {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, port.ActionArchiveProject, project, actorID, globalRole, orgRole); err != nil {
		return nil, err
	}
	if err := project.Archive(time.Now()); err != nil {
		return nil, err
	}
	if err := s.persistWithEvent(ctx, project, actorID, EventProjectArchived); err != nil {
		return nil, err
	}
	return project, nil
}

// Restore reverts Archive.
func (s *ProjectService) Restore(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Project, error) {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, port.ActionRestoreProject, project, actorID, globalRole, orgRole); err != nil {
		return nil, err
	}
	if err := project.Restore(time.Now()); err != nil {
		return nil, err
	}
	if err := s.persistWithEvent(ctx, project, actorID, EventProjectRestored); err != nil {
		return nil, err
	}
	return project, nil
}

// DeleteProject soft deletes the project. The lead or an org owner/admin may delete it
// (delete_project in the permission scheme), and the caller must type the exact project name.
// Permission is checked before the name so the name is not revealed to actors who cannot delete
// the project.
func (s *ProjectService) DeleteProject(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, confirmation string) error {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, id)
	if err != nil {
		return err
	}
	if err := s.requireManage(ctx, port.ActionDeleteProject, project, actorID, globalRole, orgRole); err != nil {
		return err
	}
	if err := project.ConfirmDeletion(confirmation); err != nil {
		return err
	}
	return s.softDelete(ctx, project, actorID)
}

// KeyAvailable reports whether the key is valid and free in the organization.
func (s *ProjectService) KeyAvailable(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID, key string) (bool, error) {
	orgRole, err := s.orgRoleOf(ctx, orgID, actorID)
	if err != nil {
		return false, err
	}
	if orgRole == "" && globalRole != platformAdmin {
		return false, domain.ErrOrganizationNotFound
	}
	normalized, err := domain.ValidateKey(key)
	if err != nil {
		return false, err
	}
	return s.repo.KeyAvailable(ctx, orgID, normalized)
}

func (s *ProjectService) softDelete(ctx context.Context, project *domain.Project, actorID uuid.UUID) error {
	if err := project.SoftDelete(time.Now()); err != nil {
		return err
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Delete(ctx, project.OrganizationID, project.ID); err != nil {
			return err
		}
		return s.publish(ctx, project, actorID, EventProjectDeleted)
	})
}

// persistWithEvent saves the project and records its event in one transaction.
func (s *ProjectService) persistWithEvent(ctx context.Context, project *domain.Project, actorID uuid.UUID, eventType string) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Update(ctx, project); err != nil {
			return err
		}
		return s.publish(ctx, project, actorID, eventType)
	})
}

func (s *ProjectService) publish(ctx context.Context, project *domain.Project, actorID uuid.UUID, eventType string) error {
	if s.events == nil {
		return nil
	}
	return s.events.PublishProjectEvent(ctx, port.ProjectEvent{
		OrganizationID: project.OrganizationID,
		ProjectID:      project.ID,
		ActorID:        actorID,
		Type:           eventType,
		Payload:        map[string]any{"key": project.Key, "name": project.Name},
	})
}

func (s *ProjectService) inTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if s.tx == nil {
		return fn(ctx)
	}
	return s.tx.InTx(ctx, fn)
}

// validInitialMembers checks that every initial member belongs to the organization. The creator
// is always lead and is removed from the list; duplicates are dropped.
func (s *ProjectService) validInitialMembers(ctx context.Context, orgID, actorID uuid.UUID, ids []uuid.UUID) ([]port.Member, error) {
	now := time.Now().UTC()
	var members []port.Member
	for _, id := range dedupeUUIDs(ids) {
		if id == actorID {
			continue
		}
		_, found, err := s.orgs.MemberRole(ctx, orgID, id)
		if err != nil {
			return nil, apperror.Wrap(apperror.KindInternal, "failed to check organization membership", err)
		}
		if !found {
			return nil, domain.ErrUserNotInOrg
		}
		members = append(members, port.Member{UserID: id, Role: "contributor", CreatedAt: now})
	}
	return members, nil
}

func normalizeArchiveFilter(value port.ArchiveFilter) (port.ArchiveFilter, error) {
	switch value {
	case "":
		return port.ArchiveFilterExclude, nil
	case port.ArchiveFilterExclude, port.ArchiveFilterInclude, port.ArchiveFilterOnly:
		return value, nil
	default:
		return "", domain.ErrInvalidProject
	}
}

func normalizeSortField(value port.SortField) (port.SortField, error) {
	switch value {
	case "":
		return port.SortByName, nil
	case port.SortByName, port.SortByKey, port.SortByCreatedAt, port.SortByUpdatedAt:
		return value, nil
	default:
		return "", domain.ErrInvalidProject
	}
}

// dedupeUUIDs drops nil and repeated ids, keeping the first occurrence.
func dedupeUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

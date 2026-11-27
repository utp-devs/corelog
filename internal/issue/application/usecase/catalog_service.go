package usecase

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// Organization roles used by the catalog permissions. They match the organization module names.
const (
	orgRoleAdmin = "admin"
	orgRoleGuest = "guest"
)

// Project actions that guard the project issue-type settings.
const (
	actionViewProject = "view_project"
	actionEditProject = "edit_project"
)

// CatalogService implements port.CatalogService.
type CatalogService struct {
	store      port.CatalogStore
	roles      port.OrgRoleChecker
	authorizer port.ProjectAuthorizer
	tx         database.TxManager
	clock      clock.Clock
}

var _ port.CatalogService = (*CatalogService)(nil)

// NewCatalogService wires the catalog use cases. All dependencies are required.
func NewCatalogService(store port.CatalogStore, roles port.OrgRoleChecker, authorizer port.ProjectAuthorizer, tx database.TxManager, clk clock.Clock) *CatalogService {
	return &CatalogService{store: store, roles: roles, authorizer: authorizer, tx: tx, clock: clk}
}

// ListCatalog returns the items of a catalog. Every organization member may read.
func (s *CatalogService) ListCatalog(ctx context.Context, actorID, orgID uuid.UUID, kind domain.CatalogKind) ([]*domain.CatalogItem, error) {
	if err := s.roles.RequireOrgRole(ctx, actorID, orgID, orgRoleGuest); err != nil {
		return nil, err
	}
	return s.store.ListItems(ctx, orgID, kind)
}

// CreateCatalogItem adds an item at the end of the catalog. Owners and admins only.
func (s *CatalogService) CreateCatalogItem(ctx context.Context, actorID, orgID uuid.UUID, kind domain.CatalogKind, in port.CatalogInput) (*domain.CatalogItem, error) {
	if err := s.roles.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return nil, err
	}
	now := s.clock.Now().UTC()
	item := &domain.CatalogItem{
		OrganizationID: orgID,
		Kind:           kind,
		Name:           in.Name,
		Description:    in.Description,
		Icon:           in.Icon,
		Color:          in.Color,
		HierarchyLevel: in.HierarchyLevel,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if kind != domain.CatalogIssueType {
		item.HierarchyLevel = domain.HierarchyStandard
	}
	if kind == domain.CatalogResolution {
		item.Icon, item.Color = "", ""
	}
	if err := normalizeItem(item); err != nil {
		return nil, err
	}

	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.store.LockOrganization(ctx, orgID); err != nil {
			return err
		}
		items, err := s.store.ListItems(ctx, orgID, kind)
		if err != nil {
			return err
		}
		if err := checkNameFree(items, item.Name, uuid.Nil); err != nil {
			return err
		}
		if kind == domain.CatalogIssueType && item.HierarchyLevel != domain.HierarchyStandard {
			for _, existing := range items {
				if existing.HierarchyLevel == item.HierarchyLevel {
					return domain.ErrCatalogHierarchyTaken
				}
			}
		}
		item.Position = len(items)
		if err := s.store.InsertItem(ctx, item); err != nil {
			return err
		}
		if in.IsDefault || len(items) == 0 {
			return s.store.SetDefaultItem(ctx, orgID, kind, item.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.store.FindItem(ctx, orgID, kind, item.ID)
}

// UpdateCatalogItem changes name, description, icon, color or the default flag. The hierarchy level of an
// issue type never changes. Owners and admins only.
func (s *CatalogService) UpdateCatalogItem(ctx context.Context, actorID, orgID uuid.UUID, kind domain.CatalogKind, id uuid.UUID, patch port.CatalogPatch) (*domain.CatalogItem, error) {
	if err := s.roles.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return nil, err
	}
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.store.LockOrganization(ctx, orgID); err != nil {
			return err
		}
		item, err := s.store.FindItem(ctx, orgID, kind, id)
		if err != nil {
			return err
		}
		if patch.Name != nil {
			item.Name = *patch.Name
		}
		if patch.Description != nil {
			item.Description = *patch.Description
		}
		if kind != domain.CatalogResolution {
			if patch.Icon != nil {
				item.Icon = *patch.Icon
			}
			if patch.Color != nil {
				item.Color = *patch.Color
			}
		}
		if err := normalizeItem(item); err != nil {
			return err
		}
		items, err := s.store.ListItems(ctx, orgID, kind)
		if err != nil {
			return err
		}
		if err := checkNameFree(items, item.Name, item.ID); err != nil {
			return err
		}
		item.UpdatedAt = s.clock.Now().UTC()
		if err := s.store.UpdateItem(ctx, item); err != nil {
			return err
		}
		// Only true is meaningful: the default moves to this item. Clearing it would leave no default.
		if patch.IsDefault != nil && *patch.IsDefault && !item.IsDefault {
			return s.store.SetDefaultItem(ctx, orgID, kind, item.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.store.FindItem(ctx, orgID, kind, id)
}

// DeleteCatalogItem removes an item. Items in use need reassignTo: the issues move to it in the same
// transaction. The last item of a required kind cannot be removed. Owners and admins only.
func (s *CatalogService) DeleteCatalogItem(ctx context.Context, actorID, orgID uuid.UUID, kind domain.CatalogKind, id uuid.UUID, reassignTo *uuid.UUID) error {
	if err := s.roles.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return err
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.store.LockOrganization(ctx, orgID); err != nil {
			return err
		}
		item, err := s.store.FindItem(ctx, orgID, kind, id)
		if err != nil {
			return err
		}
		items, err := s.store.ListItems(ctx, orgID, kind)
		if err != nil {
			return err
		}
		if err := domain.CheckRemovalKeepsRequired(items, *item); err != nil {
			return err
		}
		if item.Usage > 0 {
			if reassignTo == nil {
				return domain.ErrCatalogInUse
			}
			target, err := s.store.FindItem(ctx, orgID, kind, *reassignTo)
			if err != nil {
				if apperror.KindOf(err) == apperror.KindNotFound {
					return domain.ErrCatalogReassignTarget
				}
				return err
			}
			if err := domain.ReassignTargetValid(*item, *target); err != nil {
				return err
			}
			if _, err := s.store.MoveIssues(ctx, item, target); err != nil {
				return err
			}
		}
		if err := s.store.DeleteItem(ctx, item); err != nil {
			return err
		}
		if item.IsDefault {
			// The first remaining item by position takes over the default flag.
			for _, candidate := range items {
				if candidate.ID != item.ID {
					return s.store.SetDefaultItem(ctx, orgID, kind, candidate.ID)
				}
			}
		}
		return nil
	})
}

// ReorderCatalog stores the positions of the items. ids must list every item of the catalog exactly once.
func (s *CatalogService) ReorderCatalog(ctx context.Context, actorID, orgID uuid.UUID, kind domain.CatalogKind, ids []uuid.UUID) ([]*domain.CatalogItem, error) {
	if err := s.roles.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return nil, err
	}
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.store.LockOrganization(ctx, orgID); err != nil {
			return err
		}
		items, err := s.store.ListItems(ctx, orgID, kind)
		if err != nil {
			return err
		}
		if err := domain.ValidateReorder(items, ids); err != nil {
			return err
		}
		return s.store.SetPositions(ctx, orgID, kind, ids)
	})
	if err != nil {
		return nil, err
	}
	return s.store.ListItems(ctx, orgID, kind)
}

// ProjectIssueTypes returns the issue types a project uses. The flag reports whether the project has its own
// configuration; without it every issue type of the organization applies.
func (s *CatalogService) ProjectIssueTypes(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]*domain.CatalogItem, bool, error) {
	if err := s.authorizer.Authorize(ctx, actionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, false, err
	}
	return s.store.ProjectIssueTypes(ctx, projectID)
}

// ReplaceProjectIssueTypes sets the issue types of a project, in the given order. The set must keep one
// standard type, the epic-level type and the subtask-level type. Requires edit_project.
func (s *CatalogService) ReplaceProjectIssueTypes(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, issueTypeIDs []uuid.UUID) ([]*domain.CatalogItem, error) {
	if err := s.authorizer.Authorize(ctx, actionEditProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		orgID, err := s.store.ProjectOrganization(ctx, projectID)
		if err != nil {
			return err
		}
		if err := s.store.LockOrganization(ctx, orgID); err != nil {
			return err
		}
		types, err := s.store.ListItems(ctx, orgID, domain.CatalogIssueType)
		if err != nil {
			return err
		}
		byID := make(map[uuid.UUID]*domain.CatalogItem, len(types))
		for _, item := range types {
			byID[item.ID] = item
		}
		seen := make(map[uuid.UUID]bool, len(issueTypeIDs))
		var unique []uuid.UUID
		levels := map[int]int{}
		for _, id := range issueTypeIDs {
			item, ok := byID[id]
			if !ok {
				return domain.ErrCatalogReassignTarget
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			unique = append(unique, id)
			levels[item.HierarchyLevel]++
		}
		if levels[domain.HierarchyStandard] == 0 || levels[domain.HierarchyEpic] == 0 || levels[domain.HierarchySubtask] == 0 {
			return domain.ErrProjectTypesRequired
		}
		return s.store.ReplaceProjectIssueTypes(ctx, projectID, unique)
	})
	if err != nil {
		return nil, err
	}
	items, _, err := s.store.ProjectIssueTypes(ctx, projectID)
	return items, err
}

// normalizeItem trims and validates the fields of an item.
func normalizeItem(item *domain.CatalogItem) error {
	name, err := domain.NormalizeCatalogName(item.Name, item.Kind)
	if err != nil {
		return err
	}
	description, err := domain.NormalizeCatalogDescription(item.Description)
	if err != nil {
		return err
	}
	item.Name, item.Description = name, description
	return domain.ValidateCatalogItem(*item)
}

// checkNameFree rejects a name already used by another item of the catalog, ignoring case.
func checkNameFree(items []*domain.CatalogItem, name string, exceptID uuid.UUID) error {
	for _, item := range items {
		if item.ID != exceptID && strings.EqualFold(item.Name, name) {
			return domain.ErrCatalogNameTaken
		}
	}
	return nil
}

package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// applyCreateCatalog resolves the type and priority of a new issue from the catalogs and fills the metadata.
func (s *IssueService) applyCreateCatalog(ctx context.Context, issue *domain.Issue, in port.CreateInput) error {
	if err := s.resolveTypeInto(ctx, issue, in.IssueTypeID, in.IssueType); err != nil {
		return err
	}
	if err := s.resolvePriorityInto(ctx, issue, in.PriorityID, in.Priority); err != nil {
		return err
	}
	return issue.UpdateMetadata(issue.IssueType, in.ParentID, in.DueDate, in.StoryPoints)
}

// resolveTypeInto sets the catalog type of the issue. With neither an id nor a legacy value the current type
// stays. Without a selector the organization default applies, restricted to the project's enabled types.
func (s *IssueService) resolveTypeInto(ctx context.Context, issue *domain.Issue, id *uuid.UUID, legacy domain.IssueType) error {
	if id == nil && legacy == "" {
		if issue.IssueTypeID != uuid.Nil {
			return nil
		}
	}
	sel, explicit := selectorFor(id, string(legacy))
	item, err := s.catalog.Select(ctx, issue.ProjectID, domain.CatalogIssueType, sel)
	if err != nil {
		if explicit && apperror.KindOf(err) == apperror.KindNotFound {
			return domain.ErrCatalogValueUnknown
		}
		return err
	}
	if explicit {
		enabled, err := s.catalog.IssueTypeEnabled(ctx, issue.ProjectID, item.ID)
		if err != nil {
			return err
		}
		if !enabled {
			return domain.ErrCatalogTypeNotEnabled
		}
	}
	issue.IssueTypeID = item.ID
	issue.IssueType = domain.LegacyIssueTypeValue(item)
	issue.IssueTypeRef = domain.RefOf(item)
	return nil
}

// resolvePriorityInto sets the catalog priority of the issue, with the same rules as resolveTypeInto.
func (s *IssueService) resolvePriorityInto(ctx context.Context, issue *domain.Issue, id *uuid.UUID, legacy domain.Priority) error {
	if id == nil && legacy == "" {
		if issue.PriorityID != uuid.Nil {
			return nil
		}
	}
	sel, explicit := selectorFor(id, string(legacy))
	item, err := s.catalog.Select(ctx, issue.ProjectID, domain.CatalogPriority, sel)
	if err != nil {
		if explicit && apperror.KindOf(err) == apperror.KindNotFound {
			return domain.ErrCatalogValueUnknown
		}
		return err
	}
	issue.PriorityID = item.ID
	issue.Priority = domain.LegacyPriorityValue(item)
	issue.PriorityRef = domain.RefOf(item)
	return nil
}

// applyResolution gives a done issue its default resolution when it has none. Reopening is handled by the domain.
func (s *IssueService) applyResolution(ctx context.Context, issue *domain.Issue) error {
	if s.catalog == nil || issue.ResolutionID != nil {
		return nil
	}
	if issue.Status != domain.StatusResolved && issue.Status != domain.StatusClosed {
		return nil
	}
	item, err := s.catalog.Select(ctx, issue.ProjectID, domain.CatalogResolution, port.CatalogSelector{Default: true})
	if err != nil {
		return err
	}
	id := item.ID
	issue.ResolutionID = &id
	issue.ResolutionRef = domain.RefOf(item)
	return nil
}

// selectorFor turns the optional id and legacy value of a request into a catalog selector. The boolean reports
// whether the caller named the item explicitly (as opposed to the organization default).
func selectorFor(id *uuid.UUID, legacy string) (port.CatalogSelector, bool) {
	if id != nil {
		return port.CatalogSelector{ID: *id}, true
	}
	if legacy != "" {
		return port.CatalogSelector{LegacyKey: legacy}, true
	}
	return port.CatalogSelector{Default: true}, false
}

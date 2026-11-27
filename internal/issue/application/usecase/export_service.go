package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// exportPageSize is the number of issues read per database round trip of an export.
const exportPageSize = 500

// ExportFileName is the download name of an export started now.
func (s *IssueService) ExportFileName() string {
	return domain.ExportFileName(s.now())
}

// ExportIssues streams the issues matching the filter that the caller can browse, at most MaxExportRows. start runs
// once before the first batch with whether rows were left out. Visibility is the project membership of the caller,
// the same rule the list endpoint applies; platform administrators see every project.
func (s *IssueService) ExportIssues(ctx context.Context, actorID uuid.UUID, globalRole string, filter port.ListFilter,
	start func(truncated bool) error, emit func(port.ExportBatch) error) error {
	if s.ops == nil || s.ops.exportLabels == nil {
		return errOperationsUnavailable()
	}
	filter.Page = 1
	if globalRole != "admin" {
		if s.projectRoles == nil {
			return start(false)
		}
		projectIDs, err := s.projectRoles.ListProjectIDs(ctx, actorID)
		if err != nil {
			return err
		}
		if len(projectIDs) == 0 {
			return start(false)
		}
		filter.ProjectIDs = projectIDs
	}
	total, err := s.repo.Count(ctx, filter)
	if err != nil {
		return err
	}
	limit := domain.MaxExportRows
	if err := start(total > limit); err != nil {
		return err
	}
	written := 0
	for page := 1; written < limit; page++ {
		pageFilter := filter
		pageFilter.Page = page
		pageFilter.PageSize = min(exportPageSize, limit-written)
		items, err := s.repo.FindAll(ctx, pageFilter)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		batch, err := s.exportBatch(ctx, items)
		if err != nil {
			return err
		}
		if err := emit(batch); err != nil {
			return err
		}
		written += len(items)
		if len(items) < pageFilter.PageSize {
			return nil
		}
	}
	return nil
}

// exportBatch loads the labels and the person names of one page with one statement each.
func (s *IssueService) exportBatch(ctx context.Context, items []*domain.Issue) (port.ExportBatch, error) {
	ids := make([]uuid.UUID, 0, len(items))
	for _, issue := range items {
		ids = append(ids, issue.ID)
	}
	labels, err := s.ops.exportLabels.LabelNamesByIssue(ctx, ids)
	if err != nil {
		return port.ExportBatch{}, err
	}
	var people []uuid.UUID
	for _, issue := range items {
		people = append(people, issue.ReporterID)
		if issue.AssigneeID != nil {
			people = append(people, *issue.AssigneeID)
		}
	}
	names, err := s.UserNames(ctx, people)
	if err != nil {
		return port.ExportBatch{}, err
	}
	return port.ExportBatch{Issues: items, Labels: labels, Names: names}, nil
}

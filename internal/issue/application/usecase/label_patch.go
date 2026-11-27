package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// applyLabels replaces the labels of the issue with the given names. Names that do not exist in the project are
// created (create-on-type); the caller must already be allowed to edit the issue. nil or empty names clear the
// labels. The change is recorded only when the set of labels differs.
func (s *IssueService) applyLabels(ctx context.Context, issue *domain.Issue, names []string, add func(field, from, to string)) error {
	if s.labels == nil {
		return apperror.New(apperror.KindInternal, "labels are not available")
	}
	normalized, err := domain.NormalizeLabelNames(names)
	if err != nil {
		return err
	}
	current, err := s.labels.IssueLabelIDs(ctx, issue.ID)
	if err != nil {
		return err
	}
	next, err := s.labels.ResolveLabelNames(ctx, issue.ProjectID, normalized)
	if err != nil {
		return err
	}
	beforeText := labelIDsText(current)
	afterText := labelIDsText(next)
	if beforeText == afterText {
		return nil
	}
	if err := s.labels.ReplaceIssueLabels(ctx, issue.ID, next); err != nil {
		return err
	}
	add("labels", beforeText, afterText)
	return nil
}

// labelIDsText renders the ids of a label set as the stored change value, independent of the input order.
func labelIDsText(ids []uuid.UUID) string {
	texts := make([]string, 0, len(ids))
	for _, id := range ids {
		texts = append(texts, id.String())
	}
	return domain.LabelIDsText(texts)
}

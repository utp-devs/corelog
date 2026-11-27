package domain

import (
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Clone options: the parts of an issue a clone copies besides its fields.
const (
	CloneIncludeAttachments = "attachments"
	CloneIncludeLinks       = "links"
	CloneIncludeSubtasks    = "subtasks"
	CloneIncludeWatchers    = "watchers"
)

// LinkKindClones is the link kind that every clone keeps back to its original.
const LinkKindClones IssueLinkKind = "clones"

var ErrCloneOptionInvalid = apperror.WithCode(apperror.KindValidation, "issue.clone_option_invalid", "la opcion de clonado no es valida")

// ParseCloneIncludes checks the requested clone options and drops repeated ones.
func ParseCloneIncludes(raw []string) (map[string]bool, error) {
	out := make(map[string]bool, len(raw))
	for _, option := range raw {
		switch option {
		case CloneIncludeAttachments, CloneIncludeLinks, CloneIncludeSubtasks, CloneIncludeWatchers:
			out[option] = true
		default:
			return nil, ErrCloneOptionInvalid
		}
	}
	return out, nil
}

// ErrConvertParentRequired is returned when a conversion needs the caller to choose the parent.
var ErrConvertParentRequired = apperror.WithCode(apperror.KindValidation, "issue.convert_parent_required", "la conversion requiere indicar la incidencia padre o quitarla")

// DefaultCloneSummary is the summary of a clone that does not name one.
func DefaultCloneSummary(original string) string {
	summary := "Copia de " + original
	if utf8.RuneCountInString(summary) <= MaxSummaryLength {
		return summary
	}
	runes := []rune(summary)
	return string(runes[:MaxSummaryLength])
}

// CloneDraft builds the unsaved copy of an issue in the target project. The key, the rank and the links are
// assigned by the repositories and the use case; the copy starts open and is reported by reporterID.
func CloneDraft(source *Issue, targetProjectID, reporterID uuid.UUID, summary string, now time.Time) *Issue {
	clone := &Issue{
		ProjectID:                targetProjectID,
		Summary:                  summary,
		Description:              source.Description,
		Status:                   StatusOpen,
		Priority:                 source.Priority,
		PriorityID:               source.PriorityID,
		PriorityRef:              source.PriorityRef,
		IssueType:                source.IssueType,
		IssueTypeID:              source.IssueTypeID,
		IssueTypeRef:             source.IssueTypeRef,
		ParentID:                 source.ParentID,
		DueDate:                  source.DueDate,
		StoryPoints:              source.StoryPoints,
		Rank:                     "m",
		ReporterID:               reporterID,
		Custom:                   map[string]any{},
		OriginalEstimateSeconds:  source.OriginalEstimateSeconds,
		RemainingEstimateSeconds: source.RemainingEstimateSeconds,
		StartDate:                source.StartDate,
		Flagged:                  source.Flagged,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	for key, value := range source.Custom {
		clone.Custom[key] = value
	}
	return clone
}

package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

type IssueLinkKind string

const (
	IssueLinkBlocks     IssueLinkKind = "blocks"
	IssueLinkRelates    IssueLinkKind = "relates"
	IssueLinkDuplicates IssueLinkKind = "duplicates"
)

var (
	ErrInvalidIssueLink = apperror.WithCode(apperror.KindValidation, "issue.invalid_link", "invalid issue link")
	ErrDuplicateLink    = apperror.WithCode(apperror.KindConflict, "issue.link_duplicate", "issue link already exists")
)

type IssueLink struct {
	ID            uuid.UUID
	SourceIssueID uuid.UUID
	TargetIssueID uuid.UUID
	Kind          IssueLinkKind
	CreatedBy     uuid.UUID
	CreatedAt     time.Time
}

func NewIssueLink(sourceIssueID, targetIssueID, createdBy uuid.UUID, kind IssueLinkKind) (*IssueLink, error) {
	if sourceIssueID == uuid.Nil || targetIssueID == uuid.Nil || createdBy == uuid.Nil || sourceIssueID == targetIssueID {
		return nil, ErrInvalidIssueLink
	}
	switch kind {
	case IssueLinkBlocks, IssueLinkRelates, IssueLinkDuplicates:
	default:
		return nil, ErrInvalidIssueLink
	}
	return &IssueLink{
		SourceIssueID: sourceIssueID, TargetIssueID: targetIssueID,
		Kind: kind, CreatedBy: createdBy, CreatedAt: time.Now().UTC(),
	}, nil
}

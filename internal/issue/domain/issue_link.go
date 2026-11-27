package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// IssueLinkKind is the stored kind of a link. A link is stored one-directional from its source to its target;
// the inverse wording is shown from the target side (see the label table in link_kind.go).
type IssueLinkKind string

const (
	IssueLinkBlocks     IssueLinkKind = "blocks"
	IssueLinkRelates    IssueLinkKind = "relates"
	IssueLinkDuplicates IssueLinkKind = "duplicates"
	IssueLinkClones     IssueLinkKind = "clones"
	IssueLinkCauses     IssueLinkKind = "causes"
	IssueLinkImplements IssueLinkKind = "implements"
)

var (
	ErrInvalidIssueLink  = apperror.WithCode(apperror.KindValidation, "issue.invalid_link", "el enlace no es valido")
	ErrUnknownLinkKind   = apperror.WithCode(apperror.KindValidation, "issue.link_kind_invalid", "el tipo de enlace no es valido")
	ErrSelfLink          = apperror.WithCode(apperror.KindValidation, "issue.link_self", "una incidencia no puede enlazarse consigo misma")
	ErrDuplicateLink     = apperror.WithCode(apperror.KindConflict, "issue.link_duplicate", "las incidencias ya estan enlazadas con ese tipo de enlace")
	ErrLinkCycle         = apperror.WithCode(apperror.KindConflict, "issue.link_cycle", "el enlace crearia un ciclo de bloqueos")
	ErrIssueLinkNotFound = apperror.WithCode(apperror.KindNotFound, "issue.link_not_found", "el enlace no existe")
)

// IssueLink is one directed link between two issues.
type IssueLink struct {
	ID            uuid.UUID
	SourceIssueID uuid.UUID
	TargetIssueID uuid.UUID
	Kind          IssueLinkKind
	CreatedBy     uuid.UUID
	CreatedAt     time.Time
}

// NewIssueLink builds a link from source to target. The caller passes the clock reading as createdAt.
func NewIssueLink(sourceIssueID, targetIssueID, createdBy uuid.UUID, kind IssueLinkKind, createdAt time.Time) (*IssueLink, error) {
	if sourceIssueID == uuid.Nil || targetIssueID == uuid.Nil || createdBy == uuid.Nil {
		return nil, ErrInvalidIssueLink
	}
	if sourceIssueID == targetIssueID {
		return nil, ErrSelfLink
	}
	if !kind.Valid() {
		return nil, ErrUnknownLinkKind
	}
	return &IssueLink{
		SourceIssueID: sourceIssueID, TargetIssueID: targetIssueID,
		Kind: kind, CreatedBy: createdBy, CreatedAt: createdAt.UTC(),
	}, nil
}

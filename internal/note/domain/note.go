// Package domain holds the personal note rules. A note belongs to one user and is never shared.
package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

const (
	MaxTitleLength = 120
	MaxBodyLength  = 20000
	MaxTags        = 10
	MaxTagLength   = 30
	DefaultColor   = "yellow"
)

// Colors is the fixed palette a note can use. Keys are stored as sent; clients map them to shades.
var Colors = []string{"yellow", "blue", "green", "pink", "purple", "orange", "red", "gray"}

var (
	ErrNotFound      = apperror.WithCode(apperror.KindNotFound, "note.not_found", "nota no encontrada")
	ErrLinkForbidden = apperror.WithCode(apperror.KindNotFound, "note.link_forbidden", "el elemento a vincular no existe o no puedes verlo")
	ErrInvalid       = apperror.WithCode(apperror.KindValidation, "note.invalid", "datos de la nota no validos")
)

// invalid returns a validation error that names the offending field.
func invalid(field, message string) error {
	return apperror.WithCode(apperror.KindValidation, "note.invalid", "datos de la nota no validos").
		WithDetails(apperror.FieldError{Field: field, Message: message})
}

// Fields is the user-editable content of a note at creation time.
type Fields struct {
	Title     string
	Body      string
	Color     string
	Tags      []string
	Pinned    bool
	IssueID   *uuid.UUID
	ProjectID *uuid.UUID
}

// Patch is a partial update. A nil field keeps the current value. ClearIssue and ClearProject remove a link.
type Patch struct {
	Title        *string
	Body         *string
	Color        *string
	Tags         *[]string
	IssueID      *uuid.UUID
	ClearIssue   bool
	ProjectID    *uuid.UUID
	ClearProject bool
}

// Note is one private note.
type Note struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Title     string
	Body      string
	Color     string
	Pinned    bool
	Tags      []string
	IssueID   *uuid.UUID
	ProjectID *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewNote validates the fields and builds a note stamped with now.
func NewNote(ownerID uuid.UUID, f Fields, now time.Time) (Note, error) {
	if ownerID == uuid.Nil {
		return Note{}, ErrInvalid
	}
	n := Note{OwnerID: ownerID, Pinned: f.Pinned, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
	if err := n.setTitle(f.Title); err != nil {
		return Note{}, err
	}
	if err := n.setBody(f.Body); err != nil {
		return Note{}, err
	}
	if err := n.setColor(f.Color); err != nil {
		return Note{}, err
	}
	tags, err := NormalizeTags(f.Tags)
	if err != nil {
		return Note{}, err
	}
	n.Tags = tags
	if err := n.setLinks(f.IssueID, f.ProjectID, false, false); err != nil {
		return Note{}, err
	}
	return n, nil
}

// ApplyPatch changes the fields present in the patch and refreshes UpdatedAt. The whole patch is validated
// before any field changes, so a failing patch leaves the note untouched.
func (n *Note) ApplyPatch(p Patch, now time.Time) error {
	staged := *n
	staged.Tags = append([]string(nil), n.Tags...)
	if p.Title != nil {
		if err := staged.setTitle(*p.Title); err != nil {
			return err
		}
	}
	if p.Body != nil {
		if err := staged.setBody(*p.Body); err != nil {
			return err
		}
	}
	if p.Color != nil {
		if err := staged.setColor(*p.Color); err != nil {
			return err
		}
	}
	if p.Tags != nil {
		tags, err := NormalizeTags(*p.Tags)
		if err != nil {
			return err
		}
		staged.Tags = tags
	}
	if err := staged.setLinks(p.IssueID, p.ProjectID, p.ClearIssue, p.ClearProject); err != nil {
		return err
	}
	staged.UpdatedAt = now.UTC()
	*n = staged
	return nil
}

// SetPinned pins or unpins the note and refreshes UpdatedAt.
func (n *Note) SetPinned(pinned bool, now time.Time) {
	n.Pinned = pinned
	n.UpdatedAt = now.UTC()
}

// IssueLinkChanged reports whether the patch points the note at a different issue than it has now.
func (n Note) IssueLinkChanged(p Patch) bool {
	if p.ClearIssue {
		return n.IssueID != nil
	}
	return p.IssueID != nil && (n.IssueID == nil || *n.IssueID != *p.IssueID)
}

// ProjectLinkChanged reports whether the patch points the note at a different project than it has now.
func (n Note) ProjectLinkChanged(p Patch) bool {
	if p.ClearProject {
		return n.ProjectID != nil
	}
	return p.ProjectID != nil && (n.ProjectID == nil || *n.ProjectID != *p.ProjectID)
}

func (n *Note) setTitle(raw string) error {
	title := strings.TrimSpace(raw)
	if utf8.RuneCountInString(title) > MaxTitleLength {
		return invalid("title", "el titulo admite como maximo 120 caracteres")
	}
	n.Title = title
	return nil
}

func (n *Note) setBody(body string) error {
	if utf8.RuneCountInString(body) > MaxBodyLength {
		return invalid("body", "el cuerpo admite como maximo 20000 caracteres")
	}
	n.Body = body
	return nil
}

func (n *Note) setColor(raw string) error {
	color := strings.TrimSpace(raw)
	if color == "" {
		color = DefaultColor
	}
	if !IsValidColor(color) {
		return invalid("color", "color no reconocido")
	}
	n.Color = color
	return nil
}

func (n *Note) setLinks(issueID, projectID *uuid.UUID, clearIssue, clearProject bool) error {
	if clearIssue {
		n.IssueID = nil
	}
	if clearProject {
		n.ProjectID = nil
	}
	if issueID != nil {
		if *issueID == uuid.Nil {
			return invalid("issue_id", "identificador no valido")
		}
		id := *issueID
		n.IssueID = &id
	}
	if projectID != nil {
		if *projectID == uuid.Nil {
			return invalid("project_id", "identificador no valido")
		}
		id := *projectID
		n.ProjectID = &id
	}
	return nil
}

// IsValidColor reports whether key belongs to the palette.
func IsValidColor(key string) bool {
	for _, c := range Colors {
		if c == key {
			return true
		}
	}
	return false
}

// NormalizeTags trims and lowercases each tag, drops duplicates keeping the first occurrence and checks
// the count and length limits. A nil or empty input returns an empty, non-nil slice.
func NormalizeTags(raw []string) ([]string, error) {
	tags := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		tag := strings.ToLower(strings.TrimSpace(item))
		if tag == "" {
			return nil, invalid("tags", "las etiquetas no pueden estar vacias")
		}
		if utf8.RuneCountInString(tag) > MaxTagLength {
			return nil, invalid("tags", "cada etiqueta admite como maximo 30 caracteres")
		}
		if _, dup := seen[tag]; dup {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	if len(tags) > MaxTags {
		return nil, invalid("tags", "una nota admite como maximo 10 etiquetas")
	}
	return tags, nil
}

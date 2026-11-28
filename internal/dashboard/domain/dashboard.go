// Package domain holds the dashboard entities, the gadget catalog and their configuration rules. It is pure: no
// persistence and no transport.
package domain

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Stable error codes of the dashboards.
const (
	CodeInvalidDashboard = "dashboard.invalid"
	CodeInvalidGadget    = "dashboard.invalid_gadget"
	CodeInvalidShare     = "dashboard.invalid_share"
	CodeNotFound         = "dashboard.not_found"
	CodeGadgetNotFound   = "dashboard.gadget_not_found"
	CodeNameTaken        = "dashboard.name_taken"
	CodeForbidden        = "dashboard.forbidden"
	CodeNoOrganization   = "dashboard.no_organization"
	CodeSourceFailed     = "dashboard.source_unavailable"
)

// Limits.
const (
	MaxGadgets     = 20
	MaxName        = 120
	MaxDescription = 500
	MaxTitle       = 120
	MaxShares      = 100
	MaxConfigBytes = 16 << 10
)

// Layout is the column template of a dashboard: one, two or three equal columns, or two columns where the first
// ("2-1") or the second ("1-2") is twice as wide.
type Layout string

const (
	Layout1  Layout = "1"
	Layout2  Layout = "2"
	Layout3  Layout = "3"
	Layout21 Layout = "2-1"
	Layout12 Layout = "1-2"
)

// Valid reports whether l is a known template.
func (l Layout) Valid() bool {
	switch l {
	case Layout1, Layout2, Layout3, Layout21, Layout12:
		return true
	}
	return false
}

// Columns returns how many columns the template has.
func (l Layout) Columns() int {
	switch l {
	case Layout1:
		return 1
	case Layout3:
		return 3
	}
	return 2
}

// Gadget is one widget of a dashboard.
type Gadget struct {
	ID          uuid.UUID
	Type        string
	Title       string
	Config      json.RawMessage
	Position    int
	ColumnIndex int
}

// Dashboard is a set of gadgets arranged in columns.
type Dashboard struct {
	ID             uuid.UUID
	OwnerID        uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Description    string
	Layout         Layout
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Gadgets        []Gadget
}

// ShareType says what a share's target is.
type ShareType string

const (
	ShareUser         ShareType = "user"
	ShareTeam         ShareType = "team"
	ShareProject      ShareType = "project"
	ShareOrganization ShareType = "organization"
)

// Permission is what a share grants.
type Permission string

const (
	PermissionView Permission = "view"
	PermissionEdit Permission = "edit"
)

// Share grants a user, team, project audience or the organization access to a dashboard.
type Share struct {
	Type       ShareType
	TargetID   uuid.UUID
	Permission Permission
}

// Access is what a viewer can do with a dashboard.
type Access int

const (
	AccessNone Access = iota
	AccessView
	AccessEdit
	AccessOwner
)

// ResolveAccess combines ownership with the best share permission ("" when no share reaches the viewer).
func ResolveAccess(isOwner bool, shared Permission) Access {
	switch {
	case isOwner:
		return AccessOwner
	case shared == PermissionEdit:
		return AccessEdit
	case shared == PermissionView:
		return AccessView
	}
	return AccessNone
}

// CanEdit reports whether the content (name, layout, gadgets) can be changed.
func (a Access) CanEdit() bool { return a >= AccessEdit }

// CanManage reports whether the dashboard can be deleted, shared or made the default.
func (a Access) CanManage() bool { return a == AccessOwner }

// String is the name used in responses.
func (a Access) String() string {
	switch a {
	case AccessOwner:
		return "owner"
	case AccessEdit:
		return "edit"
	case AccessView:
		return "view"
	}
	return "none"
}

var (
	// ErrNotFound hides dashboards the viewer cannot see.
	ErrNotFound = apperror.WithCode(apperror.KindNotFound, CodeNotFound, "panel no encontrado")
	// ErrGadgetNotFound is returned when a gadget id does not belong to the dashboard.
	ErrGadgetNotFound = apperror.WithCode(apperror.KindNotFound, CodeGadgetNotFound, "gadget no encontrado")
	// ErrForbidden is returned when the viewer sees the dashboard but cannot do the action.
	ErrForbidden = apperror.WithCode(apperror.KindForbidden, CodeForbidden, "no tienes permiso para esta acción sobre el panel")
	// ErrNameTaken is returned when the owner already has a dashboard with that name.
	ErrNameTaken = apperror.WithCode(apperror.KindConflict, CodeNameTaken, "ya tienes un panel con ese nombre")
	// ErrNoOrganization is returned when the user belongs to no organization.
	ErrNoOrganization = apperror.WithCode(apperror.KindValidation, CodeNoOrganization, "no perteneces a ninguna organización")
)

func invalid(code, field, message string) *apperror.Error {
	return apperror.WithCode(apperror.KindValidation, code, message).WithDetails(apperror.FieldError{Field: field, Message: message})
}

// NormalizeName trims and validates a dashboard name.
func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", invalid(CodeInvalidDashboard, "name", "el nombre es obligatorio")
	}
	if utf8.RuneCountInString(name) > MaxName {
		return "", invalid(CodeInvalidDashboard, "name", "el nombre admite como máximo 120 caracteres")
	}
	return name, nil
}

// NormalizeDescription trims and validates a description.
func NormalizeDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > MaxDescription {
		return "", invalid(CodeInvalidDashboard, "description", "la descripción admite como máximo 500 caracteres")
	}
	return description, nil
}

// ParseLayout validates a layout text; empty means the default two columns.
func ParseLayout(text string) (Layout, error) {
	if text == "" {
		return Layout2, nil
	}
	l := Layout(text)
	if !l.Valid() {
		return "", invalid(CodeInvalidDashboard, "layout", "el diseño debe ser 1, 2, 3, 2-1 o 1-2")
	}
	return l, nil
}

// GadgetInput is a gadget as the client sends it. ID is set for gadgets that already exist.
type GadgetInput struct {
	ID          *uuid.UUID
	Type        string
	Title       string
	Config      json.RawMessage
	ColumnIndex int
	Position    *int
}

// NormalizeGadgets validates the full gadget list of a dashboard and returns it ordered by column and position
// with consecutive positions per column. Existing ids must be in existing.
func NormalizeGadgets(layout Layout, in []GadgetInput, existing map[uuid.UUID]bool) ([]Gadget, error) {
	if len(in) > MaxGadgets {
		return nil, invalid(CodeInvalidGadget, "gadgets", "un panel admite como máximo 20 gadgets")
	}
	columns := layout.Columns()
	type entry struct {
		g     Gadget
		order int
	}
	entries := make([]entry, 0, len(in))
	seen := map[uuid.UUID]bool{}
	for i, g := range in {
		field := "gadgets[" + itoa(i) + "]"
		if g.ID != nil {
			if !existing[*g.ID] {
				return nil, apperror.WithCode(apperror.KindValidation, CodeGadgetNotFound, "el gadget no pertenece al panel").
					WithDetails(apperror.FieldError{Field: field + ".id", Message: "el gadget no pertenece al panel"})
			}
			if seen[*g.ID] {
				return nil, invalid(CodeInvalidGadget, field+".id", "el gadget está repetido")
			}
			seen[*g.ID] = true
		}
		if g.ColumnIndex < 0 || g.ColumnIndex >= columns {
			return nil, invalid(CodeInvalidGadget, field+".column_index", "la columna no existe en el diseño elegido")
		}
		title := strings.TrimSpace(g.Title)
		if utf8.RuneCountInString(title) > MaxTitle {
			return nil, invalid(CodeInvalidGadget, field+".title", "el título admite como máximo 120 caracteres")
		}
		if !KnownGadget(g.Type) {
			return nil, invalid(CodeInvalidGadget, field+".type", "tipo de gadget desconocido")
		}
		if title == "" {
			title = DefaultTitle(g.Type)
		}
		config, err := NormalizeConfig(g.Type, g.Config)
		if err != nil {
			return nil, prefixField(err, field+".config")
		}
		pos := i
		if g.Position != nil {
			pos = *g.Position
		}
		gadget := Gadget{Type: g.Type, Title: title, Config: config, ColumnIndex: g.ColumnIndex}
		if g.ID != nil {
			gadget.ID = *g.ID
		}
		entries = append(entries, entry{g: gadget, order: pos*1000 + i})
	}
	// Stable order by column, then requested position, then input order.
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && less(entries[j].g, entries[j].order, entries[j-1].g, entries[j-1].order); j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
	out := make([]Gadget, len(entries))
	next := map[int]int{}
	for i, e := range entries {
		e.g.Position = next[e.g.ColumnIndex]
		next[e.g.ColumnIndex]++
		out[i] = e.g
	}
	return out, nil
}

func less(a Gadget, ao int, b Gadget, bo int) bool {
	if a.ColumnIndex != b.ColumnIndex {
		return a.ColumnIndex < b.ColumnIndex
	}
	return ao < bo
}

func prefixField(err error, prefix string) error {
	var ae *apperror.Error
	if e, ok := err.(*apperror.Error); ok {
		ae = e
	} else {
		return err
	}
	out := apperror.WithCode(ae.Kind, ae.Code, ae.Message)
	for _, d := range ae.Details {
		field := prefix
		if d.Field != "" {
			field += "." + d.Field
		}
		out.WithDetails(apperror.FieldError{Field: field, Message: d.Message})
	}
	if len(ae.Details) == 0 {
		out.WithDetails(apperror.FieldError{Field: prefix, Message: ae.Message})
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// NormalizeShares validates a share list: known types and permissions, no duplicates, the organization share
// targeting the dashboard's organization, and not the owner themselves.
func NormalizeShares(shares []Share, organizationID, ownerID uuid.UUID) ([]Share, error) {
	if len(shares) > MaxShares {
		return nil, invalid(CodeInvalidShare, "shares", "un panel admite como máximo 100 comparticiones")
	}
	seen := map[[2]string]bool{}
	out := make([]Share, 0, len(shares))
	for i, s := range shares {
		field := "shares[" + itoa(i) + "]"
		switch s.Type {
		case ShareUser, ShareTeam, ShareProject, ShareOrganization:
		default:
			return nil, invalid(CodeInvalidShare, field+".type", "el tipo debe ser user, team, project u organization")
		}
		if s.Permission == "" {
			s.Permission = PermissionView
		}
		if s.Permission != PermissionView && s.Permission != PermissionEdit {
			return nil, invalid(CodeInvalidShare, field+".permission", "el permiso debe ser view o edit")
		}
		if s.TargetID == uuid.Nil {
			return nil, invalid(CodeInvalidShare, field+".target_id", "falta el destino")
		}
		if s.Type == ShareOrganization {
			s.TargetID = organizationID
		}
		if s.Type == ShareUser && s.TargetID == ownerID {
			return nil, invalid(CodeInvalidShare, field+".target_id", "no hace falta compartir un panel con su propietario")
		}
		key := [2]string{string(s.Type), s.TargetID.String()}
		if seen[key] {
			return nil, invalid(CodeInvalidShare, field, "la compartición está repetida")
		}
		seen[key] = true
		out = append(out, s)
	}
	return out, nil
}

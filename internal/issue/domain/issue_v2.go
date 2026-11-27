package domain

import (
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Limits of the issue fields.
const (
	MaxSummaryLength     = 255
	MaxDescriptionLength = 100000
	MaxStoryPoints       = 999.5
	// MaxEstimateSeconds is one hundred years: far above any real estimate and safely inside BIGINT.
	MaxEstimateSeconds int64 = 3_153_600_000
)

var (
	ErrSummaryTooLong        = apperror.WithCode(apperror.KindValidation, "issue.summary_too_long", "el resumen admite como maximo 255 caracteres")
	ErrDescriptionTooLong    = apperror.WithCode(apperror.KindValidation, "issue.description_too_long", "la descripcion admite como maximo 100000 caracteres")
	ErrInvalidEstimate       = apperror.WithCode(apperror.KindValidation, "issue.invalid_estimate", "la estimacion debe ser un numero de segundos mayor o igual a cero")
	ErrInvalidDates          = apperror.WithCode(apperror.KindValidation, "issue.invalid_dates", "la fecha de inicio no puede ser posterior a la fecha de vencimiento")
	ErrStale                 = apperror.WithCode(apperror.KindConflict, "issue.stale", "la incidencia cambio desde que la abriste; recargala e intenta de nuevo")
	ErrFieldNotClearable     = apperror.WithCode(apperror.KindValidation, "issue.field_not_clearable", "este campo no admite null")
	ErrEpicHasParent         = apperror.WithCode(apperror.KindValidation, "issue.hierarchy_epic_parent", "una epica no puede tener incidencia padre")
	ErrSubtaskNeedsParent    = apperror.WithCode(apperror.KindValidation, "issue.hierarchy_subtask_parent_required", "una subtarea requiere una incidencia padre")
	ErrSubtaskParentInvalid  = apperror.WithCode(apperror.KindValidation, "issue.hierarchy_subtask_parent_invalid", "el padre de una subtarea no puede ser una epica ni otra subtarea")
	ErrStandardParentInvalid = apperror.WithCode(apperror.KindValidation, "issue.hierarchy_standard_parent_invalid", "el padre de una incidencia estandar solo puede ser una epica")
	ErrParentOtherProject    = apperror.WithCode(apperror.KindValidation, "issue.hierarchy_parent_other_project", "el padre debe estar en el mismo proyecto")
	ErrParentCycle           = apperror.WithCode(apperror.KindValidation, "issue.hierarchy_cycle", "la jerarquia no admite ciclos")
	ErrParentNotFound        = apperror.WithCode(apperror.KindValidation, "issue.parent_not_found", "la incidencia padre no existe")
	ErrChildrenInvalid       = apperror.WithCode(apperror.KindValidation, "issue.hierarchy_children_invalid", "el cambio de tipo dejaria hijos con una jerarquia invalida")
	ErrIssueNotDeleted       = apperror.WithCode(apperror.KindConflict, "issue.not_deleted", "la incidencia no esta eliminada")
)

// Change is one field that a patch modified. From and To are the textual values (ids for references, dates as
// YYYY-MM-DD) and nil when the field had no value before or has none after.
type Change struct {
	Field string
	From  *string
	To    *string
}

// NewChange builds a change. Empty strings count as no value.
func NewChange(field, from, to string) Change {
	c := Change{Field: field}
	if from != "" {
		c.From = &from
	}
	if to != "" {
		c.To = &to
	}
	return c
}

// NormalizeSummary trims the summary and checks that it is present and within the limit.
func NormalizeSummary(summary string) (string, error) {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return "", ErrInvalidSummary
	}
	if utf8.RuneCountInString(summary) > MaxSummaryLength {
		return "", ErrSummaryTooLong
	}
	return summary, nil
}

// NormalizeDescription trims the markdown description. An empty description is valid.
func NormalizeDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > MaxDescriptionLength {
		return "", ErrDescriptionTooLong
	}
	return description, nil
}

// ValidateStoryPoints accepts nil, or a value from 0 to 999.5 in steps of 0.5.
func ValidateStoryPoints(points *float64) error {
	if points == nil {
		return nil
	}
	v := *points
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > MaxStoryPoints || math.Mod(v*2, 1) != 0 {
		return ErrInvalidStoryPoints
	}
	return nil
}

// ValidateEstimate accepts nil, or a number of seconds from 0 to MaxEstimateSeconds.
func ValidateEstimate(seconds *int64) error {
	if seconds == nil {
		return nil
	}
	if *seconds < 0 || *seconds > MaxEstimateSeconds {
		return ErrInvalidEstimate
	}
	return nil
}

// DateOnly truncates a time to its UTC calendar date.
func DateOnly(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// FormatDate renders a date as YYYY-MM-DD, or "" for nil.
func FormatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

// ValidateDates checks that the start date is not after the due date.
func ValidateDates(start, due *time.Time) error {
	if start != nil && due != nil && DateOnly(*start).After(DateOnly(*due)) {
		return ErrInvalidDates
	}
	return nil
}

// HierarchyLevel is the level of an issue in the hierarchy: epic above standard above subtask.
// It reads the catalog type when it is known and falls back to the legacy type.
func (t *Issue) HierarchyLevel() int {
	if t.IssueTypeRef != nil {
		return t.IssueTypeRef.HierarchyLevel
	}
	return LegacyHierarchyLevel(t.IssueType)
}

// LegacyHierarchyLevel maps the legacy enum to a hierarchy level.
func LegacyHierarchyLevel(t IssueType) int {
	switch t {
	case IssueTypeEpic:
		return HierarchyEpic
	case IssueTypeSubtask:
		return HierarchySubtask
	default:
		return HierarchyStandard
	}
}

// ParentRule checks whether an issue of childLevel may sit under a parent of parentLevel (hasParent false
// means no parent). sameProject is only read when there is a parent.
func ParentRule(childLevel int, hasParent bool, parentLevel int, sameProject bool) error {
	switch childLevel {
	case HierarchyEpic:
		if hasParent {
			return ErrEpicHasParent
		}
	case HierarchySubtask:
		if !hasParent {
			return ErrSubtaskNeedsParent
		}
		if parentLevel != HierarchyStandard {
			return ErrSubtaskParentInvalid
		}
	default:
		if hasParent && parentLevel != HierarchyEpic {
			return ErrStandardParentInvalid
		}
	}
	if hasParent && !sameProject {
		return ErrParentOtherProject
	}
	return nil
}

// HierarchyNode is the part of an issue that the hierarchy rules read.
type HierarchyNode struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Level     int
	ParentID  *uuid.UUID
}

// CheckParent validates the parent of child. parent is nil when the child has none. ancestors are the parents of
// parent from the nearest to the root; they only serve the cycle check.
func CheckParent(child HierarchyNode, parent *HierarchyNode, ancestors []uuid.UUID) error {
	if parent == nil {
		return ParentRule(child.Level, false, 0, true)
	}
	if parent.ID == child.ID {
		return ErrParentCycle
	}
	for _, id := range ancestors {
		if id == child.ID {
			return ErrParentCycle
		}
	}
	return ParentRule(child.Level, true, parent.Level, parent.ProjectID == child.ProjectID)
}

// CheckChildren validates that every child level stays valid under a parent of parentLevel.
func CheckChildren(parentLevel int, childLevels []int) error {
	for _, level := range childLevels {
		if err := ParentRule(level, true, parentLevel, true); err != nil {
			return ErrChildrenInvalid
		}
	}
	return nil
}

// IssueCounts are the sizes of the collections of an issue.
type IssueCounts struct {
	Comments    int
	Attachments int
	Children    int
	Links       int
	Watchers    int
	Votes       int
}

// Breadcrumb is the navigation context of an issue: its project and parent.
type Breadcrumb struct {
	ProjectID   uuid.UUID
	ProjectKey  string
	ProjectName string
	Parent      *ParentRef
}

// ParentRef is the display data of a parent issue.
type ParentRef struct {
	ID      uuid.UUID
	Key     string
	Summary string
	Type    string
}

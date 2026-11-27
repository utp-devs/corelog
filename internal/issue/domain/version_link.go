package domain

import (
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Kinds of issue to version links.
const (
	// VersionLinkFix marks the versions an issue is fixed in.
	VersionLinkFix = "fix"
	// VersionLinkAffects marks the versions an issue affects.
	VersionLinkAffects = "affects"
)

// Bounds of the version and component lists of one issue.
const (
	MaxIssueVersionLinks = 50
	MaxIssueComponents   = 20
)

// Status values of a version as the issue module reads them. They mirror the release module statuses.
const (
	versionStatusReleased = "released"
	versionStatusArchived = "archived"
)

// Typed errors of the version and component links. The codes are part of the API contract.
var (
	ErrIssueVersionInvalid = apperror.WithCode(apperror.KindValidation, "issue.version_invalid",
		"Alguna versión no existe en el proyecto de la incidencia o la lista es demasiado larga")
	ErrIssueVersionClosed = apperror.WithCode(apperror.KindConflict, "issue.version_closed",
		"La versión está publicada o archivada y no admite esta vinculación")
	ErrIssueComponentInvalid = apperror.WithCode(apperror.KindValidation, "issue.component_invalid",
		"Algún componente no existe en el proyecto de la incidencia o la lista es demasiado larga")
)

// LinkedRef is the display data of a version or component linked to an issue.
type LinkedRef struct {
	ID   uuid.UUID
	Name string
}

// IssueLinks are the links of one issue, loaded in batch for lists and views.
type IssueLinks struct {
	FixVersions     []LinkedRef
	AffectsVersions []LinkedRef
	Components      []LinkedRef
}

// VersionRef is a version as the issue module resolves it: its name and release status.
type VersionRef struct {
	ID     uuid.UUID
	Name   string
	Status string
}

// ComponentDefault is what a component contributes to the default assignee of a new issue.
type ComponentDefault struct {
	ID     uuid.UUID
	Name   string
	Policy string
	LeadID *uuid.UUID
}

// Component assignee policies, as stored in components.default_assignee_policy.
const (
	ComponentPolicyNone          = "none"
	ComponentPolicyComponentLead = "component_lead"
	ComponentPolicyProjectLead   = "project_lead"
)

// DedupeIDs drops repeated ids keeping the first occurrence and rejects lists longer than limit.
func DedupeIDs(ids []uuid.UUID, limit int) ([]uuid.UUID, bool) {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, len(out) <= limit
}

// CheckFixAccepted tells whether a fix link to a version with this status may be added. Released and archived
// versions only take fix issues when the caller overrides the rule.
func CheckFixAccepted(status string, override bool) error {
	if (status == versionStatusReleased || status == versionStatusArchived) && !override {
		return ErrIssueVersionClosed
	}
	return nil
}

// CheckAffectsAccepted tells whether an affects link to a version with this status may be added. Archived versions
// refuse affects links unless the caller overrides the rule; released versions accept them.
func CheckAffectsAccepted(status string, override bool) error {
	if status == versionStatusArchived && !override {
		return ErrIssueVersionClosed
	}
	return nil
}

// NamesText renders the names of linked items as the stored change value: sorted, comma separated.
func NamesText(refs []LinkedRef) string {
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

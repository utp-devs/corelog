package postgres

import (
	"errors"

	"github.com/jhonsferg/corelog/internal/search/application/port"
)

// ErrVersionsUnavailable is returned by a VersionResolver when version data cannot be queried (the tables do
// not exist yet or the feature is off).
var ErrVersionsUnavailable = errors.New("search: versions are not available")

// SQLFragment is a boolean SQL expression over the aliases i (issues) and p (projects). Its '?' placeholders
// are text parameters, bound in order from Args.
type SQLFragment struct {
	SQL  string
	Args []string
}

// VersionResolver is the narrow optional port through which the version fields (fixVersion and
// affectedVersion) reach version data owned by another module.
type VersionResolver interface {
	Condition(kind port.VersionKind, spec port.VersionSpec) (SQLFragment, error)
}

// VersionTables resolves version conditions over the versions and issue_versions tables.
type VersionTables struct{}

var _ VersionResolver = VersionTables{}

// Condition builds the fragment for one value: a version name, releasedVersions(), unreleasedVersions() or
// "has no version".
func (VersionTables) Condition(kind port.VersionKind, spec port.VersionSpec) (SQLFragment, error) {
	k := "fix"
	if kind == port.VersionAffects {
		k = "affects"
	}
	base := "EXISTS (SELECT 1 FROM issue_versions iv JOIN versions v ON v.id = iv.version_id WHERE iv.issue_id = i.id AND iv.kind = '" + k + "'"
	switch {
	case spec.Empty:
		return SQLFragment{SQL: "NOT EXISTS (SELECT 1 FROM issue_versions iv WHERE iv.issue_id = i.id AND iv.kind = '" + k + "')"}, nil
	case spec.Function == "releasedversions":
		return SQLFragment{SQL: base + " AND v.status = 'released')"}, nil
	case spec.Function == "unreleasedversions":
		return SQLFragment{SQL: base + " AND v.status = 'unreleased')"}, nil
	case spec.Function != "":
		return SQLFragment{}, errors.New("función no válida para versiones")
	}
	return SQLFragment{SQL: base + " AND (lower(v.name) = lower(?) OR v.id::text = lower(?)))", Args: []string{spec.Name, spec.Name}}, nil
}

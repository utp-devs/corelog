package postgres

import (
	"testing"

	"github.com/jmoiron/sqlx"
)

// fixtureOrgID is the organization that owns the projects of the hand-built test schemas.
const fixtureOrgID = "00000000-0000-4000-8000-0000000000a1"

// addCatalogFixtures extends a hand-built schema (projects and issues already exist) with the catalog tables
// and the catalog columns of issues, and seeds one default item of each catalog for fixtureOrgID.
func addCatalogFixtures(t *testing.T, db *sqlx.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE issue_types (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(), organization_id UUID NOT NULL, name TEXT NOT NULL,
			icon TEXT NOT NULL DEFAULT 'task', color TEXT NOT NULL DEFAULT 'blue',
			hierarchy_level SMALLINT NOT NULL DEFAULT 0, position INT NOT NULL DEFAULT 0,
			is_default BOOLEAN NOT NULL DEFAULT FALSE, legacy_key TEXT)`,
		`CREATE TABLE priorities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(), organization_id UUID NOT NULL, name TEXT NOT NULL,
			icon TEXT NOT NULL DEFAULT 'minus', color TEXT NOT NULL DEFAULT 'gray',
			position INT NOT NULL DEFAULT 0, is_default BOOLEAN NOT NULL DEFAULT FALSE, legacy_key TEXT)`,
		`CREATE TABLE resolutions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(), organization_id UUID NOT NULL, name TEXT NOT NULL,
			position INT NOT NULL DEFAULT 0, is_default BOOLEAN NOT NULL DEFAULT FALSE)`,
		`ALTER TABLE projects ADD COLUMN organization_id UUID NOT NULL DEFAULT '` + fixtureOrgID + `'`,
		`ALTER TABLE issues ADD COLUMN issue_type_id UUID, ADD COLUMN priority_id UUID,
			ADD COLUMN resolution_id UUID, ADD COLUMN resolved_at TIMESTAMPTZ`,
		`INSERT INTO issue_types (organization_id, name, is_default, legacy_key)
			VALUES ('` + fixtureOrgID + `', 'Tarea', TRUE, 'task')`,
		`INSERT INTO priorities (organization_id, name, is_default, legacy_key)
			VALUES ('` + fixtureOrgID + `', 'Media', TRUE, 'medium')`,
		`INSERT INTO resolutions (organization_id, name, is_default)
			VALUES ('` + fixtureOrgID + `', 'Hecho', TRUE)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("catalog fixture: %v\n%s", err, stmt)
		}
	}
}

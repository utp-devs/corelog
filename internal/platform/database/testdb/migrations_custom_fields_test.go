package testdb_test

import "testing"

// TestMigrationsCustomFieldsRoundTrip applies the custom field migration over legacy issues, checks that existing
// issues read as having no custom values, that the tables exist with their constraints, and that down and up
// cycle cleanly.
func TestMigrationsCustomFieldsRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 28)
	seedOrgMigLegacy(t, env.pool, true)
	env.runFile(t, 29, ".up.sql")
	env.runFile(t, 30, ".up.sql")
	env.runFile(t, 31, ".up.sql")

	env.runFile(t, 32, ".up.sql")
	assertCustomFieldSchema(t, env)

	env.runFile(t, 32, ".down.sql")
	for _, table := range []string{"custom_fields", "custom_field_contexts", "project_field_layouts"} {
		if got := countOrgMigWhere(t, env.pool, `information_schema.tables`,
			`table_name = '`+table+`' AND table_schema = current_schema()`); got != 0 {
			t.Fatalf("%s still exists after the down migration", table)
		}
	}
	if got := countOrgMigWhere(t, env.pool, `information_schema.columns`,
		`table_name = 'issues' AND column_name = 'custom' AND table_schema = current_schema()`); got != 0 {
		t.Fatalf("issues.custom still exists after the down migration")
	}

	env.runFile(t, 32, ".up.sql")
	assertCustomFieldSchema(t, env)
}

// assertCustomFieldSchema checks the tables, the default of issues.custom and the value constraints.
func assertCustomFieldSchema(t *testing.T, env *orgMigEnv) {
	t.Helper()
	assertOrgMigValue(t, env.pool, `SELECT custom::text FROM issues WHERE project_id = $1 LIMIT 1`, "{}",
		"legacy issues have empty custom values", orgMigProjectID)

	if _, err := env.pool.Exec(`
		INSERT INTO custom_fields (organization_id, name, field_type) VALUES ($1, 'Tipo raro', 'color')`, orgMigDefaultOrgID); err == nil {
		t.Fatal("unknown field_type must be rejected by the CHECK constraint")
	}
	var fieldID string
	if err := env.pool.QueryRow(`
		INSERT INTO custom_fields (organization_id, name, field_type) VALUES ($1, 'Enlace', 'url') RETURNING id::text`,
		orgMigDefaultOrgID).Scan(&fieldID); err != nil {
		t.Fatalf("insert field: %v", err)
	}
	if _, err := env.pool.Exec(`
		INSERT INTO custom_fields (organization_id, name, field_type) VALUES ($1, 'enlace', 'url')`, orgMigDefaultOrgID); err == nil {
		t.Fatal("names must be unique per organization ignoring case")
	}
	if _, err := env.pool.Exec(`
		INSERT INTO custom_field_contexts (field_id, project_id, required) VALUES ($1, $2, true)`, fieldID, orgMigProjectID); err != nil {
		t.Fatalf("insert context: %v", err)
	}
	if _, err := env.pool.Exec(`
		INSERT INTO custom_field_contexts (field_id, project_id, required) VALUES ($1, $2, false)`, fieldID, orgMigProjectID); err == nil {
		t.Fatal("the same field, project and type must appear once")
	}
}

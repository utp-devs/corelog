package testdb_test

import "testing"

const legalMigUserID = "00000000-0000-4000-8000-0000000000e6"

// TestMigrationsLegalConsentsRoundTrip checks the legal tables, their constraints and cascade, then runs the
// down and up migration again.
func TestMigrationsLegalConsentsRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 46)

	exec := func(query string, args ...any) error {
		_, err := env.pool.Exec(query, args...)
		return err
	}
	if err := exec(`INSERT INTO users (id, name, email, password_hash, role, display_name, timezone, created_at, updated_at)
		VALUES ($1, 'Legal', 'legal@example.test', 'hash', 'user', 'Legal', 'UTC', now(), now())`, legalMigUserID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := exec(`INSERT INTO legal_documents (slug, version, title, body_markdown, effective_from) VALUES ('privacy_policy', 1, 'Politica', 'texto', now())`); err != nil {
		t.Fatalf("insert document: %v", err)
	}
	if err := exec(`INSERT INTO legal_documents (slug, version, title, body_markdown, effective_from) VALUES ('privacy_policy', 1, 'Otra', 'texto', now())`); err == nil {
		t.Error("duplicate (slug, version) accepted")
	}
	if err := exec(`INSERT INTO legal_documents (slug, version, title, body_markdown, effective_from) VALUES ('unknown', 1, 'X', 'x', now())`); err == nil {
		t.Error("unknown slug accepted")
	}
	if err := exec(`INSERT INTO legal_documents (slug, version, title, body_markdown, required, effective_from) VALUES ('communications', 1, 'Avisos', 'x', true, now())`); err == nil {
		t.Error("required communications accepted")
	}

	consent := `INSERT INTO user_consents (user_id, document_slug, document_version, accepted_at, ip, user_agent, source) VALUES ($1, 'privacy_policy', 1, now(), '203.0.113.0', 'agent', $2)`
	if err := exec(consent, legalMigUserID, "register"); err != nil {
		t.Fatalf("insert consent: %v", err)
	}
	if err := exec(consent, legalMigUserID, "register"); err == nil {
		t.Error("two active consents for the same version accepted")
	}
	if err := exec(consent, legalMigUserID, "bogus"); err == nil {
		t.Error("unknown source accepted")
	}
	if err := exec(`UPDATE user_consents SET revoked_at = now() WHERE user_id = $1`, legalMigUserID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := exec(consent, legalMigUserID, "profile"); err != nil {
		t.Errorf("re-accept after revocation rejected: %v", err)
	}

	if err := exec(`DELETE FROM users WHERE id = $1`, legalMigUserID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM user_consents`, "0", "consents cascade on user delete")

	env.runFile(t, 46, ".down.sql")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN ('legal_documents', 'user_consents')`, "0", "legal tables removed by the down")

	env.runFile(t, 46, ".up.sql")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN ('legal_documents', 'user_consents')`, "2", "legal tables after the up again")
}

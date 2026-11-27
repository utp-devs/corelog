package testdb_test

import (
	"testing"
)

const twoFactorUserID = "00000000-0000-4000-8000-0000000000f9"

// TestMigrationsTwoFactorRoundTrip applies the schema up to the two-factor migration on legacy data, checks
// that the tables cascade when a user is removed, then runs the down and up migration again.
func TestMigrationsTwoFactorRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 28)
	seedOrgMigLegacy(t, env.pool, true)
	for version := 29; version <= 33; version++ {
		env.runFile(t, version, ".up.sql")
	}

	if _, err := env.pool.Exec(`
		INSERT INTO users (id, name, email, password_hash, role, display_name, timezone, created_at, updated_at)
		VALUES ($1, 'Two Factor', 'tf@example.test', 'hash', 'user', 'Two Factor', 'UTC', now(), now())`, twoFactorUserID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := env.pool.Exec(`
		INSERT INTO user_two_factor (user_id, secret_encrypted, enabled_at)
		VALUES ($1, '\x0102', now())`, twoFactorUserID); err != nil {
		t.Fatalf("insert user_two_factor: %v", err)
	}
	if _, err := env.pool.Exec(`
		INSERT INTO user_recovery_codes (user_id, code_hash)
		VALUES ($1, decode(repeat('ab', 32), 'hex'))`, twoFactorUserID); err != nil {
		t.Fatalf("insert user_recovery_codes: %v", err)
	}
	if _, err := env.pool.Exec(`
		INSERT INTO user_two_factor_challenges (user_id, expires_at)
		VALUES ($1, now() + interval '5 minutes')`, twoFactorUserID); err != nil {
		t.Fatalf("insert user_two_factor_challenges: %v", err)
	}
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM user_recovery_codes WHERE user_id = $1`,
		"1", "recovery code stored", twoFactorUserID)

	if _, err := env.pool.Exec(`DELETE FROM users WHERE id = $1`, twoFactorUserID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	for _, table := range []string{"user_two_factor", "user_recovery_codes", "user_two_factor_challenges"} {
		assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM `+table+` WHERE user_id = $1`,
			"0", table+" cascades on user delete", twoFactorUserID)
	}

	env.runFile(t, 33, ".down.sql")
	if got := countOrgMigWhere(t, env.pool, `information_schema.tables`,
		`table_name LIKE 'user_two_factor%' AND table_schema = current_schema()`); got != 0 {
		t.Fatalf("two-factor tables still exist after the down migration")
	}

	env.runFile(t, 33, ".up.sql")
	assertOrgMigValue(t, env.pool, `
		SELECT count(*)::text FROM information_schema.tables
		WHERE table_name LIKE 'user_two_factor%' AND table_schema = current_schema()`,
		"2", "two-factor tables after the up migration again")
}

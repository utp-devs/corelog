package testdb_test

import "testing"

// TestMigrationsUserNamePhoneRoundTrip applies the user identity migration over legacy users, checks the
// backfill of first and last names, the trigger that keeps name consistent, the phone constraints and the
// verified-only uniqueness, then runs the down and up migrations again.
func TestMigrationsUserNamePhoneRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 44)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := env.pool.Exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	scalar := func(q string, args ...any) string {
		t.Helper()
		var v string
		if err := env.pool.QueryRow(q, args...).Scan(&v); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
		return v
	}
	legacy := []struct{ email, name, first, last string }{
		{"single@example.test", "Madonna", "Madonna", ""},
		{"two@example.test", "Ana Perez", "Ana", "Perez"},
		{"three@example.test", "Ana Perez Lopez", "Ana", "Perez Lopez"},
		{"four@example.test", "Juan Carlos Perez Lopez", "Juan Carlos", "Perez Lopez"},
		{"spaces@example.test", "  Luis   Gomez  ", "Luis", "Gomez"},
	}
	for _, l := range legacy {
		exec(`INSERT INTO users (name, display_name, email, password_hash, role) VALUES ($1, $1, $2, 'x', 'user')`, l.name, l.email)
	}

	env.runFile(t, 45, ".up.sql")

	for _, l := range legacy {
		got := scalar(`SELECT first_name || '|' || last_name FROM users WHERE email = $1`, l.email)
		if want := l.first + "|" + l.last; got != want {
			t.Errorf("backfill %q = %q, want %q", l.name, got, want)
		}
	}
	if got := scalar(`SELECT name FROM users WHERE email = 'spaces@example.test'`); got != "  Luis   Gomez  " {
		t.Errorf("backfill must not touch name, got %q", got)
	}

	// Legacy writer: only name.
	exec(`INSERT INTO users (name, display_name, email, password_hash, role) VALUES ('Rosa Diaz', 'Rosa', 'rosa@example.test', 'x', 'user')`)
	if got := scalar(`SELECT first_name || '|' || last_name FROM users WHERE email = 'rosa@example.test'`); got != "Rosa|Diaz" {
		t.Errorf("insert with name only: %q", got)
	}
	// New writer: first and last names win and name follows.
	exec(`INSERT INTO users (name, first_name, last_name, display_name, email, password_hash, role) VALUES ('x', 'Maria Jose', 'Quispe', 'MJ', 'mj@example.test', 'x', 'user')`)
	if got := scalar(`SELECT name FROM users WHERE email = 'mj@example.test'`); got != "Maria Jose Quispe" {
		t.Errorf("name after insert with first/last = %q", got)
	}
	exec(`UPDATE users SET last_name = 'Rojas' WHERE email = 'mj@example.test'`)
	if got := scalar(`SELECT name FROM users WHERE email = 'mj@example.test'`); got != "Maria Jose Rojas" {
		t.Errorf("name after last_name update = %q", got)
	}
	exec(`UPDATE users SET name = 'Lucia Mar Soto' WHERE email = 'mj@example.test'`)
	if got := scalar(`SELECT first_name || '|' || last_name FROM users WHERE email = 'mj@example.test'`); got != "Lucia|Mar Soto" {
		t.Errorf("first/last after name update = %q", got)
	}
	exec(`UPDATE users SET email = 'mj2@example.test' WHERE email = 'mj@example.test'`)
	if got := scalar(`SELECT name FROM users WHERE email = 'mj2@example.test'`); got != "Lucia Mar Soto" {
		t.Errorf("unrelated update changed name: %q", got)
	}

	// Phone constraints.
	bad := []string{
		`UPDATE users SET phone_country = 'pe', phone_number = '912345678', phone_e164 = '+51912345678' WHERE email = 'rosa@example.test'`,
		`UPDATE users SET phone_country = 'PE', phone_number = '912345678', phone_e164 = '51912345678' WHERE email = 'rosa@example.test'`,
		`UPDATE users SET phone_country = 'PE', phone_number = '912345678', phone_e164 = '+0912345678' WHERE email = 'rosa@example.test'`,
		`UPDATE users SET phone_country = 'PE' WHERE email = 'rosa@example.test'`,
		`UPDATE users SET locale = 'ES' WHERE email = 'rosa@example.test'`,
	}
	for _, q := range bad {
		if _, err := env.pool.Exec(q); err == nil {
			t.Errorf("constraint accepted invalid data: %s", q)
		}
	}
	exec(`UPDATE users SET phone_country = 'PE', phone_number = '912345678', phone_e164 = '+51912345678' WHERE email = 'rosa@example.test'`)
	// Unverified numbers may repeat; verified numbers are unique.
	exec(`UPDATE users SET phone_country = 'PE', phone_number = '912345678', phone_e164 = '+51912345678' WHERE email = 'two@example.test'`)
	exec(`UPDATE users SET phone_verified_at = now() WHERE email = 'rosa@example.test'`)
	if _, err := env.pool.Exec(`UPDATE users SET phone_verified_at = now() WHERE email = 'two@example.test'`); err == nil {
		t.Error("a second verified owner of the same number was accepted")
	}
	exec(`INSERT INTO user_phone_verifications (user_id, phone_e164, code_hash, expires_at)
		SELECT id, '+51912345678', decode(repeat('ab', 32), 'hex'), now() + interval '10 minutes' FROM users WHERE email = 'rosa@example.test'`)

	env.runFile(t, 45, ".down.sql")
	if got := scalar(`SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'users' AND column_name IN ('first_name', 'last_name', 'phone_e164', 'locale')`); got != "0" {
		t.Fatalf("columns survive the down migration: %s", got)
	}
	if got := scalar(`SELECT name FROM users WHERE email = 'rosa@example.test'`); got != "Rosa Diaz" {
		t.Fatalf("name after down = %q", got)
	}
	env.runFile(t, 45, ".up.sql")
	if got := scalar(`SELECT first_name || '|' || last_name FROM users WHERE email = 'four@example.test'`); got != "Juan Carlos|Perez Lopez" {
		t.Fatalf("backfill after second up = %q", got)
	}
}

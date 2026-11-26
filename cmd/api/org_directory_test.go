package main

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

func TestOrgDirectoryIgnoresDeactivatedMembers(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	orgID, userID := uuid.New(), uuid.New()

	if _, err := db.ExecContext(ctx, `
		INSERT INTO users (id, name, display_name, email, password_hash, role)
		VALUES ($1, 'Ana', 'Ana', 'ana-dir@example.test', 'x', 'user')`, userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO organizations (id, slug, name) VALUES ($1, 'dir-test', 'Dir Test')`, orgID); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member')`, orgID, userID); err != nil {
		t.Fatalf("insert member: %v", err)
	}

	dir := newOrgDirectory(db)
	role, found, err := dir.MemberRole(ctx, orgID, userID)
	if err != nil || !found || role != "member" {
		t.Fatalf("active member: role=%q found=%v err=%v", role, found, err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE users SET deactivated_at = now() WHERE id = $1`, userID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, found, err = dir.MemberRole(ctx, orgID, userID); err != nil || found {
		t.Fatalf("deactivated member must not be found: found=%v err=%v", found, err)
	}
}

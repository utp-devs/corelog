package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// orgDirectory answers organization questions for the team and project modules
// without making them import the organization hexagon.
type orgDirectory struct {
	db *sqlx.DB
}

func newOrgDirectory(db *sqlx.DB) orgDirectory { return orgDirectory{db: db} }

// MemberRole returns the organization role of a user; found is false when the
// user is not a member (or the organization is deleted or unknown).
func (d orgDirectory) MemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, bool, error) {
	var role string
	err := d.db.GetContext(ctx, &role, `
		SELECT m.role
		FROM organization_members m
		JOIN organizations o ON o.id = m.organization_id AND o.deleted_at IS NULL
		WHERE m.organization_id = $1 AND m.user_id = $2`, orgID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("org directory: member role: %w", err)
	}
	return role, true, nil
}

// TeamCreationPolicy returns who may create teams: "members" or "admins".
func (d orgDirectory) TeamCreationPolicy(ctx context.Context, orgID uuid.UUID) (string, error) {
	return d.setting(ctx, orgID, "who_can_create_teams", "members")
}

// ProjectCreationPolicy returns who may create projects and the default visibility.
func (d orgDirectory) ProjectCreationPolicy(ctx context.Context, orgID uuid.UUID) (string, string, error) {
	policy, err := d.setting(ctx, orgID, "who_can_create_projects", "members")
	if err != nil {
		return "", "", err
	}
	visibility, err := d.setting(ctx, orgID, "default_project_visibility", "open")
	if err != nil {
		return "", "", err
	}
	return policy, visibility, nil
}

func (d orgDirectory) setting(ctx context.Context, orgID uuid.UUID, key, fallback string) (string, error) {
	var value sql.NullString
	err := d.db.GetContext(ctx, &value, `
		SELECT settings ->> $2 FROM organizations WHERE id = $1 AND deleted_at IS NULL`, orgID, key)
	if errors.Is(err, sql.ErrNoRows) {
		return fallback, nil
	}
	if err != nil {
		return "", fmt.Errorf("org directory: setting %s: %w", key, err)
	}
	if !value.Valid || value.String == "" {
		return fallback, nil
	}
	return value.String, nil
}

package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jmoiron/sqlx"
)

// openAccessSchema creates an isolated schema with the tables read by accessQuery and returns the DB.
func openAccessSchema(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("CORELOG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CORELOG_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	schema := "role_lookup_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE organizations (
			id UUID PRIMARY KEY,
			deleted_at TIMESTAMPTZ
		)`,
		`CREATE TABLE organization_members (
			organization_id UUID NOT NULL,
			user_id UUID NOT NULL,
			role VARCHAR(20) NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'guest')),
			PRIMARY KEY (organization_id, user_id)
		)`,
		`CREATE TABLE projects (
			id UUID PRIMARY KEY,
			organization_id UUID NOT NULL,
			visibility VARCHAR(10) NOT NULL
		)`,
		`CREATE TABLE project_members (
			project_id UUID NOT NULL,
			user_id UUID NOT NULL,
			role VARCHAR(20) NOT NULL CHECK (role IN ('lead', 'contributor', 'viewer')),
			PRIMARY KEY (project_id, user_id)
		)`,
		`CREATE TABLE team_members (
			team_id UUID NOT NULL,
			user_id UUID NOT NULL,
			PRIMARY KEY (team_id, user_id)
		)`,
		`CREATE TABLE project_team_roles (
			project_id UUID NOT NULL,
			team_id UUID NOT NULL,
			role VARCHAR(20) NOT NULL CHECK (role IN ('lead', 'contributor', 'viewer')),
			PRIMARY KEY (project_id, team_id)
		)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// TestRoleLookupFindAccessPostgres covers the effective context read from PostgreSQL: org role,
// explicit project role, visibility (legacy team = open), deleted organizations and projects whose
// caller has no organization membership.
func TestRoleLookupFindAccessPostgres(t *testing.T) {
	db := openAccessSchema(t)
	ctx := context.Background()

	org, deletedOrg := uuid.New(), uuid.New()
	openProject, privateProject, legacyProject := uuid.New(), uuid.New(), uuid.New()
	deletedOrgProject, strayProject := uuid.New(), uuid.New()
	owner, member, guest, lead, stranger := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO organizations (id, deleted_at) VALUES ($1, NULL), ($2, now())`, org, deletedOrg)
	exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES
		($1, $2, 'owner'), ($1, $3, 'member'), ($1, $4, 'guest'), ($1, $5, 'member'), ($6, $7, 'member')`,
		org, owner, member, guest, lead, deletedOrg, member)
	teamProject := uuid.New()
	exec(`INSERT INTO projects (id, organization_id, visibility) VALUES
		($1, $2, 'open'), ($3, $2, 'private'), ($4, $2, 'team'), ($5, $6, 'open'), ($7, $2, 'open'), ($8, $2, 'private')`,
		openProject, org, privateProject, legacyProject, deletedOrgProject, deletedOrg, strayProject, teamProject)
	exec(`INSERT INTO project_members (project_id, user_id, role) VALUES
		($1, $2, 'lead'), ($3, $4, 'viewer'), ($5, $6, 'contributor'), ($7, $8, 'lead')`,
		openProject, lead, privateProject, guest, deletedOrgProject, member, strayProject, stranger)
	// teamA: member and stranger (stranger is not an organization member). teamB: member.
	teamA, teamB := uuid.New(), uuid.New()
	exec(`INSERT INTO team_members (team_id, user_id) VALUES ($1, $2), ($1, $3), ($4, $2)`, teamA, member, stranger, teamB)
	exec(`INSERT INTO project_team_roles (project_id, team_id, role) VALUES
		($1, $2, 'viewer'), ($1, $3, 'contributor'), ($4, $2, 'lead')`,
		privateProject, teamA, teamB, teamProject)

	lookup := NewRoleLookup(db)
	cases := []struct {
		name    string
		project uuid.UUID
		user    uuid.UUID
		want    domain.Access
	}{
		{"org owner without project role", privateProject, owner,
			domain.Access{OrgRole: domain.OrgRoleOwner, Visibility: domain.VisibilityPrivate}},
		{"org member with explicit lead on open project", openProject, lead,
			domain.Access{OrgRole: domain.OrgRoleMember, ProjectRole: roleOf(domain.ProjectRoleLead), Visibility: domain.VisibilityOpen}},
		{"org member on open project without role", openProject, member,
			domain.Access{OrgRole: domain.OrgRoleMember, Visibility: domain.VisibilityOpen}},
		{"guest with explicit viewer role", privateProject, guest,
			domain.Access{OrgRole: domain.OrgRoleGuest, ProjectRole: roleOf(domain.ProjectRoleViewer), Visibility: domain.VisibilityPrivate}},
		{"legacy team visibility reads as open", legacyProject, member,
			domain.Access{OrgRole: domain.OrgRoleMember, Visibility: domain.VisibilityOpen}},
		{"deleted organization ignores org role and project role", deletedOrgProject, member,
			domain.Access{OrgRole: domain.OrgRoleNone, Visibility: domain.VisibilityOpen}},
		{"project role without organization membership is ignored", strayProject, stranger,
			domain.Access{OrgRole: domain.OrgRoleNone, Visibility: domain.VisibilityOpen}},
		{"outsider on private project", privateProject, stranger,
			domain.Access{OrgRole: domain.OrgRoleNone, Visibility: domain.VisibilityPrivate}},
		{"best team grant wins over other team grants", privateProject, member,
			domain.Access{OrgRole: domain.OrgRoleMember, TeamRole: roleOf(domain.ProjectRoleContributor), Visibility: domain.VisibilityPrivate}},
		{"team lead grant on private project", teamProject, member,
			domain.Access{OrgRole: domain.OrgRoleMember, TeamRole: roleOf(domain.ProjectRoleLead), Visibility: domain.VisibilityPrivate}},
		{"team grant ignored for a user outside the organization", privateProject, stranger,
			domain.Access{OrgRole: domain.OrgRoleNone, Visibility: domain.VisibilityPrivate}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lookup.FindAccess(ctx, tc.project, tc.user)
			if err != nil {
				t.Fatalf("FindAccess: %v", err)
			}
			if got.OrgRole != tc.want.OrgRole || got.Visibility != tc.want.Visibility ||
				!sameRole(got.ProjectRole, tc.want.ProjectRole) || !sameRole(got.TeamRole, tc.want.TeamRole) {
				t.Errorf("FindAccess = %+v, want %+v", got, tc.want)
			}
		})
	}

	got, err := lookup.FindAccess(ctx, uuid.New(), owner)
	if err != nil || got.ProjectRole != nil || got.OrgRole != "" {
		t.Errorf("FindAccess(missing project) = %+v, %v; want empty access and no error", got, err)
	}

	role, err := lookup.FindRole(ctx, openProject, lead)
	if err != nil || role == nil || *role != domain.ProjectRoleLead {
		t.Errorf("FindRole(lead) = %v, %v; want lead", role, err)
	}
	role, err = lookup.FindRole(ctx, openProject, member)
	if err != nil || role != nil {
		t.Errorf("FindRole(member without project role) = %v, %v; want nil and no error", role, err)
	}
}

func roleOf(r domain.ProjectRole) *domain.ProjectRole {
	return &r
}

func sameRole(a, b *domain.ProjectRole) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

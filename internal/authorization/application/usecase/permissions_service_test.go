package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/authorization/application/port"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// memSchemes is an in-memory SchemeRepository.
type memSchemes struct {
	rows map[uuid.UUID]domain.Scheme
}

func newMemSchemes() *memSchemes { return &memSchemes{rows: map[uuid.UUID]domain.Scheme{}} }

func (m *memSchemes) Load(_ context.Context, projectID uuid.UUID) (*domain.Scheme, error) {
	s, ok := m.rows[projectID]
	if !ok {
		return nil, nil
	}
	return &s, nil
}

func (m *memSchemes) Replace(_ context.Context, projectID uuid.UUID, scheme domain.Scheme) error {
	m.rows[projectID] = scheme
	return nil
}

func (m *memSchemes) Reset(_ context.Context, projectID uuid.UUID) error {
	delete(m.rows, projectID)
	return nil
}

var _ port.SchemeRepository = (*memSchemes)(nil)

func roleOfValue(r domain.ProjectRole) *domain.ProjectRole { return &r }

// grantsWith returns the default grants with the holders of one permission replaced.
func grantsWith(p domain.Permission, holders ...domain.Holder) []domain.Grant {
	grants := domain.DefaultScheme().Grants()
	for i := range grants {
		if grants[i].Permission == p {
			grants[i].Holders = holders
		}
	}
	return grants
}

func newPermissionFixture(access domain.Access) (*PermissionService, *Authorizer, *memSchemes) {
	schemes := newMemSchemes()
	authorizer := NewAuthorizer(&fakeAccess{access: access}).WithSchemes(schemes)
	return NewPermissionService(authorizer, schemes), authorizer, schemes
}

// TestViewerGrantedCreateIssueFollowsTheScheme checks the phase gate behavior: a viewer granted
// create_issue can create, and loses it again after the scheme is reset.
func TestViewerGrantedCreateIssueFollowsTheScheme(t *testing.T) {
	ctx := context.Background()
	project, actor := uuid.New(), uuid.New()
	viewer := domain.Access{OrgRole: domain.OrgRoleMember, ProjectRole: roleOfValue(domain.ProjectRoleViewer), Visibility: domain.VisibilityOpen}
	lead := domain.Access{OrgRole: domain.OrgRoleMember, ProjectRole: roleOfValue(domain.ProjectRoleLead), Visibility: domain.VisibilityOpen}

	// Both callers read the same scheme store, as they do in production.
	schemes := newMemSchemes()
	viewerAuthorizer := NewAuthorizer(&fakeAccess{access: viewer}).WithSchemes(schemes)
	leadService := NewPermissionService(NewAuthorizer(&fakeAccess{access: lead}).WithSchemes(schemes), schemes)

	if err := viewerAuthorizer.Authorize(ctx, port.ActionCreateIssue, actor, "user", project); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer must not create issues by default, got %v", err)
	}
	viewerService := NewPermissionService(viewerAuthorizer, schemes)
	granted := grantsWith(domain.PermissionCreateIssue, domain.HolderLead, domain.HolderContributor, domain.HolderViewer)
	if _, err := viewerService.Replace(ctx, actor, "user", project, granted); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("a viewer without edit_project must get forbidden on replace, got %v", err)
	}

	if _, err := leadService.Replace(ctx, actor, "user", project, granted); err != nil {
		t.Fatalf("lead replace: %v", err)
	}
	if err := viewerAuthorizer.Authorize(ctx, port.ActionCreateIssue, actor, "user", project); err != nil {
		t.Fatalf("viewer granted create_issue must pass, got %v", err)
	}

	if _, err := leadService.Reset(ctx, actor, "user", project); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if err := viewerAuthorizer.Authorize(ctx, port.ActionCreateIssue, actor, "user", project); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("after reset the viewer must be denied again, got %v", err)
	}
}

// TestRevokedContributorCannotCreateIssue checks that removing a role from the scheme denies it.
func TestRevokedContributorCannotCreateIssue(t *testing.T) {
	ctx := context.Background()
	project, actor := uuid.New(), uuid.New()
	contributor := domain.Access{OrgRole: domain.OrgRoleMember, ProjectRole: roleOfValue(domain.ProjectRoleContributor), Visibility: domain.VisibilityOpen}
	lead := domain.Access{OrgRole: domain.OrgRoleMember, ProjectRole: roleOfValue(domain.ProjectRoleLead), Visibility: domain.VisibilityOpen}

	schemes := newMemSchemes()
	leadService := NewPermissionService(NewAuthorizer(&fakeAccess{access: lead}).WithSchemes(schemes), schemes)
	contributorAuthorizer := NewAuthorizer(&fakeAccess{access: contributor}).WithSchemes(schemes)

	if err := contributorAuthorizer.Authorize(ctx, port.ActionCreateIssue, actor, "user", project); err != nil {
		t.Fatalf("contributor must create issues by default, got %v", err)
	}
	revoked := grantsWith(domain.PermissionCreateIssue, domain.HolderLead)
	if _, err := leadService.Replace(ctx, actor, "user", project, revoked); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := contributorAuthorizer.Authorize(ctx, port.ActionCreateIssue, actor, "user", project); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("revoked contributor must be forbidden, got %v", err)
	}
}

func TestGetFlagsDefaultSchemeAndHidesPrivateProjects(t *testing.T) {
	ctx := context.Background()
	project, actor := uuid.New(), uuid.New()

	member := domain.Access{OrgRole: domain.OrgRoleMember, ProjectRole: roleOfValue(domain.ProjectRoleContributor), Visibility: domain.VisibilityPrivate}
	service, _, _ := newPermissionFixture(member)
	view, err := service.Get(ctx, actor, "user", project)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !view.IsDefault || len(view.Grants) != len(domain.Permissions) {
		t.Fatalf("default view: isDefault=%v grants=%d", view.IsDefault, len(view.Grants))
	}

	outsider := domain.Access{OrgRole: domain.OrgRoleNone, Visibility: domain.VisibilityPrivate}
	outsiderService, _, _ := newPermissionFixture(outsider)
	if _, err := outsiderService.Get(ctx, actor, "user", project); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("outsider on a private project must get not found, got %v", err)
	}
	if _, err := outsiderService.Effective(ctx, actor, "user", project); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("outsider effective permissions must be not found, got %v", err)
	}
}

func TestReplaceRejectsInvalidSchemeWith422Code(t *testing.T) {
	ctx := context.Background()
	project, actor := uuid.New(), uuid.New()
	lead := domain.Access{OrgRole: domain.OrgRoleMember, ProjectRole: roleOfValue(domain.ProjectRoleLead), Visibility: domain.VisibilityOpen}
	service, _, schemes := newPermissionFixture(lead)

	invalid := grantsWith(domain.PermissionManageMembers, domain.HolderContributor)
	_, err := service.Replace(ctx, actor, "user", project, invalid)
	if apperror.KindOf(err) != apperror.KindValidation || apperror.CodeOf(err) != domain.CodeInvalidScheme {
		t.Fatalf("expected validation with code %s, got kind=%v code=%q", domain.CodeInvalidScheme, apperror.KindOf(err), apperror.CodeOf(err))
	}
	if _, stored := schemes.rows[project]; stored {
		t.Fatal("an invalid scheme must not be stored")
	}
}

func TestEffectivePermissionsPerRole(t *testing.T) {
	ctx := context.Background()
	project, actor := uuid.New(), uuid.New()

	cases := []struct {
		name   string
		access domain.Access
		role   string
		has    []domain.Permission
		lacks  []domain.Permission
	}{
		{
			name:   "lead",
			access: domain.Access{OrgRole: domain.OrgRoleMember, ProjectRole: roleOfValue(domain.ProjectRoleLead), Visibility: domain.VisibilityOpen},
			role:   "lead",
			// The lead may delete the project under the default scheme.
			has: []domain.Permission{domain.PermissionEditProject, domain.PermissionManageMembers, domain.PermissionDeleteIssue, domain.PermissionDeleteProject},
		},
		{
			name:   "org owner is lead with delete",
			access: domain.Access{OrgRole: domain.OrgRoleOwner, Visibility: domain.VisibilityPrivate},
			role:   "lead",
			has:    []domain.Permission{domain.PermissionDeleteProject, domain.PermissionEditProject},
		},
		{
			name:   "contributor",
			access: domain.Access{OrgRole: domain.OrgRoleMember, ProjectRole: roleOfValue(domain.ProjectRoleContributor), Visibility: domain.VisibilityOpen},
			role:   "contributor",
			has:    []domain.Permission{domain.PermissionCreateIssue, domain.PermissionBrowseProject},
			lacks:  []domain.Permission{domain.PermissionManageMembers, domain.PermissionEditProject, domain.PermissionManageSprints},
		},
		{
			name:   "viewer",
			access: domain.Access{OrgRole: domain.OrgRoleMember, ProjectRole: roleOfValue(domain.ProjectRoleViewer), Visibility: domain.VisibilityOpen},
			role:   "viewer",
			has:    []domain.Permission{domain.PermissionBrowseProject, domain.PermissionWatchIssue},
			lacks:  []domain.Permission{domain.PermissionCreateIssue, domain.PermissionCommentIssue},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, _, _ := newPermissionFixture(tc.access)
			view, err := service.Effective(ctx, actor, "user", project)
			if err != nil {
				t.Fatalf("effective: %v", err)
			}
			if view.Role != tc.role {
				t.Fatalf("role = %q, want %q", view.Role, tc.role)
			}
			granted := map[domain.Permission]bool{}
			for _, p := range view.Permissions {
				granted[p] = true
			}
			for _, p := range tc.has {
				if !granted[p] {
					t.Errorf("missing %s", p)
				}
			}
			for _, p := range tc.lacks {
				if granted[p] {
					t.Errorf("unexpected %s", p)
				}
			}
		})
	}
}

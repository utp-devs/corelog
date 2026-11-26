package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/authorization/application/port"
	"github.com/jhonsferg/corelog/internal/authorization/authztest"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// fakeAccess implements the full AccessLookup port (and RoleLookup through it).
type fakeAccess struct {
	access domain.Access
	err    error
}

func (f *fakeAccess) FindRole(_ context.Context, _, _ uuid.UUID) (*domain.ProjectRole, error) {
	return f.access.ProjectRole, f.err
}

func (f *fakeAccess) FindAccess(_ context.Context, _, _ uuid.UUID) (domain.Access, error) {
	return f.access, f.err
}

// roleOnly implements only RoleLookup, like the lookups of the other modules' tests.
type roleOnly struct {
	role *domain.ProjectRole
}

func (r roleOnly) FindRole(_ context.Context, _, _ uuid.UUID) (*domain.ProjectRole, error) {
	return r.role, nil
}

// accessOf converts a test actor into the context of one project with the given visibility.
func accessOf(a authztest.Actor, visibility string) domain.Access {
	access := domain.Access{PlatformRole: a.PlatformRole, OrgRole: domain.OrgRole(a.OrgRole), Visibility: domain.Visibility(visibility)}
	if a.ProjectRole != "" {
		role := domain.ProjectRole(a.ProjectRole)
		access.ProjectRole = &role
	}
	return access
}

// TestAuthorizerDecisionsByActor runs the shared ProjectMatrix through the use case, for open and
// private projects, using the full context lookup.
func TestAuthorizerDecisionsByActor(t *testing.T) {
	ctx := context.Background()
	project := uuid.New()

	for _, visibility := range []string{authztest.VisibilityOpen, authztest.VisibilityPrivate} {
		t.Run(visibility, func(t *testing.T) {
			authztest.RunMatrix(t, authztest.StandardActors, authztest.ProjectPermissions, authztest.ProjectMatrix[visibility],
				func(a authztest.Actor, permission string) bool {
					authorizer := NewAuthorizer(&fakeAccess{access: accessOf(a, visibility)})
					err := authorizer.Authorize(ctx, port.Action(permission), uuid.New(), a.PlatformRole, project)
					return err == nil
				})
		})
	}
}

// TestAuthorizerDeniedIsForbidden checks that every denial is reported as forbidden, never as another kind.
func TestAuthorizerDeniedIsForbidden(t *testing.T) {
	ctx := context.Background()
	project := uuid.New()

	for _, visibility := range []string{authztest.VisibilityOpen, authztest.VisibilityPrivate} {
		for _, action := range port.Actions {
			for _, actor := range authztest.StandardActors {
				authorizer := NewAuthorizer(&fakeAccess{access: accessOf(actor, visibility)})
				err := authorizer.Authorize(ctx, action, uuid.New(), actor.PlatformRole, project)
				if err != nil && apperror.KindOf(err) != apperror.KindForbidden {
					t.Errorf("%s on %s for %s: expected forbidden, got %v", action, visibility, actor.Name, err)
				}
			}
		}
	}
}

func TestAuthorizerPlatformAdminIsReadOnlyWithoutMembership(t *testing.T) {
	ctx := context.Background()
	authorizer := NewAuthorizer(&fakeAccess{access: domain.Access{OrgRole: domain.OrgRoleNone, Visibility: domain.VisibilityPrivate}})
	if err := authorizer.Authorize(ctx, port.ActionViewProject, uuid.New(), "admin", uuid.New()); err != nil {
		t.Fatalf("platform admin should view any project, got %v", err)
	}
	if err := authorizer.Authorize(ctx, port.ActionEditProject, uuid.New(), "admin", uuid.New()); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("platform admin must not edit without an organization or project role, got %v", err)
	}
}

// TestAuthorizerFallbackUsesExplicitRole checks the RoleLookup-only path: explicit roles decide,
// there are no organization or visibility rules.
func TestAuthorizerFallbackUsesExplicitRole(t *testing.T) {
	ctx := context.Background()
	lead := domain.ProjectRoleLead

	withLead := NewAuthorizer(roleOnly{role: &lead})
	if err := withLead.Authorize(ctx, port.ActionEditProject, uuid.New(), "user", uuid.New()); err != nil {
		t.Fatalf("explicit lead should edit the project, got %v", err)
	}
	withoutRole := NewAuthorizer(roleOnly{})
	if err := withoutRole.Authorize(ctx, port.ActionViewProject, uuid.New(), "user", uuid.New()); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("no explicit role should be forbidden, got %v", err)
	}
}

func TestAuthorizerPropagatesLookupErrors(t *testing.T) {
	lookupErr := errors.New("database unavailable")
	authorizer := NewAuthorizer(&fakeAccess{err: lookupErr})
	err := authorizer.Authorize(context.Background(), port.ActionViewProject, uuid.New(), "user", uuid.New())
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected the lookup error to be returned, got %v", err)
	}
}

func TestAuthorizerUnknownActionIsForbidden(t *testing.T) {
	authorizer := NewAuthorizer(&fakeAccess{access: domain.Access{ProjectRole: roleValue(domain.ProjectRoleLead)}})
	err := authorizer.Authorize(context.Background(), port.Action("unknown"), uuid.New(), "user", uuid.New())
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("expected forbidden for an unknown action, got %v", err)
	}
}

func roleValue(r domain.ProjectRole) *domain.ProjectRole {
	return &r
}

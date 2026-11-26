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

type fakeRoles struct {
	role   *domain.ProjectRole
	err    error
	called bool
}

func (f *fakeRoles) FindRole(_ context.Context, _, _ uuid.UUID) (*domain.ProjectRole, error) {
	f.called = true
	return f.role, f.err
}

// actorProjectRole converts the actor's project role name into the domain value (nil = no membership).
func actorProjectRole(a authztest.Actor) *domain.ProjectRole {
	if a.ProjectRole == "" {
		return nil
	}
	r := domain.ProjectRole(a.ProjectRole)
	return &r
}

// permissionMatrix is the same permission table as the domain policy matrix, expressed in
// use-case actions. Actors not listed for an action are denied.
var permissionMatrix = map[port.Action][]string{
	port.ActionViewProject:    {"admin", "lead", "contributor", "viewer"},
	port.ActionEditProject:    {"admin", "lead"},
	port.ActionDeleteProject:  {"admin"},
	port.ActionManageMembers:  {"admin", "lead"},
	port.ActionCreateIssue:    {"admin", "lead", "contributor"},
	port.ActionEditIssue:      {"admin", "lead", "contributor"},
	port.ActionComment:        {"admin", "lead", "contributor"},
	port.ActionWatchIssue:     {"admin", "lead", "contributor", "viewer"},
	port.ActionMoveCards:      {"admin", "lead", "contributor"},
	port.ActionManageBoard:    {"admin", "lead"},
	port.ActionManageWorkflow: {"admin", "lead"},
	port.ActionManageSprints:  {"admin", "lead"},
}

func matrixPermissions() ([]string, map[string][]string) {
	permissions := make([]string, 0, len(port.Actions))
	allowed := make(map[string][]string, len(permissionMatrix))
	for _, action := range port.Actions {
		permissions = append(permissions, string(action))
		allowed[string(action)] = permissionMatrix[action]
	}
	return permissions, allowed
}

func TestAuthorizerDecisionsByRole(t *testing.T) {
	ctx := context.Background()
	project := uuid.New()

	permissions, allowed := matrixPermissions()
	authztest.RunMatrix(t, authztest.StandardActors, permissions, allowed, func(a authztest.Actor, permission string) bool {
		authorizer := NewAuthorizer(&fakeRoles{role: actorProjectRole(a)})
		err := authorizer.Authorize(ctx, port.Action(permission), uuid.New(), a.PlatformRole, project)
		return err == nil
	})
}

// TestAuthorizerDeniedIsForbidden checks that every denial of the matrix is reported as a
// forbidden error, never as another kind.
func TestAuthorizerDeniedIsForbidden(t *testing.T) {
	ctx := context.Background()
	project := uuid.New()

	for _, action := range port.Actions {
		for _, actor := range authztest.StandardActors {
			authorizer := NewAuthorizer(&fakeRoles{role: actorProjectRole(actor)})
			err := authorizer.Authorize(ctx, action, uuid.New(), actor.PlatformRole, project)
			if err != nil && apperror.KindOf(err) != apperror.KindForbidden {
				t.Errorf("%s for %s: expected forbidden, got %v", action, actor.Name, err)
			}
		}
	}
}

func TestAuthorizerAdminSkipsMembershipLookup(t *testing.T) {
	roles := &fakeRoles{err: errors.New("must not be consulted for admin")}
	authorizer := NewAuthorizer(roles)
	for _, action := range port.Actions {
		if err := authorizer.Authorize(context.Background(), action, uuid.New(), "admin", uuid.New()); err != nil {
			t.Errorf("admin should be allowed %s, got %v", action, err)
		}
	}
	if roles.called {
		t.Error("membership lookup was called for an admin")
	}
}

func TestAuthorizerPropagatesLookupErrors(t *testing.T) {
	lookupErr := errors.New("database unavailable")
	authorizer := NewAuthorizer(&fakeRoles{err: lookupErr})
	err := authorizer.Authorize(context.Background(), port.ActionViewProject, uuid.New(), "requester", uuid.New())
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected the lookup error to be returned, got %v", err)
	}
}

func TestAuthorizerUnknownActionIsForbidden(t *testing.T) {
	authorizer := NewAuthorizer(&fakeRoles{role: actorProjectRole(authztest.Actor{ProjectRole: "lead"})})
	err := authorizer.Authorize(context.Background(), port.Action("unknown"), uuid.New(), "requester", uuid.New())
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("expected forbidden for an unknown action, got %v", err)
	}
}

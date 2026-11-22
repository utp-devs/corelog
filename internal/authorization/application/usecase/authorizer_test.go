package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/authorization/application/port"
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

func roleOf(r domain.ProjectRole) *domain.ProjectRole {
	return &r
}

func TestAuthorizerDecisionsByRole(t *testing.T) {
	ctx := context.Background()
	actor, project := uuid.New(), uuid.New()

	cases := []struct {
		name   string
		action port.Action
		role   *domain.ProjectRole
		allow  bool
	}{
		{"lead manages members", port.ActionManageMembers, roleOf(domain.ProjectRoleLead), true},
		{"contributor cannot manage members", port.ActionManageMembers, roleOf(domain.ProjectRoleContributor), false},
		{"lead cannot delete the project", port.ActionDeleteProject, roleOf(domain.ProjectRoleLead), false},
		{"viewer can watch an issue", port.ActionWatchIssue, roleOf(domain.ProjectRoleViewer), true},
		{"viewer cannot create an issue", port.ActionCreateIssue, roleOf(domain.ProjectRoleViewer), false},
		{"contributor can move cards", port.ActionMoveCards, roleOf(domain.ProjectRoleContributor), true},
		{"viewer cannot move cards", port.ActionMoveCards, roleOf(domain.ProjectRoleViewer), false},
		{"contributor cannot manage the board", port.ActionManageBoard, roleOf(domain.ProjectRoleContributor), false},
		{"lead manages the board", port.ActionManageBoard, roleOf(domain.ProjectRoleLead), true},
		{"lead manages the workflow", port.ActionManageWorkflow, roleOf(domain.ProjectRoleLead), true},
		{"contributor cannot manage the workflow", port.ActionManageWorkflow, roleOf(domain.ProjectRoleContributor), false},
		{"non member cannot view the project", port.ActionViewProject, nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roles := &fakeRoles{role: tc.role}
			authorizer := NewAuthorizer(roles)
			err := authorizer.Authorize(ctx, tc.action, actor, "requester", project)
			if tc.allow && err != nil {
				t.Fatalf("expected allowed, got %v", err)
			}
			if !tc.allow {
				if apperror.KindOf(err) != apperror.KindForbidden {
					t.Fatalf("expected forbidden, got %v", err)
				}
			}
		})
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
	authorizer := NewAuthorizer(&fakeRoles{role: roleOf(domain.ProjectRoleLead)})
	err := authorizer.Authorize(context.Background(), port.Action("unknown"), uuid.New(), "requester", uuid.New())
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("expected forbidden for an unknown action, got %v", err)
	}
}

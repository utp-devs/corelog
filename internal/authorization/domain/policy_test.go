package domain

import "testing"

func role(role ProjectRole) *ProjectRole {
	return &role
}

func TestPolicyByProjectRole(t *testing.T) {
	policy := Policy{}

	if !policy.CanViewProject("requester", role(ProjectRoleViewer)) {
		t.Fatal("viewer should view project")
	}
	if policy.CanEditProject("requester", role(ProjectRoleContributor)) {
		t.Fatal("contributor should not edit project")
	}
	if !policy.CanEditIssue("requester", role(ProjectRoleContributor)) {
		t.Fatal("contributor should edit issues")
	}
	if policy.CanManageProjectMembers("requester", role(ProjectRoleContributor)) {
		t.Fatal("contributor should not manage project members")
	}
	if !policy.CanManageProjectMembers("requester", role(ProjectRoleLead)) {
		t.Fatal("lead should manage project members")
	}
}

func TestPolicyAdmin(t *testing.T) {
	policy := Policy{}
	if !policy.CanManageProjectMembers("admin", nil) {
		t.Fatal("admin should manage project members")
	}
	if !policy.CanEditProject("admin", nil) {
		t.Fatal("admin should edit projects")
	}
}

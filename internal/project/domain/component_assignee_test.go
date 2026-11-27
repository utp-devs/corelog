package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestComponentPolicyRules(t *testing.T) {
	lead := uuid.New()
	componentLead := AssigneeComponentLead
	projectLead := AssigneeProjectLead
	bogus := "random"
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		patch   ComponentPatch
		code    string
		want    string
		hasLead bool
	}{
		{"policy none by default", ComponentPatch{}, "", AssigneeNone, false},
		{"component lead needs a lead", ComponentPatch{Policy: &componentLead}, "component.lead_required", "", false},
		{"component lead with lead", ComponentPatch{LeadIDSet: true, LeadID: &lead, Policy: &componentLead}, "", AssigneeComponentLead, true},
		{"project lead needs no lead", ComponentPatch{Policy: &projectLead}, "", AssigneeProjectLead, false},
		{"unknown policy", ComponentPatch{Policy: &bogus}, "component.policy_invalid", "", false},
		{"lead cleared keeps none", ComponentPatch{LeadIDSet: true}, "", AssigneeNone, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewComponent(uuid.New(), "Pagos", "")
			if err != nil {
				t.Fatal(err)
			}
			err = c.Update(tc.patch, now)
			if apperror.CodeOf(err) != tc.code {
				t.Fatalf("code = %q, want %q (%v)", apperror.CodeOf(err), tc.code, err)
			}
			if tc.code != "" {
				return
			}
			if c.DefaultAssigneePolicy != tc.want || (c.LeadID != nil) != tc.hasLead {
				t.Fatalf("policy %q lead %v", c.DefaultAssigneePolicy, c.LeadID)
			}
		})
	}
}

func TestComponentDefaultAssignee(t *testing.T) {
	lead := uuid.New()
	c := &Component{LeadID: &lead, DefaultAssigneePolicy: AssigneeComponentLead}
	if got := c.DefaultAssignee(); got == nil || *got != lead {
		t.Fatalf("component lead not resolved: %v", got)
	}
	c.DefaultAssigneePolicy = AssigneeProjectLead
	if c.DefaultAssignee() != nil {
		t.Fatal("project_lead policy is resolved by the caller, not here")
	}
}

func TestComponentRenameValidation(t *testing.T) {
	c, _ := NewComponent(uuid.New(), "Pagos", "")
	empty := "   "
	if err := c.Update(ComponentPatch{Name: &empty}, time.Now()); apperror.CodeOf(err) != "project.catalog_item_invalid" {
		t.Fatalf("blank name accepted: %v", err)
	}
}

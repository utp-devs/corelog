package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Default assignee policies of a component, applied when an issue is created with the component and no assignee.
const (
	// AssigneeNone leaves the issue to the project policy.
	AssigneeNone = "none"
	// AssigneeComponentLead assigns the issue to the lead of the component.
	AssigneeComponentLead = "component_lead"
	// AssigneeProjectLead assigns the issue to the project lead.
	AssigneeProjectLead = "project_lead"
)

// ValidAssigneePolicy reports whether p is a known component assignee policy.
func ValidAssigneePolicy(p string) bool {
	return p == AssigneeNone || p == AssigneeComponentLead || p == AssigneeProjectLead
}

// Typed errors of the component catalog. The codes are part of the API contract.
var (
	ErrComponentNotFound  = apperror.WithCode(apperror.KindNotFound, "component.not_found", "Componente no encontrado")
	ErrComponentNameTaken = apperror.WithCode(apperror.KindConflict, "component.name_taken",
		"Ya existe un componente con ese nombre en el proyecto")
	ErrComponentLead = apperror.WithCode(apperror.KindValidation, "component.lead_invalid",
		"El responsable del componente debe ser miembro del proyecto")
	ErrComponentLeadRequired = apperror.WithCode(apperror.KindValidation, "component.lead_required",
		"La politica por defecto del componente necesita un responsable")
	ErrComponentPolicy = apperror.WithCode(apperror.KindValidation, "component.policy_invalid",
		"La politica de asignacion por defecto no es valida")
	ErrComponentMoveTarget = apperror.WithCode(apperror.KindValidation, "component.move_target_invalid",
		"El componente de destino debe ser de este proyecto y distinto del que se borra")
)

// ComponentInput is the body of a new component.
type ComponentInput struct {
	Name        string
	Description string
	LeadID      *uuid.UUID
	Policy      string
}

// ComponentPatch is a partial update. A nil pointer keeps the field. LeadIDSet tells a cleared lead (null) from an
// untouched one.
type ComponentPatch struct {
	Name        *string
	Description *string
	LeadIDSet   bool
	LeadID      *uuid.UUID
	Policy      *string
}

// Update applies the patch. The lead's membership of the project is checked by the use case, which owns the
// membership data.
func (c *Component) Update(p ComponentPatch, now time.Time) error {
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if name == "" || len([]rune(name)) > 80 {
			return ErrInvalidCatalogItem
		}
		c.Name = name
	}
	if p.Description != nil {
		c.Description = strings.TrimSpace(*p.Description)
	}
	if p.LeadIDSet {
		c.LeadID = p.LeadID
	}
	if p.Policy != nil {
		if !ValidAssigneePolicy(*p.Policy) {
			return ErrComponentPolicy
		}
		c.DefaultAssigneePolicy = *p.Policy
	}
	if c.DefaultAssigneePolicy == AssigneeComponentLead && c.LeadID == nil {
		return ErrComponentLeadRequired
	}
	c.UpdatedAt = now.UTC()
	return nil
}

// DefaultAssignee resolves the assignee a new issue gets from this component. It returns nil when the component
// has no opinion (policy none, or component_lead without a lead). A project_lead policy is resolved by the caller.
func (c *Component) DefaultAssignee() *uuid.UUID {
	if c.DefaultAssigneePolicy == AssigneeComponentLead {
		return c.LeadID
	}
	return nil
}

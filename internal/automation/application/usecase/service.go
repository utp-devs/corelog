// Package usecase implements rule management and the automation engine.
package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/automation/application/port"
	"github.com/jhonsferg/corelog/internal/automation/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// ActionManage is the permission needed to manage rules.
const ActionManage = "manage_automation"

// Deps are the collaborators of the service and the engine. Directory, Notifier and Poster may be nil: actions
// that need them then fail with a clear message.
type Deps struct {
	Repo      port.Repository
	Auth      port.ProjectAuthorizer
	Issues    port.IssueReader
	Actions   port.IssueActions
	Matcher   port.Matcher
	Directory port.Directory
	Notifier  port.Notifier
	Poster    port.Poster
	Publisher events.Publisher // optional audit events
	Clock     clock.Clock
}

// Service implements port.Service.
type Service struct{ Deps }

var _ port.Service = (*Service)(nil)

// NewService returns the service.
func NewService(d Deps) *Service { return &Service{Deps: d} }

func (s *Service) authorize(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID) error {
	return s.Auth.Authorize(ctx, ActionManage, actorID, role, projectID)
}

// ruleProject authorizes access to an existing rule through its project. Rules without project are hidden.
func (s *Service) ruleProject(ctx context.Context, actorID uuid.UUID, role string, id uuid.UUID) (domain.Rule, error) {
	r, err := s.Repo.Get(ctx, id)
	if err != nil {
		return domain.Rule{}, err
	}
	if r.ProjectID == nil {
		return domain.Rule{}, domain.ErrNotFound
	}
	if err := s.authorize(ctx, actorID, role, *r.ProjectID); err != nil {
		return domain.Rule{}, err
	}
	return r, nil
}

func (s *Service) audit(ctx context.Context, kind string, actorID uuid.UUID, r domain.Rule) error {
	if s.Publisher == nil {
		return nil
	}
	org, id := r.OrgID, r.ID
	return s.Publisher.Publish(ctx, events.Event{
		OrgID: &org, ProjectID: r.ProjectID, ActorID: &actorID, EntityID: &id, Type: kind, EntityType: "automation_rule",
		Payload: map[string]any{"name": r.Name}, CreatedAt: s.Clock.Now(),
	})
}

// checkQueries validates the CoreQL texts of a rule with the search port.
func (s *Service) checkQueries(ctx context.Context, r domain.Rule) error {
	if s.Matcher == nil {
		return nil
	}
	texts := map[string]string{}
	if r.Trigger.Type == domain.TriggerScheduled {
		texts["trigger.query"] = r.Trigger.Query
	}
	for i, c := range r.Conditions {
		if c.Type == domain.ConditionQuery {
			texts[fmt.Sprintf("conditions[%d].query", i)] = c.Query
		}
	}
	for field, q := range texts {
		if err := s.Matcher.Validate(ctx, q); err != nil {
			return apperror.WithCode(apperror.KindValidation, "automation.invalid_query", "Regla no valida: la consulta CoreQL no es correcta").
				WithDetails(apperror.FieldError{Field: field, Message: "consulta no valida"})
		}
	}
	return nil
}

// Create stores a new rule of a project.
func (s *Service) Create(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID, in port.RuleInput) (domain.Rule, error) {
	if err := s.authorize(ctx, actorID, role, projectID); err != nil {
		return domain.Rule{}, err
	}
	org, ok, err := s.Repo.ProjectOrg(ctx, projectID)
	if err != nil {
		return domain.Rule{}, err
	}
	if !ok {
		return domain.Rule{}, apperror.WithCode(apperror.KindNotFound, "project.not_found", "Proyecto no encontrado")
	}
	now := s.Clock.Now()
	r := domain.Rule{
		OrgID: org, ProjectID: &projectID, Name: strings.TrimSpace(in.Name), Enabled: true, Trigger: in.Trigger,
		Conditions: orEmpty(in.Conditions), Actions: in.Actions, CreatedBy: &actorID, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if in.Enabled != nil {
		r.Enabled = *in.Enabled
	}
	if err := r.Validate(); err != nil {
		return domain.Rule{}, err
	}
	if err := s.checkQueries(ctx, r); err != nil {
		return domain.Rule{}, err
	}
	if err := s.Repo.Create(ctx, &r); err != nil {
		return domain.Rule{}, err
	}
	_ = s.audit(ctx, "automation.rule_created", actorID, r)
	return r, nil
}

func orEmpty(c []domain.Condition) []domain.Condition {
	if c == nil {
		return []domain.Condition{}
	}
	return c
}

// List returns the rules of a project.
func (s *Service) List(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID) ([]domain.Rule, error) {
	if err := s.authorize(ctx, actorID, role, projectID); err != nil {
		return nil, err
	}
	return s.Repo.ListByProject(ctx, projectID)
}

// Get returns one rule.
func (s *Service) Get(ctx context.Context, actorID uuid.UUID, role string, id uuid.UUID) (domain.Rule, error) {
	return s.ruleProject(ctx, actorID, role, id)
}

// Patch changes a rule. When Version is given and stale the change is refused with a conflict.
func (s *Service) Patch(ctx context.Context, actorID uuid.UUID, role string, id uuid.UUID, in port.PatchInput) (domain.Rule, error) {
	r, err := s.ruleProject(ctx, actorID, role, id)
	if err != nil {
		return domain.Rule{}, err
	}
	if in.Version != nil && *in.Version != r.Version {
		return domain.Rule{}, domain.ErrVersionConflict
	}
	if in.Name != nil {
		r.Name = strings.TrimSpace(*in.Name)
	}
	if in.Enabled != nil {
		r.Enabled = *in.Enabled
	}
	if in.Trigger != nil {
		r.Trigger = *in.Trigger
	}
	if in.Conditions != nil {
		r.Conditions = orEmpty(*in.Conditions)
	}
	if in.Actions != nil {
		r.Actions = *in.Actions
	}
	if err := r.Validate(); err != nil {
		return domain.Rule{}, err
	}
	if err := s.checkQueries(ctx, r); err != nil {
		return domain.Rule{}, err
	}
	r.UpdatedAt = s.Clock.Now()
	if err := s.Repo.Update(ctx, &r); err != nil {
		return domain.Rule{}, err
	}
	_ = s.audit(ctx, "automation.rule_updated", actorID, r)
	return r, nil
}

// Delete removes a rule and its runs.
func (s *Service) Delete(ctx context.Context, actorID uuid.UUID, role string, id uuid.UUID) error {
	r, err := s.ruleProject(ctx, actorID, role, id)
	if err != nil {
		return err
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.audit(ctx, "automation.rule_deleted", actorID, r)
	return nil
}

// SetEnabled enables or disables a rule.
func (s *Service) SetEnabled(ctx context.Context, actorID uuid.UUID, role string, id uuid.UUID, enabled bool) (domain.Rule, error) {
	if _, err := s.ruleProject(ctx, actorID, role, id); err != nil {
		return domain.Rule{}, err
	}
	r, err := s.Repo.SetEnabled(ctx, id, enabled, s.Clock.Now())
	if err != nil {
		return domain.Rule{}, err
	}
	_ = s.audit(ctx, "automation.rule_updated", actorID, r)
	return r, nil
}

// Runs lists the latest runs of a rule, optionally filtered by status.
func (s *Service) Runs(ctx context.Context, actorID uuid.UUID, role string, id uuid.UUID, status string, limit int) ([]domain.Run, error) {
	if _, err := s.ruleProject(ctx, actorID, role, id); err != nil {
		return nil, err
	}
	switch status {
	case "", domain.RunSuccess, domain.RunSkipped, domain.RunFailed:
	default:
		return nil, apperror.WithCode(apperror.KindValidation, "automation.invalid_filter", "Estado de ejecucion no valido")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.Repo.ListRuns(ctx, id, status, limit)
}

// Validate is the dry run: it checks the definition and describes what the actions would do for a sample issue.
func (s *Service) Validate(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID, in port.RuleInput, sample *uuid.UUID) (port.Validation, error) {
	if err := s.authorize(ctx, actorID, role, projectID); err != nil {
		return port.Validation{}, err
	}
	r := domain.Rule{Name: in.Name, Trigger: in.Trigger, Conditions: orEmpty(in.Conditions), Actions: in.Actions}
	out := port.Validation{Errors: []string{}, Planned: []string{}}
	if err := r.Validate(); err != nil {
		out.Errors = append(out.Errors, messageOf(err))
		return out, nil
	}
	if err := s.checkQueries(ctx, r); err != nil {
		out.Errors = append(out.Errors, messageOf(err))
		return out, nil
	}
	out.Valid = true
	var snap domain.Snapshot
	if sample != nil && s.Issues != nil {
		got, err := s.Issues.Snapshot(ctx, *sample)
		if err != nil {
			return port.Validation{}, err
		}
		if err := s.Auth.Authorize(ctx, "browse_project", actorID, role, projectID); err != nil {
			return port.Validation{}, err
		}
		snap = got
	}
	facts := domain.SmartFacts(snap, "Automatización", in.Name, domain.Facts{})
	for _, a := range r.Actions {
		out.Planned = append(out.Planned, describe(a, facts))
	}
	return out, nil
}

func messageOf(err error) string {
	if details := apperror.DetailsOf(err); len(details) > 0 {
		return details[0].Field + ": " + details[0].Message
	}
	return err.Error()
}

// describe renders what an action would do, in Spanish.
func describe(a domain.Action, facts map[string]string) string {
	switch a.Type {
	case domain.ActionTransition:
		return "Transicionar la incidencia a " + a.Status
	case domain.ActionAssign:
		return "Asignar la incidencia (" + a.Who + ")"
	case domain.ActionSetField:
		return "Establecer " + a.Field + " = " + domain.Render(a.Value, facts)
	case domain.ActionAddComment:
		return "Comentar: " + domain.Render(a.Text, facts)
	case domain.ActionAddLabel:
		return "Añadir la etiqueta " + a.Label
	case domain.ActionRemoveLabel:
		return "Quitar la etiqueta " + a.Label
	case domain.ActionCreateSubtask:
		return "Crear la subtarea: " + domain.Render(a.Text, facts)
	case domain.ActionSendWebhook:
		return "Enviar un webhook a " + a.URL
	case domain.ActionSendNotification:
		return "Notificar (" + a.Who + "): " + domain.Render(a.Text, facts)
	}
	return a.Type
}

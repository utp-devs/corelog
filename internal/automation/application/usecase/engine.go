package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/automation/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// consumerName is the cursor name of the engine in the events dispatcher.
const consumerName = "automation"

// Engine evaluates rules for domain events and scheduled runs. Failures stay inside the run of one rule.
type Engine struct{ Deps }

// NewEngine returns the engine.
func NewEngine(d Deps) *Engine { return &Engine{Deps: d} }

// Consumer returns the events consumer.
func (e *Engine) Consumer() events.Consumer { return engineConsumer{e: e} }

type engineConsumer struct{ e *Engine }

func (engineConsumer) Name() string { return consumerName }

func (c engineConsumer) Handle(ctx context.Context, ev events.Event) error {
	return c.e.Handle(ctx, ev)
}

// factsOf extracts the trigger facts of an event.
func factsOf(ev events.Event) domain.Facts {
	f := domain.Facts{Type: ev.Type}
	if list, ok := ev.Payload["changes"].([]any); ok {
		for _, item := range list {
			if m, ok := item.(map[string]any); ok {
				if name, ok := m["field"].(string); ok {
					f.ChangedFields = append(f.ChangedFields, name)
				}
			}
		}
	}
	f.From, _ = ev.Payload["from_status"].(string)
	f.To, _ = ev.Payload["to_status"].(string)
	return f
}

// originOf reads the automation origin recorded in an event payload.
func originOf(ev events.Event) (uuid.UUID, int) {
	var rule uuid.UUID
	if raw, ok := ev.Payload[events.PayloadAutomationRuleID].(string); ok {
		rule, _ = uuid.Parse(raw)
	}
	depth := 0
	switch v := ev.Payload[events.PayloadAutomationDepth].(type) {
	case float64:
		depth = int(v)
	case int:
		depth = v
	case int64:
		depth = int(v)
	}
	return rule, depth
}

func issueOf(ev events.Event) (uuid.UUID, bool) {
	if ev.EntityType == "issue" && ev.EntityID != nil {
		return *ev.EntityID, true
	}
	if raw, ok := ev.Payload["issue_id"].(string); ok {
		if id, err := uuid.Parse(raw); err == nil {
			return id, true
		}
	}
	return uuid.Nil, false
}

// Handle runs the rules of the event's project that its trigger matches. Only infrastructure errors that make
// retrying useful (loading the rules) are returned; everything else is recorded in the run log.
func (e *Engine) Handle(ctx context.Context, ev events.Event) error {
	if _, ok := domain.TriggerFor(ev.Type); !ok || ev.ProjectID == nil {
		return nil
	}
	origin, depth := originOf(ev)
	if depth >= domain.MaxChainDepth {
		return nil
	}
	org, ok, err := e.Repo.ProjectOrg(ctx, *ev.ProjectID)
	if err != nil || !ok {
		return err
	}
	rules, err := e.Repo.ActiveFor(ctx, org, *ev.ProjectID)
	if err != nil {
		return err
	}
	facts := factsOf(ev)
	issueID, hasIssue := issueOf(ev)
	for _, rule := range rules {
		if !rule.Trigger.Matches(facts) {
			continue
		}
		var issue *uuid.UUID
		if hasIssue {
			issue = &issueID
		}
		eventID := ev.ID
		e.runRule(ctx, rule, runInput{
			projectID: *ev.ProjectID, issueID: issue, eventID: &eventID, actorID: ev.ActorID, facts: facts,
			origin: origin, depth: depth,
		})
	}
	return nil
}

type runInput struct {
	projectID uuid.UUID
	issueID   *uuid.UUID
	eventID   *int64
	actorID   *uuid.UUID
	facts     domain.Facts
	origin    uuid.UUID
	depth     int
}

// runRule evaluates loop protection, the rate limit and the conditions, runs the actions and records the run.
// Panics and errors are contained so other rules still run.
func (e *Engine) runRule(ctx context.Context, rule domain.Rule, in runInput) {
	run := domain.Run{RuleID: rule.ID, EventID: in.eventID, IssueID: in.issueID, CreatedAt: e.Clock.Now()}
	defer func() {
		if p := recover(); p != nil {
			run.Status, run.Detail = domain.RunFailed, fmt.Sprintf("error interno: %v", p)
			e.record(ctx, &run)
		}
	}()
	if skip, reason := domain.Loop(rule.ID, in.origin, in.depth); skip {
		run.Status, run.Detail = domain.RunSkipped, reason
		e.record(ctx, &run)
		return
	}
	since := e.Clock.Now().Add(-time.Hour)
	if n, err := e.Repo.CountRunsSince(ctx, rule.ID, since); err == nil && n >= domain.MaxRunsPerHour {
		run.Status, run.Detail = domain.RunSkipped, "limite de 100 ejecuciones por hora alcanzado"
		e.record(ctx, &run)
		return
	}
	var snap domain.Snapshot
	if in.issueID != nil && e.Issues != nil {
		got, err := e.Issues.Snapshot(ctx, *in.issueID)
		if err != nil {
			run.Status, run.Detail = domain.RunFailed, "no se pudo leer la incidencia"
			e.record(ctx, &run)
			return
		}
		snap = got
	}
	passed, reason, err := e.conditions(ctx, rule, snap, in)
	if err != nil {
		run.Status, run.Detail = domain.RunFailed, "no se pudieron evaluar las condiciones: "+err.Error()
		e.record(ctx, &run)
		return
	}
	if !passed {
		run.Status, run.Detail = domain.RunSkipped, reason
		e.record(ctx, &run)
		return
	}
	actorName := ""
	if in.actorID != nil && e.Directory != nil {
		actorName, _ = e.Directory.UserName(ctx, *in.actorID)
	}
	runCtx := events.WithOrigin(ctx, events.Origin{RuleID: rule.ID, Name: rule.Name, Depth: in.depth + 1})
	done, err := e.execute(runCtx, rule, snap, domain.SmartFacts(snap, actorName, rule.Name, in.facts), in)
	if err != nil {
		run.Status, run.Detail = domain.RunFailed, fmt.Sprintf("accion %d: %s", done+1, safeMessage(err))
	} else {
		run.Status, run.Detail = domain.RunSuccess, fmt.Sprintf("%d acciones ejecutadas", done)
	}
	e.record(ctx, &run)
}

// safeMessage keeps the public message of an application error and hides anything else.
func safeMessage(err error) string {
	if apperror.KindOf(err) != apperror.KindInternal {
		return err.Error()
	}
	return "error interno"
}

func userErr(msg string) error { return apperror.New(apperror.KindValidation, msg) }

// webhookBody is the JSON body of a send_webhook action.
func webhookBody(rule domain.Rule, snap domain.Snapshot, f domain.Facts) []byte {
	body, _ := json.Marshal(map[string]any{
		"rule":  map[string]any{"id": rule.ID.String(), "name": rule.Name},
		"event": f.Type,
		"issue": map[string]any{"key": snap.Key, "summary": snap.Summary, "status": snap.Status},
	})
	return body
}

func (e *Engine) record(ctx context.Context, run *domain.Run) {
	if err := e.Repo.RecordRun(ctx, run); err != nil {
		slog.Error("automation run could not be recorded", "rule_id", run.RuleID.String(), "error", err)
	}
}

// conditions evaluates every condition. It reports the reason of the first one that fails.
func (e *Engine) conditions(ctx context.Context, rule domain.Rule, snap domain.Snapshot, in runInput) (bool, string, error) {
	for i, c := range rule.Conditions {
		switch c.Type {
		case domain.ConditionQuery:
			if in.issueID == nil || e.Matcher == nil {
				return false, fmt.Sprintf("condicion %d: no hay incidencia que evaluar", i+1), nil
			}
			ok, err := e.Matcher.Matches(ctx, in.projectID, *in.issueID, c.Query)
			if err != nil {
				return false, "", err
			}
			if !ok {
				return false, fmt.Sprintf("condicion %d no cumplida: la incidencia no cumple la consulta", i+1), nil
			}
		case domain.ConditionActorRole:
			if in.actorID == nil || e.Directory == nil {
				return false, fmt.Sprintf("condicion %d: evento sin actor", i+1), nil
			}
			role, err := e.Directory.ActorRole(ctx, in.projectID, *in.actorID)
			if err != nil {
				return false, "", err
			}
			if role != c.Role {
				return false, fmt.Sprintf("condicion %d no cumplida: el rol del actor no es %s", i+1, c.Role), nil
			}
		default:
			if !c.Holds(snap) {
				return false, fmt.Sprintf("condicion %d no cumplida", i+1), nil
			}
		}
	}
	return true, "", nil
}

// execute runs the actions in order and stops at the first failure. It returns how many succeeded.
func (e *Engine) execute(ctx context.Context, rule domain.Rule, snap domain.Snapshot, facts map[string]string, in runInput) (int, error) {
	for i, a := range rule.Actions {
		if err := e.action(ctx, rule, a, snap, facts, in); err != nil {
			return i, err
		}
	}
	return len(rule.Actions), nil
}

var errNoIssue = userErr("la accion necesita una incidencia")

func (e *Engine) action(ctx context.Context, rule domain.Rule, a domain.Action, snap domain.Snapshot, facts map[string]string, in runInput) error {
	if a.Type != domain.ActionSendWebhook && in.issueID == nil && a.Type != domain.ActionSendNotification {
		return errNoIssue
	}
	switch a.Type {
	case domain.ActionTransition:
		return e.Actions.Transition(ctx, *in.issueID, a.Status)
	case domain.ActionAssign:
		who, err := e.resolveUser(ctx, a, snap, in.projectID, rule.RunCount)
		if err != nil {
			return err
		}
		return e.Actions.Assign(ctx, *in.issueID, who)
	case domain.ActionSetField:
		return e.Actions.SetField(ctx, *in.issueID, a.Field, domain.Render(a.Value, facts))
	case domain.ActionAddComment:
		return e.Actions.AddComment(ctx, *in.issueID, "Automatización: "+html.EscapeString(rule.Name)+"\n\n"+domain.Render(a.Text, facts))
	case domain.ActionAddLabel:
		return e.Actions.AddLabel(ctx, *in.issueID, a.Label)
	case domain.ActionRemoveLabel:
		return e.Actions.RemoveLabel(ctx, *in.issueID, a.Label)
	case domain.ActionCreateSubtask:
		_, err := e.Actions.CreateSubtask(ctx, *in.issueID, domain.Render(a.Text, facts))
		return err
	case domain.ActionSendWebhook:
		if e.Poster == nil {
			return userErr("los webhooks salientes no estan disponibles")
		}
		return e.Poster.Post(ctx, a.URL, webhookBody(rule, snap, in.facts))
	case domain.ActionSendNotification:
		if e.Notifier == nil {
			return userErr("las notificaciones no estan disponibles")
		}
		who, err := e.resolveUser(ctx, a, snap, in.projectID, rule.RunCount)
		if err != nil {
			return err
		}
		link := ""
		if snap.Key != "" {
			link = "/issues/" + snap.Key
		}
		return e.Notifier.Notify(ctx, who, "automation", domain.Render(a.Text, facts), link)
	}
	return fmt.Errorf("accion desconocida: %s", a.Type)
}

// resolveUser picks the user an assignment or notification targets.
func (e *Engine) resolveUser(ctx context.Context, a domain.Action, snap domain.Snapshot, projectID uuid.UUID, runCount int64) (uuid.UUID, error) {
	switch a.Who {
	case domain.WhoUser:
		return uuid.Parse(a.UserID)
	case domain.WhoReporter:
		if snap.ReporterID == uuid.Nil {
			return uuid.Nil, userErr("la incidencia no tiene informador")
		}
		return snap.ReporterID, nil
	case domain.WhoAssignee:
		if snap.AssigneeID == nil {
			return uuid.Nil, userErr("la incidencia no tiene responsable")
		}
		return *snap.AssigneeID, nil
	case domain.WhoLead:
		if e.Directory == nil {
			return uuid.Nil, userErr("directorio no disponible")
		}
		return e.Directory.ProjectLead(ctx, projectID)
	case domain.WhoRoundRobin:
		if e.Directory == nil {
			return uuid.Nil, userErr("directorio no disponible")
		}
		team, err := uuid.Parse(a.TeamID)
		if err != nil {
			return uuid.Nil, err
		}
		members, err := e.Directory.TeamMembers(ctx, team)
		if err != nil {
			return uuid.Nil, err
		}
		if len(members) == 0 {
			return uuid.Nil, userErr("el equipo no tiene miembros")
		}
		return members[int(runCount%int64(len(members)))], nil
	}
	return uuid.Nil, userErr("destinatario desconocido")
}

// RunScheduled runs the scheduled rules that are due at now: for each, the issues its CoreQL query matches (up to
// MaxScheduledIssues) go through the usual conditions and actions. The scheduler of the application calls it
// periodically (for example every minute); a rule that ran less than its interval ago is not due.
func (e *Engine) RunScheduled(ctx context.Context, now time.Time) error {
	rules, err := e.Repo.ActiveScheduled(ctx)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if rule.ProjectID == nil || e.Matcher == nil {
			continue
		}
		every := time.Duration(rule.Trigger.EveryMinutes) * time.Minute
		if rule.LastRunAt != nil && now.Before(rule.LastRunAt.Add(every)) {
			continue
		}
		if err := e.Repo.TouchScheduled(ctx, rule.ID, now); err != nil {
			slog.Error("automation schedule could not be recorded", "rule_id", rule.ID.String(), "error", err)
			continue
		}
		ids, err := e.Matcher.Find(ctx, *rule.ProjectID, rule.Trigger.Query, domain.MaxScheduledIssues)
		if err != nil {
			run := domain.Run{RuleID: rule.ID, Status: domain.RunFailed, Detail: "la consulta programada fallo", CreatedAt: now}
			e.record(ctx, &run)
			continue
		}
		for _, id := range ids {
			issue := id
			e.runRule(ctx, rule, runInput{projectID: *rule.ProjectID, issueID: &issue, facts: domain.Facts{Type: "scheduled"}})
		}
	}
	return nil
}

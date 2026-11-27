package main

import (
	"context"

	"github.com/google/uuid"

	boardport "github.com/jhonsferg/corelog/internal/board/application/port"
	issueport "github.com/jhonsferg/corelog/internal/issue/application/port"
	workflowusecase "github.com/jhonsferg/corelog/internal/workflow/application/usecase"
	workflowdomain "github.com/jhonsferg/corelog/internal/workflow/domain"
)

// workflowEngineAdapter adapts the workflow service to the engine port of the issue module. It lives in the
// composition root, the only place that knows both hexagons.
type workflowEngineAdapter struct {
	workflows *workflowusecase.Service
}

var _ issueport.WorkflowEngine = workflowEngineAdapter{}

func idText(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

func idValue(text string) uuid.UUID {
	id, err := uuid.Parse(text)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func engineActor(a issueport.TransitionActor) workflowdomain.ActorContext {
	return workflowdomain.ActorContext{UserID: idText(a.UserID), Role: a.Role, LeadID: idText(a.LeadID), Permissions: a.Permissions}
}

func engineIssue(i issueport.TransitionIssue) workflowdomain.IssueSnapshot {
	return workflowdomain.IssueSnapshot{
		StatusID: idText(i.StatusID), AssigneeID: idText(i.AssigneeID), ReporterID: idText(i.ReporterID),
		ResolutionID: idText(i.ResolutionID), OpenSubtasks: i.OpenSubtasks, Fields: i.Fields,
	}
}

func (a workflowEngineAdapter) Available(ctx context.Context, projectID uuid.UUID, issue issueport.TransitionIssue, actor issueport.TransitionActor) ([]issueport.TransitionOption, error) {
	available, err := a.workflows.Available(ctx, projectID, engineIssue(issue), engineActor(actor))
	if err != nil {
		return nil, err
	}
	out := make([]issueport.TransitionOption, 0, len(available))
	for _, t := range available {
		out = append(out, issueport.TransitionOption{
			ID: t.TransitionID, Name: t.Name, ToStatusID: idValue(t.ToStatusID), ToStatusName: t.ToStatusName,
			ToCategory: string(t.ToCategory), RequiredFields: t.RequiredFields, RequiresComment: t.RequiresComment,
			RequiresResolution: t.RequiresResolution, RequiresAssignee: t.RequiresAssignee,
		})
	}
	return out, nil
}

func (a workflowEngineAdapter) Plan(ctx context.Context, projectID uuid.UUID, issue issueport.TransitionIssue, actor issueport.TransitionActor, transitionID string, input issueport.TransitionInput) (issueport.TransitionPlan, error) {
	result, name, err := a.workflows.Plan(ctx, projectID, engineIssue(issue), engineActor(actor), transitionID, workflowdomain.TransitionInput{
		Fields: input.Fields, Comment: input.Comment, ResolutionID: idText(input.ResolutionID), AssigneeID: idText(input.AssigneeID),
	})
	if err != nil {
		return issueport.TransitionPlan{}, err
	}
	target, err := a.workflows.StatusOf(ctx, projectID, result.ToStatusID)
	if err != nil {
		return issueport.TransitionPlan{}, err
	}
	plan := issueport.TransitionPlan{
		TransitionID: result.TransitionID, TransitionName: name, FromStatusID: idValue(result.FromStatusID),
		ToStatusID: idValue(result.ToStatusID), ToStatusName: target.Name, ToCategory: string(target.Category),
		UseDefaultDone: result.Resolution.UseDefaultDone, ResolutionID: idValue(result.Resolution.ID),
		AssigneeSet: result.Assignee.Set, AssigneeID: idValue(result.Assignee.UserID),
		Comment: result.Comment, Comments: result.Comments, Events: result.Events,
	}
	switch result.Resolution.Action {
	case workflowdomain.ResolutionSet:
		plan.ResolutionAction = issueport.PlanResolutionSet
	case workflowdomain.ResolutionClear:
		plan.ResolutionAction = issueport.PlanResolutionClear
	}
	for _, f := range result.FieldSets {
		plan.Fields = append(plan.Fields, issueport.PlannedField{Field: f.Field, Value: f.Value})
	}
	return plan, nil
}

func (a workflowEngineAdapter) ColumnStatusIDs(ctx context.Context, columnID uuid.UUID) ([]uuid.UUID, error) {
	return a.workflows.ColumnStatusIDs(ctx, columnID)
}

// boardIssueMover adapts the transition service of the issue module to the IssueMover port of the board: the
// target column is resolved to a transition of the card.
type boardIssueMover struct {
	issues issueport.TransitionService
}

var _ boardport.IssueMover = boardIssueMover{}

func (m boardIssueMover) MoveIssue(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueID, columnID uuid.UUID) error {
	return m.issues.MoveIssueToColumn(ctx, actorID, globalRole, projectID, issueID, columnID)
}

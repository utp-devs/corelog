package main

import (
	"context"

	"github.com/google/uuid"

	boardport "github.com/jhonsferg/corelog/internal/board/application/port"
	issueport "github.com/jhonsferg/corelog/internal/issue/application/port"
)

// boardCardTransitions adapts the transition service of the issue module to the transitions port of the board.
type boardCardTransitions struct {
	issues issueport.TransitionService
}

var _ boardport.CardTransitions = boardCardTransitions{}

func (a boardCardTransitions) Options(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID) ([]boardport.TransitionOption, error) {
	options, err := a.issues.ListTransitions(ctx, issueID, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	out := make([]boardport.TransitionOption, 0, len(options))
	for _, o := range options {
		out = append(out, boardport.TransitionOption{
			ID: o.ID, Name: o.Name, ToStatusID: o.ToStatusID, RequiredFields: o.RequiredFields,
			RequiresComment: o.RequiresComment, RequiresResolution: o.RequiresResolution, RequiresAssignee: o.RequiresAssignee,
		})
	}
	return out, nil
}

func (a boardCardTransitions) Run(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID, transitionID string, expectedStatusID uuid.UUID, input boardport.TransitionInput) error {
	_, err := a.issues.TransitionIssue(ctx, issueport.TransitionRequest{
		IssueID: issueID, ActorID: actorID, GlobalRole: globalRole, TransitionID: transitionID, ExpectedStatusID: expectedStatusID,
		Input: issueport.TransitionInput{Fields: input.Fields, Comment: input.Comment, ResolutionID: input.ResolutionID, AssigneeID: input.AssigneeID},
	})
	return err
}

// boardCardEditor applies the effects of a swimlane drop through the issue service, so activity and
// notifications are recorded like any other edit.
type boardCardEditor struct {
	issues issueport.Service
}

var _ boardport.CardEditor = boardCardEditor{}

func (e boardCardEditor) SetAssignee(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID, assigneeID *uuid.UUID) error {
	if assigneeID == nil {
		_, err := e.issues.Unassign(ctx, issueID, actorID, globalRole)
		return err
	}
	_, err := e.issues.Assign(ctx, issueID, actorID, *assigneeID, globalRole)
	return err
}

func (e boardCardEditor) SetEpic(ctx context.Context, actorID uuid.UUID, globalRole string, issueID, epicID uuid.UUID) error {
	_, err := e.issues.Patch(ctx, issueID, actorID, globalRole, issueport.PatchInput{ParentID: issueport.Some(epicID)})
	return err
}

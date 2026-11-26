package http

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type createSavedFilterRequest struct {
	Name     string                      `json:"name" validate:"required,max=80"`
	Criteria *domain.SavedFilterCriteria `json:"filters" validate:"required"`
}

type savedFilterResponse struct {
	ID        uuid.UUID                  `json:"id"`
	Name      string                     `json:"name"`
	Criteria  domain.SavedFilterCriteria `json:"filters"`
	CreatedAt time.Time                  `json:"created_at"`
	UpdatedAt time.Time                  `json:"updated_at"`
}

func toSavedFilterResponse(filter *domain.SavedFilter) savedFilterResponse {
	return savedFilterResponse{ID: filter.ID, Name: filter.Name, Criteria: filter.Criteria, CreatedAt: filter.CreatedAt, UpdatedAt: filter.UpdatedAt}
}

type createTicketRequest struct {
	Title       string     `json:"title" validate:"required,min=3,max=200"`
	Description string     `json:"description" validate:"required,min=3"`
	Priority    string     `json:"priority" validate:"required,oneof=low medium high critical"`
	ProjectID   string     `json:"project_id" validate:"omitempty,uuid"`
	IssueType   string     `json:"issue_type" validate:"omitempty,oneof=task bug story epic subtask"`
	ParentID    string     `json:"parent_id" validate:"omitempty,uuid4"`
	DueDate     *time.Time `json:"due_date"`
	StoryPoints *float64   `json:"story_points"`
}

type updateMetadataRequest struct {
	IssueType   string     `json:"issue_type" validate:"required,oneof=task bug story epic subtask"`
	ParentID    string     `json:"parent_id" validate:"omitempty,uuid4"`
	DueDate     *time.Time `json:"due_date"`
	StoryPoints *float64   `json:"story_points"`
}

type updateDetailsRequest struct {
	Title       string     `json:"title" validate:"required,min=3,max=200"`
	Description string     `json:"description" validate:"required,min=3"`
	Priority    string     `json:"priority" validate:"required,oneof=low medium high critical"`
	IssueType   string     `json:"issue_type" validate:"required,oneof=task bug story epic subtask"`
	ParentID    string     `json:"parent_id" validate:"omitempty,uuid4"`
	DueDate     *time.Time `json:"due_date"`
	StoryPoints *float64   `json:"story_points"`
}

type createCommentRequest struct {
	Body string `json:"body" validate:"required,min=1,max=5000"`
}

type updateCommentRequest struct {
	Body string `json:"body" validate:"required,min=1,max=5000"`
}

type updateRankRequest struct {
	Rank string `json:"rank" validate:"required,max=64"`
}

type commentResponse struct {
	ID        string     `json:"id"`
	IssueID   string     `json:"issue_id"`
	AuthorID  string     `json:"author_id"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type activityResponse struct {
	ID        string    `json:"id"`
	IssueID   string    `json:"issue_id"`
	ActorID   string    `json:"actor_id"`
	Action    string    `json:"action"`
	FieldName *string   `json:"field_name,omitempty"`
	OldValue  *string   `json:"old_value,omitempty"`
	NewValue  *string   `json:"new_value,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type createIssueLinkRequest struct {
	TargetIssueID string `json:"target_issue_id" validate:"required,uuid4"`
	Kind          string `json:"kind" validate:"required,oneof=blocks relates duplicates"`
}

type issueLinkResponse struct {
	ID            string    `json:"id"`
	SourceIssueID string    `json:"source_issue_id"`
	TargetIssueID string    `json:"target_issue_id"`
	Kind          string    `json:"kind"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

type watcherStateResponse struct {
	IsWatching bool `json:"is_watching"`
	Count      int  `json:"count"`
}

type updateStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=open in_progress resolved closed"`
}

// assignRequest acepta assignee_id como uuid o como null (desasignar). Se distingue la ausencia del campo
// del null: un campo omitido se rechaza.
type assignRequest struct {
	AssigneeID nullableUUID `json:"assignee_id"`
}

// nullableUUID recuerda si el campo llego en el JSON y guarda su valor textual, o nil si llego null.
type nullableUUID struct {
	Present bool
	Value   *string
}

func (n *nullableUUID) UnmarshalJSON(data []byte) error {
	n.Present = true
	if string(bytes.TrimSpace(data)) == "null" {
		n.Value = nil
		return nil
	}
	// Un valor que no es texto JSON llega tal cual y falla la validacion de uuid (validation.failed).
	value := string(data)
	if err := json.Unmarshal(data, &value); err != nil {
		value = string(data)
	}
	n.Value = &value
	return nil
}

// assignedUserValidation valida el uuid de una asignacion no nula.
type assignedUserValidation struct {
	AssigneeID string `json:"assignee_id" validate:"required,uuid4"`
}

type ticketResponse struct {
	ID            string     `json:"id"`
	ProjectID     string     `json:"project_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Status        string     `json:"status"`
	Priority      string     `json:"priority"`
	IssueType     string     `json:"issue_type"`
	ParentID      *string    `json:"parent_id,omitempty"`
	DueDate       *time.Time `json:"due_date,omitempty"`
	StoryPoints   *float64   `json:"story_points,omitempty"`
	Rank          string     `json:"rank"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
	RequesterID   string     `json:"requester_id"`
	RequesterName string     `json:"requester_name"`
	AssigneeID    *string    `json:"assignee_id,omitempty"`
	AssigneeName  *string    `json:"assignee_name,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// toTicketResponse arma la respuesta de una incidencia. names contiene el nombre de cada persona implicada.
func toTicketResponse(t *domain.Ticket, names map[uuid.UUID]string) ticketResponse {
	resp := ticketResponse{
		ID:            t.ID.String(),
		ProjectID:     t.ProjectID.String(),
		Title:         t.Title,
		Description:   t.Description,
		Status:        string(t.Status),
		Priority:      string(t.Priority),
		IssueType:     string(t.IssueType),
		DueDate:       t.DueDate,
		StoryPoints:   t.StoryPoints,
		Rank:          t.Rank,
		ClosedAt:      t.ClosedAt,
		RequesterID:   t.RequesterID.String(),
		RequesterName: names[t.RequesterID],
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	}
	if t.AssigneeID != nil {
		id := t.AssigneeID.String()
		resp.AssigneeID = &id
		if name, ok := names[*t.AssigneeID]; ok {
			resp.AssigneeName = &name
		}
	}
	if t.ParentID != nil {
		id := t.ParentID.String()
		resp.ParentID = &id
	}
	return resp
}

func toTicketResponses(tickets []*domain.Ticket, names map[uuid.UUID]string) []ticketResponse {
	out := make([]ticketResponse, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, toTicketResponse(t, names))
	}
	return out
}

// personIDs devuelve los usuarios que aparecen en las incidencias (solicitante y responsable).
func personIDs(tickets ...*domain.Ticket) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(tickets)*2)
	for _, t := range tickets {
		ids = append(ids, t.RequesterID)
		if t.AssigneeID != nil {
			ids = append(ids, *t.AssigneeID)
		}
	}
	return ids
}

func toCommentResponse(comment *domain.Comment) commentResponse {
	return commentResponse{
		ID: comment.ID.String(), IssueID: comment.IssueID.String(), AuthorID: comment.AuthorID.String(),
		Body: comment.Body, CreatedAt: comment.CreatedAt, UpdatedAt: comment.UpdatedAt,
		DeletedAt: comment.DeletedAt,
	}
}

func toCommentResponses(comments []*domain.Comment) []commentResponse {
	out := make([]commentResponse, 0, len(comments))
	for _, comment := range comments {
		out = append(out, toCommentResponse(comment))
	}
	return out
}

func toActivityResponses(activities []*domain.Activity) []activityResponse {
	out := make([]activityResponse, 0, len(activities))
	for _, activity := range activities {
		out = append(out, activityResponse{
			ID: activity.ID.String(), IssueID: activity.IssueID.String(), ActorID: activity.ActorID.String(),
			Action: activity.Action, FieldName: activity.FieldName, OldValue: activity.OldValue,
			NewValue: activity.NewValue, CreatedAt: activity.CreatedAt,
		})
	}
	return out
}

func toIssueLinkResponse(link *domain.IssueLink) issueLinkResponse {
	return issueLinkResponse{
		ID: link.ID.String(), SourceIssueID: link.SourceIssueID.String(),
		TargetIssueID: link.TargetIssueID.String(), Kind: string(link.Kind),
		CreatedBy: link.CreatedBy.String(), CreatedAt: link.CreatedAt,
	}
}

func toIssueLinkResponses(links []*domain.IssueLink) []issueLinkResponse {
	out := make([]issueLinkResponse, 0, len(links))
	for _, link := range links {
		out = append(out, toIssueLinkResponse(link))
	}
	return out
}

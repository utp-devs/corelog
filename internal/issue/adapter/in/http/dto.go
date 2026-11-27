package http

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/issue/domain"
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

type createIssueRequest struct {
	Summary     string                     `json:"summary" validate:"required,min=3,max=200"`
	Description string                     `json:"description" validate:"required,min=3"`
	Priority    string                     `json:"priority" validate:"omitempty,oneof=low medium high critical"`
	PriorityID  string                     `json:"priority_id" validate:"omitempty,uuid"`
	ProjectID   string                     `json:"project_id" validate:"omitempty,uuid"`
	IssueType   string                     `json:"issue_type" validate:"omitempty,oneof=task bug story epic subtask"`
	IssueTypeID string                     `json:"issue_type_id" validate:"omitempty,uuid"`
	ParentID    string                     `json:"parent_id" validate:"omitempty,uuid4"`
	DueDate     *time.Time                 `json:"due_date"`
	StoryPoints *float64                   `json:"story_points"`
	Custom      map[string]json.RawMessage `json:"custom"`
}

type updateMetadataRequest struct {
	IssueType   string     `json:"issue_type" validate:"omitempty,oneof=task bug story epic subtask"`
	IssueTypeID string     `json:"issue_type_id" validate:"omitempty,uuid"`
	ParentID    string     `json:"parent_id" validate:"omitempty,uuid4"`
	DueDate     *time.Time `json:"due_date"`
	StoryPoints *float64   `json:"story_points"`
}

type updateDetailsRequest struct {
	Summary     string                     `json:"summary" validate:"required,min=3,max=200"`
	Description string                     `json:"description" validate:"required,min=3"`
	Priority    string                     `json:"priority" validate:"omitempty,oneof=low medium high critical"`
	PriorityID  string                     `json:"priority_id" validate:"omitempty,uuid"`
	IssueType   string                     `json:"issue_type" validate:"omitempty,oneof=task bug story epic subtask"`
	IssueTypeID string                     `json:"issue_type_id" validate:"omitempty,uuid"`
	ParentID    string                     `json:"parent_id" validate:"omitempty,uuid4"`
	DueDate     *time.Time                 `json:"due_date"`
	StoryPoints *float64                   `json:"story_points"`
	Custom      map[string]json.RawMessage `json:"custom"`
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

type issueResponse struct {
	ID           string                     `json:"id"`
	Key          string                     `json:"key"`
	ProjectID    string                     `json:"project_id"`
	ProjectKey   string                     `json:"project_key"`
	Summary      string                     `json:"summary"`
	Description  string                     `json:"description"`
	Status       string                     `json:"status"`
	IssueType    *issueTypeRef              `json:"issue_type"`
	Priority     *priorityRef               `json:"priority"`
	Resolution   *resolutionRef             `json:"resolution"`
	ResolvedAt   *time.Time                 `json:"resolved_at"`
	ParentID     *string                    `json:"parent_id,omitempty"`
	DueDate      *time.Time                 `json:"due_date,omitempty"`
	StoryPoints  *float64                   `json:"story_points,omitempty"`
	Rank         string                     `json:"rank"`
	ClosedAt     *time.Time                 `json:"closed_at,omitempty"`
	ReporterID   string                     `json:"reporter_id"`
	ReporterName string                     `json:"reporter_name"`
	AssigneeID   *string                    `json:"assignee_id,omitempty"`
	AssigneeName *string                    `json:"assignee_name,omitempty"`
	CreatedAt    time.Time                  `json:"created_at"`
	Custom       map[string]any             `json:"custom"`
	CustomFields []issueCustomFieldResponse `json:"custom_fields"`
	UpdatedAt    time.Time                  `json:"updated_at"`
}

// toIssueResponse arma la respuesta de una incidencia. names contiene el nombre de cada persona implicada.
func toIssueResponse(t *domain.Issue, names map[uuid.UUID]string) issueResponse {
	resp := issueResponse{
		ID:           t.ID.String(),
		Key:          t.Key(),
		ProjectID:    t.ProjectID.String(),
		ProjectKey:   t.ProjectKey,
		Summary:      t.Summary,
		Description:  t.Description,
		Status:       string(t.Status),
		IssueType:    toIssueTypeRef(t.IssueTypeRef),
		Priority:     toPriorityRef(t.PriorityRef),
		Resolution:   toResolutionRef(t.ResolutionRef),
		ResolvedAt:   t.ResolvedAt,
		DueDate:      t.DueDate,
		StoryPoints:  t.StoryPoints,
		Rank:         t.Rank,
		ClosedAt:     t.ClosedAt,
		ReporterID:   t.ReporterID.String(),
		ReporterName: names[t.ReporterID],
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
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

func toIssueResponses(issues []*domain.Issue, names map[uuid.UUID]string) []issueResponse {
	out := make([]issueResponse, 0, len(issues))
	for _, t := range issues {
		out = append(out, toIssueResponse(t, names))
	}
	return out
}

// personIDs devuelve los usuarios que aparecen en las incidencias (solicitante y responsable).
func personIDs(issues ...*domain.Issue) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(issues)*2)
	for _, t := range issues {
		ids = append(ids, t.ReporterID)
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

// issueTypeRef is the issue type of an issue as the API shows it.
type issueTypeRef struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Icon           string `json:"icon"`
	Color          string `json:"color"`
	HierarchyLevel int    `json:"hierarchy_level"`
}

// priorityRef is the priority of an issue as the API shows it.
type priorityRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

// resolutionRef is the resolution of a done issue as the API shows it.
type resolutionRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func toIssueTypeRef(ref *domain.CatalogRef) *issueTypeRef {
	if ref == nil {
		return nil
	}
	return &issueTypeRef{ID: ref.ID.String(), Name: ref.Name, Icon: ref.Icon, Color: ref.Color, HierarchyLevel: ref.HierarchyLevel}
}

func toPriorityRef(ref *domain.CatalogRef) *priorityRef {
	if ref == nil {
		return nil
	}
	return &priorityRef{ID: ref.ID.String(), Name: ref.Name, Icon: ref.Icon, Color: ref.Color}
}

func toResolutionRef(ref *domain.CatalogRef) *resolutionRef {
	if ref == nil {
		return nil
	}
	return &resolutionRef{ID: ref.ID.String(), Name: ref.Name}
}

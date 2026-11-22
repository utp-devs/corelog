package http

import "github.com/google/uuid"

type boardResponse struct {
	ID             uuid.UUID             `json:"id"`
	ProjectID      uuid.UUID             `json:"project_id"`
	Name           string                `json:"name"`
	Type           string                `json:"type"`
	Columns        []boardColumnResponse `json:"columns"`
	CanManageBoard bool                  `json:"can_manage_board"`
}

type boardColumnResponse struct {
	ID       uuid.UUID           `json:"id"`
	Name     string              `json:"name"`
	Status   string              `json:"status"`
	Position int                 `json:"position"`
	WIPLimit *int                `json:"wip_limit,omitempty"`
	Count    int                 `json:"count"`
	Cards    []boardCardResponse `json:"cards"`
}

type boardCardResponse struct {
	ID          uuid.UUID  `json:"id"`
	Type        string     `json:"type"`
	Summary     string     `json:"summary"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	AssigneeID  *uuid.UUID `json:"assignee_id,omitempty"`
	Assignee    *string    `json:"assignee,omitempty"`
	StoryPoints *float64   `json:"story_points,omitempty"`
	Rank        string     `json:"rank"`
}

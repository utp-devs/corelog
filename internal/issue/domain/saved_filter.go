package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type SavedFilterCriteria struct {
	Status    Status    `json:"status,omitempty"`
	Priority  Priority  `json:"priority,omitempty"`
	Query     string    `json:"query,omitempty"`
	IssueType IssueType `json:"issue_type,omitempty"`
	DueFrom   string    `json:"due_from,omitempty"`
	DueTo     string    `json:"due_to,omitempty"`
}

type SavedFilter struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Name      string
	Criteria  SavedFilterCriteria
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewSavedFilter(ownerID uuid.UUID, name string, criteria SavedFilterCriteria) (*SavedFilter, error) {
	name = strings.TrimSpace(name)
	if ownerID == uuid.Nil || name == "" || len([]rune(name)) > 80 {
		return nil, ErrInvalidSavedFilter
	}
	if criteria.Status != "" && !criteria.Status.IsValid() {
		return nil, ErrInvalidSavedFilter
	}
	if criteria.Priority != "" && !criteria.Priority.IsValid() {
		return nil, ErrInvalidSavedFilter
	}
	if criteria.IssueType != "" && !criteria.IssueType.IsValid() {
		return nil, ErrInvalidSavedFilter
	}
	if len([]rune(criteria.Query)) > 200 {
		return nil, ErrInvalidSavedFilter
	}
	var dueFrom, dueTo time.Time
	var hasDueFrom, hasDueTo bool
	if criteria.DueFrom != "" {
		var err error
		dueFrom, err = time.Parse(time.RFC3339, criteria.DueFrom)
		if err != nil {
			return nil, ErrInvalidSavedFilter
		}
		hasDueFrom = true
	}
	if criteria.DueTo != "" {
		var err error
		dueTo, err = time.Parse(time.RFC3339, criteria.DueTo)
		if err != nil {
			return nil, ErrInvalidSavedFilter
		}
		hasDueTo = true
	}
	if hasDueFrom && hasDueTo && dueFrom.After(dueTo) {
		return nil, ErrInvalidSavedFilter
	}
	criteria.Query = strings.TrimSpace(criteria.Query)
	return &SavedFilter{OwnerID: ownerID, Name: name, Criteria: criteria, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, nil
}

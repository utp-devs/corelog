package usecase

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// WithCustomFields fixes the custom field scope reader and the user visibility check. Without them issues
// carry no custom values and ignore the "custom" input.
func (s *IssueService) WithCustomFields(scope port.CustomFieldScopeReader, browse port.ProjectBrowseChecker) *IssueService {
	s.customScope = scope
	s.browse = browse
	return s
}

// applyCustom validates the custom input of a write and merges it into the issue. create applies defaults and
// checks every required field. An update checks the required fields it names, or all of them when the issue
// changed type (its fields changed with the context). A type change also drops the values out of scope.
func (s *IssueService) applyCustom(ctx context.Context, issue *domain.Issue, create bool, previousType uuid.UUID, input map[string]json.RawMessage) error {
	if s.customScope == nil {
		return nil
	}
	typeChanged := !create && previousType != issue.IssueTypeID
	if input == nil && !typeChanged && !create {
		return nil
	}
	scope, err := s.customScope.ScopeFor(ctx, issue.ProjectID, issue.IssueTypeID)
	if err != nil {
		return err
	}
	current := issue.Custom
	if typeChanged {
		current = domain.PruneCustomValues(current, scope)
	}
	result := domain.ApplyCustomValues(domain.CustomWrite{
		Scope:      scope,
		Current:    current,
		Input:      input,
		Create:     create,
		RequireAll: create || typeChanged,
	})
	problems := result.Problems
	userFields := make(map[string]bool, len(scope))
	for _, entry := range scope {
		if entry.Field.Type == domain.FieldUser {
			userFields[entry.Field.ID.String()] = true
		}
	}
	for key, value := range result.Written {
		if value == nil || !userFields[key] || s.browse == nil {
			continue
		}
		userID, err := uuid.Parse(value.(string))
		if err != nil {
			continue
		}
		visible, err := s.browse.CanBrowse(ctx, issue.ProjectID, userID)
		if err != nil {
			return err
		}
		if !visible {
			problems = append(problems, domain.FieldProblem{FieldID: key, Message: "la persona no puede ver el proyecto"})
		}
	}
	if len(problems) > 0 {
		return domain.ErrCustomFieldInvalid(problems)
	}
	issue.Custom = result.Values
	return nil
}

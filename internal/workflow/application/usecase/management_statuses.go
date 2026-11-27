package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// ListStatuses returns the status catalog of an organization. Any member may read it.
func (m *Management) ListStatuses(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) ([]port.OrgStatus, error) {
	if err := m.orgs.RequireOrgRole(ctx, actorID, orgID, orgRoleGuest); err != nil {
		return nil, err
	}
	return m.repo.ListStatuses(ctx, orgID)
}

// CreateStatus appends a status to the catalog. Organization administrators only.
func (m *Management) CreateStatus(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, name string, category domain.Category) (port.OrgStatus, error) {
	if err := m.orgs.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return port.OrgStatus{}, err
	}
	name, err := validateStatus(name, category)
	if err != nil {
		return port.OrgStatus{}, err
	}
	return m.repo.CreateStatus(ctx, port.OrgStatus{OrganizationID: orgID, Name: name, Category: category})
}

// UpdateStatus renames a status or changes its category. Renames and category changes are written to every
// workflow that uses the status; a category change that leaves one of those workflows invalid is refused.
func (m *Management) UpdateStatus(ctx context.Context, actorID uuid.UUID, orgID, id uuid.UUID, patch port.StatusPatch) (port.OrgStatus, error) {
	if err := m.orgs.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return port.OrgStatus{}, err
	}
	current, err := m.findStatus(ctx, orgID, id)
	if err != nil {
		return port.OrgStatus{}, err
	}
	next := current
	if patch.Name != nil {
		next.Name = *patch.Name
	}
	if patch.Category != nil {
		next.Category = *patch.Category
	}
	if next.Name, err = validateStatusName(next.Name, next.Category); err != nil {
		return port.OrgStatus{}, err
	}
	if next == current {
		return current, nil
	}

	err = m.tx.InTx(ctx, func(ctx context.Context) error {
		now := m.clock.Now().UTC()
		workflows, err := m.repo.WorkflowsUsing(ctx, orgID, id)
		if err != nil {
			return err
		}
		for _, rec := range workflows {
			w := withStatus(rec.Workflow, id.String(), next.Name, next.Category)
			if next.Category != current.Category && len(w.Validate()) > 0 {
				return port.ErrCategoryInUse
			}
			if _, err := m.repo.SaveWorkflow(ctx, rec.ProjectID, w, rec.Version, now); err != nil {
				return err
			}
		}
		if err := m.repo.UpdateStatus(ctx, next); err != nil {
			return err
		}
		if next.Category != current.Category {
			return m.repo.RefreshLegacyStatus(ctx, id, next.Category, now)
		}
		return nil
	})
	if err != nil {
		return port.OrgStatus{}, err
	}
	return next, nil
}

// DeleteStatus removes a status. Without reassignTo the status must be unused. With reassignTo (a status of the
// same organization) every issue, history row and workflow that uses it moves to that status first.
func (m *Management) DeleteStatus(ctx context.Context, actorID uuid.UUID, orgID, id uuid.UUID, reassignTo uuid.UUID) (port.DeleteResult, error) {
	if err := m.orgs.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return port.DeleteResult{}, err
	}
	current, err := m.findStatus(ctx, orgID, id)
	if err != nil {
		return port.DeleteResult{}, err
	}
	var result port.DeleteResult
	err = m.tx.InTx(ctx, func(ctx context.Context) error {
		issues, history, err := m.repo.StatusUsage(ctx, orgID, id)
		if err != nil {
			return err
		}
		workflows, err := m.repo.WorkflowsUsing(ctx, orgID, id)
		if err != nil {
			return err
		}
		if reassignTo == uuid.Nil {
			if issues > 0 || history > 0 || len(workflows) > 0 {
				return port.ErrStatusInUse
			}
			return m.repo.DeleteStatus(ctx, orgID, id)
		}
		if reassignTo == id {
			return port.ErrReassignInvalid
		}
		target, err := m.findStatus(ctx, orgID, reassignTo)
		if err != nil {
			return port.ErrReassignInvalid
		}
		now := m.clock.Now().UTC()
		moved, err := m.repo.MigrateIssues(ctx, port.MigrationScope{OrganizationID: orgID}, current, target, actorID, now)
		if err != nil {
			return err
		}
		result.ReassignedIssues = moved
		if err := m.repo.RewriteStatusHistory(ctx, id, reassignTo); err != nil {
			return err
		}
		to := domain.Status{ID: reassignTo.String(), Name: target.Name, Category: target.Category}
		for _, rec := range workflows {
			w := reassignInWorkflow(rec.Workflow, id.String(), to)
			if issues := w.Validate(); len(issues) > 0 {
				return &port.ValidationFailure{Err: port.ErrReassignInvalid, Issues: issues}
			}
			if _, err := m.repo.SaveWorkflow(ctx, rec.ProjectID, w, rec.Version, now); err != nil {
				return err
			}
		}
		return m.repo.DeleteStatus(ctx, orgID, id)
	})
	if err != nil {
		return port.DeleteResult{}, err
	}
	return result, nil
}

// ReorderStatuses stores the catalog order. ids must name every status of the organization exactly once.
func (m *Management) ReorderStatuses(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, ids []uuid.UUID) ([]port.OrgStatus, error) {
	if err := m.orgs.RequireOrgRole(ctx, actorID, orgID, orgRoleAdmin); err != nil {
		return nil, err
	}
	list, err := m.repo.ListStatuses(ctx, orgID)
	if err != nil {
		return nil, err
	}
	known := make(map[uuid.UUID]bool, len(list))
	for _, s := range list {
		known[s.ID] = true
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	if len(ids) != len(list) {
		return nil, port.ErrStatusOrderInvalid
	}
	for _, id := range ids {
		if !known[id] || seen[id] {
			return nil, port.ErrStatusOrderInvalid
		}
		seen[id] = true
	}
	err = m.tx.InTx(ctx, func(ctx context.Context) error {
		return m.repo.SetStatusPositions(ctx, orgID, ids)
	})
	if err != nil {
		return nil, err
	}
	return m.repo.ListStatuses(ctx, orgID)
}

// findStatus returns the status with the id from the organization, or port.ErrStatusNotFound.
func (m *Management) findStatus(ctx context.Context, orgID, id uuid.UUID) (port.OrgStatus, error) {
	list, err := m.repo.ListStatuses(ctx, orgID)
	if err != nil {
		return port.OrgStatus{}, err
	}
	for _, s := range list {
		if s.ID == id {
			return s, nil
		}
	}
	return port.OrgStatus{}, port.ErrStatusNotFound
}

// validateStatusName applies the catalog rules to a name and returns it trimmed.
func validateStatusName(name string, category domain.Category) (string, error) {
	return validateStatus(name, category)
}

// withStatus returns a copy of the workflow whose status id carries the new name and category.
func withStatus(w domain.Workflow, id, name string, category domain.Category) domain.Workflow {
	out := domain.Workflow{Initial: w.Initial, Transitions: w.Transitions, Statuses: make([]domain.Status, 0, len(w.Statuses))}
	for _, s := range w.Statuses {
		if s.ID == id {
			s.Name = name
			s.Category = category
		}
		out.Statuses = append(out.Statuses, s)
	}
	return out
}

// reassignInWorkflow replaces the status from with the status to: the initial status, the statuses and the
// transition ends. A status that would appear twice is kept once.
func reassignInWorkflow(w domain.Workflow, from string, to domain.Status) domain.Workflow {
	out := domain.Workflow{Initial: w.Initial}
	if out.Initial == from {
		out.Initial = to.ID
	}
	seen := make(map[string]bool, len(w.Statuses))
	for _, s := range w.Statuses {
		if s.ID == from {
			s = to
		}
		if seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		out.Statuses = append(out.Statuses, s)
	}
	for _, t := range w.Transitions {
		nt := t
		nt.From = nil
		fromSeen := map[string]bool{}
		for _, f := range t.From {
			if f == from {
				f = to.ID
			}
			if !fromSeen[f] {
				fromSeen[f] = true
				nt.From = append(nt.From, f)
			}
		}
		if t.To == from {
			nt.To = to.ID
		}
		out.Transitions = append(out.Transitions, nt)
	}
	return out
}

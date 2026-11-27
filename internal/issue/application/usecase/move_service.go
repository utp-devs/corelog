package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// maxMoveDepth bounds the walk down the children of a moved issue.
const maxMoveDepth = 16

// projectCatalogLister is implemented by the catalog adapter. It lists the values a project accepts, which the
// move error shows as the options of a required mapping. Without it the options list is empty.
type projectCatalogLister interface {
	ProjectOrganization(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error)
	ListItems(ctx context.Context, orgID uuid.UUID, kind domain.CatalogKind) ([]*domain.CatalogItem, error)
	ProjectIssueTypes(ctx context.Context, projectID uuid.UUID) ([]*domain.CatalogItem, bool, error)
}

// movePlan is the planned state of one issue of the moved tree.
type movePlan struct {
	issue    *domain.Issue
	parent   *movePlan
	children []*movePlan
	typeItem *domain.CatalogItem
	prioItem *domain.CatalogItem
	status   domain.Status
	level    int
	custom   map[string]any
	dropped  []string
}

// MoveIssue moves an issue and its live children to another project. Each issue gets a new key from the target
// counter, and the old key stays an alias. The whole tree moves in one transaction: either every issue moves or none.
func (s *IssueService) MoveIssue(ctx context.Context, id, actorID uuid.UUID, globalRole string, in port.MoveInput) (*port.MoveResult, error) {
	if s.ops == nil || s.ops.moves == nil {
		return nil, errOperationsUnavailable()
	}
	for _, status := range in.Mapping.Statuses {
		if !status.IsValid() {
			return nil, domain.ErrMoveMappingInvalid
		}
	}
	var result *port.MoveResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = s.moveTree(ctx, id, actorID, globalRole, in)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *IssueService) moveTree(ctx context.Context, id, actorID uuid.UUID, globalRole string, in port.MoveInput) (*port.MoveResult, error) {
	if s.lifecycle != nil {
		if err := s.lifecycle.LockForUpdate(ctx, id); err != nil {
			return nil, err
		}
	}
	root, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireVisible(ctx, root.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if root.ProjectID == in.ProjectID {
		return nil, domain.ErrMoveSameProject
	}
	if err := s.requireIssueCreate(ctx, in.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	plans, err := s.planMove(ctx, root, actorID, globalRole, in)
	if err != nil {
		return nil, err
	}
	warnings := []string{}
	moved := []port.MovedIssue{}
	for _, plan := range plans {
		fromKey := plan.issue.Key()
		next, notes, err := s.applyMove(ctx, plan, actorID, in.ProjectID)
		if err != nil {
			return nil, err
		}
		warnings = append(warnings, notes...)
		moved = append(moved, port.MovedIssue{ID: next.ID, FromKey: fromKey, ToKey: next.Key()})
	}
	issue, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &port.MoveResult{Issue: issue, Warnings: warnings, Moved: moved}, nil
}

// planMove collects the tree, resolves the target values and checks every rule the moved issues must satisfy.
// It writes nothing. Required mappings are reported together.
func (s *IssueService) planMove(ctx context.Context, root *domain.Issue, actorID uuid.UUID, globalRole string, in port.MoveInput) ([]*movePlan, error) {
	rootPlan := &movePlan{issue: root}
	order := []*movePlan{rootPlan}
	queue := []*movePlan{rootPlan}
	depth := map[*movePlan]int{rootPlan: 0}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if err := s.requireFieldEdit(ctx, node.issue, actorID, globalRole); err != nil {
			return nil, err
		}
		if depth[node] >= maxMoveDepth || s.children == nil {
			continue
		}
		children, err := s.children.ListChildren(ctx, node.issue.ID)
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			childPlan := &movePlan{issue: child, parent: node}
			node.children = append(node.children, childPlan)
			depth[childPlan] = depth[node] + 1
			order = append(order, childPlan)
			queue = append(queue, childPlan)
		}
	}

	var required []domain.RequiredMapping
	seen := map[string]bool{}
	require := func(item domain.RequiredMapping) {
		key := item.Field + "/" + item.From
		if !seen[key] {
			seen[key] = true
			required = append(required, item)
		}
	}
	lister, _ := s.catalog.(projectCatalogLister)
	for _, plan := range order {
		if err := s.planValues(ctx, plan, in, lister, require); err != nil {
			return nil, err
		}
	}
	if len(required) > 0 {
		return nil, domain.MoveMappingRequiredError(required)
	}
	for _, plan := range order {
		var parentLevel int
		if plan.parent != nil {
			parentLevel = plan.parent.level
		}
		if err := domain.ParentRule(plan.level, plan.parent != nil, parentLevel, true); err != nil {
			return nil, err
		}
	}
	if s.customScope != nil {
		for _, plan := range order {
			if err := s.planCustom(ctx, plan, in.ProjectID); err != nil {
				return nil, err
			}
		}
	}
	return order, nil
}

// planValues resolves the type, priority and status of one issue in the target project. A value the target does
// not accept is either mapped by the caller or reported as required.
func (s *IssueService) planValues(ctx context.Context, plan *movePlan, in port.MoveInput, lister projectCatalogLister, require func(domain.RequiredMapping)) error {
	issue := plan.issue
	plan.status = issue.Status
	if mapped, ok := in.Mapping.Statuses[issue.Status]; ok {
		plan.status = mapped
	}
	plan.level = issue.HierarchyLevel()
	if s.catalog == nil {
		return nil
	}

	typeID, explicit := issue.IssueTypeID, false
	if plan.parent == nil && in.IssueTypeID != nil {
		typeID, explicit = *in.IssueTypeID, true
	} else if mapped, ok := in.Mapping.IssueTypes[issue.IssueTypeID]; ok {
		typeID, explicit = mapped, true
	}
	if typeID != uuid.Nil {
		enabled, err := s.catalog.IssueTypeEnabled(ctx, in.ProjectID, typeID)
		if err != nil {
			return err
		}
		if !enabled {
			if explicit {
				return domain.ErrMoveMappingInvalid
			}
			require(domain.RequiredMapping{Field: domain.MappingIssueType, From: issue.IssueTypeID.String(), Options: s.catalogOptions(ctx, lister, in.ProjectID, domain.CatalogIssueType)})
		} else {
			item, err := s.catalog.Select(ctx, in.ProjectID, domain.CatalogIssueType, port.CatalogSelector{ID: typeID})
			if err != nil {
				return err
			}
			plan.typeItem = item
			plan.level = item.HierarchyLevel
		}
	}

	if issue.PriorityID != uuid.Nil {
		prioID, explicit := issue.PriorityID, false
		if mapped, ok := in.Mapping.Priorities[issue.PriorityID]; ok {
			prioID, explicit = mapped, true
		}
		item, err := s.catalog.Select(ctx, in.ProjectID, domain.CatalogPriority, port.CatalogSelector{ID: prioID})
		switch {
		case err == nil:
			plan.prioItem = item
		case apperror.KindOf(err) == apperror.KindNotFound && explicit:
			return domain.ErrMoveMappingInvalid
		case apperror.KindOf(err) == apperror.KindNotFound:
			require(domain.RequiredMapping{Field: domain.MappingPriority, From: issue.PriorityID.String(), Options: s.catalogOptions(ctx, lister, in.ProjectID, domain.CatalogPriority)})
		default:
			return err
		}
	}
	return nil
}

// catalogOptions lists the ids the target project accepts for a catalog kind, for the required-mapping answer.
func (s *IssueService) catalogOptions(ctx context.Context, lister projectCatalogLister, projectID uuid.UUID, kind domain.CatalogKind) []string {
	if lister == nil {
		return []string{}
	}
	var items []*domain.CatalogItem
	var err error
	if kind == domain.CatalogIssueType {
		items, _, err = lister.ProjectIssueTypes(ctx, projectID)
	} else {
		var orgID uuid.UUID
		orgID, err = lister.ProjectOrganization(ctx, projectID)
		if err == nil {
			items, err = lister.ListItems(ctx, orgID, kind)
		}
	}
	options := []string{}
	if err != nil {
		return options
	}
	for _, item := range items {
		options = append(options, fmt.Sprintf("%s (%s)", item.ID, item.Name))
	}
	return options
}

// planCustom keeps the custom values the target project defines for the planned type and lists the dropped ones.
func (s *IssueService) planCustom(ctx context.Context, plan *movePlan, projectID uuid.UUID) error {
	typeID := plan.issue.IssueTypeID
	if plan.typeItem != nil {
		typeID = plan.typeItem.ID
	}
	scope, err := s.customScope.ScopeFor(ctx, projectID, typeID)
	if err != nil {
		return err
	}
	kept := domain.PruneCustomValues(plan.issue.Custom, scope)
	for key := range plan.issue.Custom {
		if _, ok := kept[key]; !ok {
			plan.dropped = append(plan.dropped, key)
		}
	}
	plan.custom = kept
	return nil
}

// applyMove relocates one planned issue and writes its history and event. It returns the issue as moved and the
// warnings of that issue.
func (s *IssueService) applyMove(ctx context.Context, plan *movePlan, actorID, targetProjectID uuid.UUID) (*domain.Issue, []string, error) {
	issue := plan.issue
	fromKey := issue.Key()
	fromProject := issue.ProjectKey
	before := issue.Custom
	if before == nil {
		before = map[string]any{}
	}

	relocated, err := s.ops.moves.Relocate(ctx, issue.ID, issue.ProjectID, issue.KeyNumber, targetProjectID)
	if err != nil {
		return nil, nil, err
	}
	changes := []domain.Change{
		domain.NewChange("project", fromProject, relocated.ProjectKey),
		domain.NewChange("key", fromKey, domain.KeyAliasText(relocated.ProjectKey, relocated.KeyNumber)),
	}
	issue.ProjectID = targetProjectID
	issue.KeyNumber = relocated.KeyNumber
	issue.ProjectKey = relocated.ProjectKey

	if plan.typeItem != nil {
		changes = append(changes, domain.NewChange("issue_type", typeKey(issue), plan.typeItem.ID.String()))
		issue.IssueTypeID = plan.typeItem.ID
		issue.IssueType = domain.LegacyIssueTypeValue(plan.typeItem)
		issue.IssueTypeRef = domain.RefOf(plan.typeItem)
	}
	if plan.prioItem != nil {
		changes = append(changes, domain.NewChange("priority", priorityKey(issue), plan.prioItem.ID.String()))
		issue.PriorityID = plan.prioItem.ID
		issue.Priority = domain.LegacyPriorityValue(plan.prioItem)
		issue.PriorityRef = domain.RefOf(plan.prioItem)
	}
	if plan.status != issue.Status {
		changes = append(changes, domain.NewChange("status", string(issue.Status), string(plan.status)))
		issue.Status = plan.status
	}
	if plan.parent == nil && issue.ParentID != nil {
		changes = append(changes, domain.NewChange("parent", idString(issue.ParentID), ""))
		issue.ParentID = nil
	}
	if plan.custom != nil {
		issue.Custom = plan.custom
		changes = append(changes, customChanges(before, issue.Custom)...)
	}
	if s.lifecycle != nil {
		rank, err := s.lifecycle.NextRank(ctx, targetProjectID)
		if err != nil {
			return nil, nil, err
		}
		issue.Rank = rank
	}
	issue.UpdatedAt = s.now()
	if err := s.repo.Update(ctx, issue); err != nil {
		return nil, nil, err
	}
	if err := s.commitChanges(ctx, issue, actorID, changes); err != nil {
		return nil, nil, err
	}
	return issue, moveWarnings(issue.Key(), plan, relocated), nil
}

// aliasCandidates returns the issues that had the key before a move. Visibility is checked by ResolveKey.
func (s *IssueService) aliasCandidates(ctx context.Context, projectKey string, number int, orgID uuid.UUID) ([]*domain.Issue, error) {
	if s.ops == nil || s.ops.moves == nil {
		return nil, nil
	}
	ids, err := s.ops.moves.FindAliasIssues(ctx, projectKey, number, orgID)
	if err != nil {
		return nil, err
	}
	var out []*domain.Issue
	for _, id := range ids {
		issue, err := s.repo.FindByID(ctx, id)
		if apperror.KindOf(err) == apperror.KindNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, issue)
	}
	return out, nil
}

// moveWarnings describes what the target project did not keep of one issue.
func moveWarnings(key string, plan *movePlan, relocated port.RelocateResult) []string {
	var notes []string
	for _, field := range plan.dropped {
		notes = append(notes, fmt.Sprintf("%s: el campo personalizado %s no aplica en el proyecto destino y se elimino", key, field))
	}
	if relocated.DroppedLabels > 0 {
		notes = append(notes, fmt.Sprintf("%s: se quitaron %d etiquetas del proyecto origen", key, relocated.DroppedLabels))
	}
	if relocated.DroppedComponents > 0 {
		notes = append(notes, fmt.Sprintf("%s: se quitaron %d componentes del proyecto origen", key, relocated.DroppedComponents))
	}
	return notes
}

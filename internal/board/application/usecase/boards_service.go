package usecase

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/rank"
)

// maxViewCards bounds the number of cards one view loads.
const maxViewCards = 5000

// ErrTransitionUnavailable is returned when no transition the caller may run leads into the target column.
// It has the same code the issue module uses for the same situation.
func errTransitionUnavailable() error {
	return apperror.WithCode(apperror.KindConflict, "issue.transition_unavailable", "no hay una transición disponible hacia esa columna")
}

// BoardsService implements the board v2 use cases.
type BoardsService struct {
	repo        port.BoardRepository
	projects    port.ProjectAuthorizer
	authorizer  port.Authorizer
	transitions port.CardTransitions
	editor      port.CardEditor
	filter      port.QueryFilter
	clock       clock.Clock
}

// NewBoardsService builds the service. editor may be nil: swimlane effects are then skipped.
func NewBoardsService(repo port.BoardRepository, projects port.ProjectAuthorizer, authorizer port.Authorizer,
	transitions port.CardTransitions, editor port.CardEditor, filter port.QueryFilter, clk clock.Clock) *BoardsService {
	return &BoardsService{repo: repo, projects: projects, authorizer: authorizer, transitions: transitions, editor: editor, filter: filter, clock: clk}
}

var _ port.BoardsService = (*BoardsService)(nil)

// load returns the board once the caller is allowed to see its project (a hidden project is a 404).
func (s *BoardsService) load(ctx context.Context, boardID, actorID uuid.UUID, globalRole string) (*domain.Board, error) {
	board, err := s.repo.FindByID(ctx, boardID)
	if err != nil {
		return nil, err
	}
	if _, err := s.projects.Get(ctx, actorID, globalRole, board.ProjectID); err != nil {
		return nil, err
	}
	return board, nil
}

func (s *BoardsService) can(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) bool {
	return s.authorizer.Authorize(ctx, action, actorID, globalRole, projectID) == nil
}

func (s *BoardsService) detail(ctx context.Context, board *domain.Board, actorID uuid.UUID, globalRole string) (*port.BoardDetail, error) {
	statuses, err := s.repo.WorkflowStatuses(ctx, board.ProjectID)
	if err != nil {
		return nil, err
	}
	return &port.BoardDetail{
		Board: board, WorkflowStatuses: statuses,
		CanManage:    s.can(ctx, port.ActionManageBoard, actorID, globalRole, board.ProjectID),
		CanMoveCards: s.can(ctx, port.ActionMoveCards, actorID, globalRole, board.ProjectID),
	}, nil
}

// List returns the boards of a project the caller can see.
func (s *BoardsService) List(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) ([]port.BoardSummary, error) {
	if _, err := s.projects.Get(ctx, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	boards, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]port.BoardSummary, 0, len(boards))
	for _, b := range boards {
		out = append(out, port.BoardSummary{ID: b.ID, ProjectID: b.ProjectID, Name: b.Name, Type: b.Type,
			IsDefault: b.IsDefault, ColumnCount: len(b.Columns), SwimlaneMode: b.SwimlaneMode})
	}
	return out, nil
}

// Get returns one board with its settings.
func (s *BoardsService) Get(ctx context.Context, boardID, actorID uuid.UUID, globalRole string) (*port.BoardDetail, error) {
	board, err := s.load(ctx, boardID, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, board, actorID, globalRole)
}

// Create adds a board to the project. Without columns it creates one per status category of the workflow.
func (s *BoardsService) Create(ctx context.Context, projectID, actorID uuid.UUID, globalRole string, in port.CreateBoardInput) (*port.BoardDetail, error) {
	project, err := s.projects.Get(ctx, actorID, globalRole, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizer.Authorize(ctx, port.ActionManageBoard, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	if in.Settings.SwimlaneMode == "" {
		in.Settings.SwimlaneMode = domain.SwimlanesNone
	}
	if in.Settings.Estimation == "" {
		in.Settings.Estimation = domain.EstimationStoryPoints
	}
	if in.Settings.CardFields == nil {
		in.Settings.CardFields = append([]string(nil), domain.DefaultCardFields...)
	}
	if err := domain.ValidateSettings(&in.Settings); err != nil {
		return nil, err
	}
	if in.Type == "" {
		in.Type = domain.Type(project.Type)
	}
	if in.Type != domain.TypeKanban && in.Type != domain.TypeScrum {
		return nil, domain.ErrInvalidBoard
	}
	existing, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if len(existing) >= domain.MaxBoardsPerProject {
		return nil, domain.ErrTooManyBoards
	}
	statuses, err := s.repo.WorkflowStatuses(ctx, projectID)
	if err != nil {
		return nil, err
	}
	specs := in.Columns
	if len(specs) == 0 {
		specs = defaultColumns(statuses)
	}
	if err := domain.ValidateColumns(specs, statusSet(statuses)); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	board := &domain.Board{
		ProjectID: projectID, Name: in.Settings.Name, Type: in.Type, IsDefault: len(existing) == 0,
		SwimlaneMode: in.Settings.SwimlaneMode, Estimation: in.Settings.Estimation, CardFields: in.Settings.CardFields,
		ShowUnmappedLane: in.Settings.ShowUnmappedLane, DoneRetentionDays: in.Settings.DoneRetentionDays,
		CreatedAt: now, UpdatedAt: now,
	}
	for i, spec := range specs {
		board.Columns = append(board.Columns, domain.Column{
			Name: spec.Name, Position: i, MinIssues: spec.MinIssues, WIPLimit: spec.MaxIssues, Color: spec.Color,
			StatusIDs: spec.StatusIDs, CreatedAt: now, UpdatedAt: now,
		})
	}
	for _, b := range existing {
		if strings.EqualFold(b.Name, board.Name) {
			return nil, domain.ErrBoardNameTaken
		}
	}
	if err := s.repo.Create(ctx, board); err != nil {
		return nil, err
	}
	return s.detail(ctx, board, actorID, globalRole)
}

// defaultColumns maps the statuses of the workflow into To do, In progress and Done columns by category.
func defaultColumns(statuses []port.WorkflowStatus) []domain.ColumnSpec {
	groups := []struct{ category, name, color string }{
		{"todo", "Por hacer", "gray"}, {"in_progress", "En curso", "blue"}, {"done", "Hecho", "green"},
	}
	var specs []domain.ColumnSpec
	for _, g := range groups {
		spec := domain.ColumnSpec{Name: g.name, Color: g.color}
		for _, st := range statuses {
			if st.Category == g.category {
				spec.StatusIDs = append(spec.StatusIDs, st.ID)
			}
		}
		if len(spec.StatusIDs) > 0 {
			specs = append(specs, spec)
		}
	}
	if len(specs) == 0 {
		specs = append(specs, domain.ColumnSpec{Name: "Por hacer"})
	}
	return specs
}

func statusSet(statuses []port.WorkflowStatus) map[uuid.UUID]bool {
	set := make(map[uuid.UUID]bool, len(statuses))
	for _, st := range statuses {
		set[st.ID] = true
	}
	return set
}

// manage loads the board and requires manage_board on its project.
func (s *BoardsService) manage(ctx context.Context, boardID, actorID uuid.UUID, globalRole string) (*domain.Board, error) {
	board, err := s.load(ctx, boardID, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	if err := s.authorizer.Authorize(ctx, port.ActionManageBoard, actorID, globalRole, board.ProjectID); err != nil {
		return nil, err
	}
	return board, nil
}

// Update changes the settings of a board.
func (s *BoardsService) Update(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, in port.UpdateBoardInput) (*port.BoardDetail, error) {
	board, err := s.manage(ctx, boardID, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	settings := domain.Settings{Name: board.Name, SwimlaneMode: board.SwimlaneMode, Estimation: board.Estimation,
		CardFields: board.CardFields, ShowUnmappedLane: board.ShowUnmappedLane, DoneRetentionDays: board.DoneRetentionDays}
	if in.Name != nil {
		settings.Name = *in.Name
	}
	if in.SwimlaneMode != nil {
		settings.SwimlaneMode = *in.SwimlaneMode
	}
	if in.Estimation != nil {
		settings.Estimation = *in.Estimation
	}
	if in.CardFieldsSet {
		settings.CardFields = in.CardFields
	}
	if in.ShowUnmappedLane != nil {
		settings.ShowUnmappedLane = *in.ShowUnmappedLane
	}
	if in.DoneRetentionSet {
		settings.DoneRetentionDays = in.DoneRetentionDays
	}
	if err := domain.ValidateSettings(&settings); err != nil {
		return nil, err
	}
	if !strings.EqualFold(settings.Name, board.Name) {
		boards, err := s.repo.ListByProject(ctx, board.ProjectID)
		if err != nil {
			return nil, err
		}
		for _, b := range boards {
			if b.ID != board.ID && strings.EqualFold(b.Name, settings.Name) {
				return nil, domain.ErrBoardNameTaken
			}
		}
	}
	board.Name, board.SwimlaneMode, board.Estimation = settings.Name, settings.SwimlaneMode, settings.Estimation
	board.CardFields, board.ShowUnmappedLane, board.DoneRetentionDays = settings.CardFields, settings.ShowUnmappedLane, settings.DoneRetentionDays
	board.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateSettings(ctx, board); err != nil {
		return nil, err
	}
	return s.detail(ctx, board, actorID, globalRole)
}

// Delete removes a board. The default board of a project cannot be deleted.
func (s *BoardsService) Delete(ctx context.Context, boardID, actorID uuid.UUID, globalRole string) error {
	board, err := s.manage(ctx, boardID, actorID, globalRole)
	if err != nil {
		return err
	}
	if board.IsDefault {
		return domain.ErrDefaultBoardNotDeletable
	}
	return s.repo.Delete(ctx, boardID)
}

// ReplaceColumns replaces all the columns of the board after validating the mapping.
func (s *BoardsService) ReplaceColumns(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, columns []domain.ColumnSpec) (*port.BoardDetail, error) {
	board, err := s.manage(ctx, boardID, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	statuses, err := s.repo.WorkflowStatuses(ctx, board.ProjectID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateColumns(columns, statusSet(statuses)); err != nil {
		return nil, err
	}
	own := map[uuid.UUID]bool{}
	for _, c := range board.Columns {
		own[c.ID] = true
	}
	for i, c := range columns {
		if c.ID != uuid.Nil && !own[c.ID] {
			return nil, apperror.WithCode(apperror.KindValidation, domain.ErrInvalidColumns.Code, domain.ErrInvalidColumns.Message).
				WithDetails(apperror.FieldError{Field: "columns[" + strconv.Itoa(i) + "].id", Message: "la columna no pertenece a este tablero"})
		}
	}
	if err := s.repo.ReplaceColumns(ctx, boardID, columns, s.clock.Now()); err != nil {
		return nil, err
	}
	return s.reloaded(ctx, boardID, actorID, globalRole)
}

func (s *BoardsService) reloaded(ctx context.Context, boardID, actorID uuid.UUID, globalRole string) (*port.BoardDetail, error) {
	board, err := s.repo.FindByID(ctx, boardID)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, board, actorID, globalRole)
}

// ReplaceQuickFilters replaces the quick filters of the board.
func (s *BoardsService) ReplaceQuickFilters(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, filters []port.QueryInput) (*port.BoardDetail, error) {
	if _, err := s.manage(ctx, boardID, actorID, globalRole); err != nil {
		return nil, err
	}
	names, queries := splitQueries(filters)
	if err := domain.ValidateQueries("quick_filters", names, queries); err != nil {
		return nil, err
	}
	items := make([]domain.QuickFilter, len(filters))
	for i := range filters {
		items[i] = domain.QuickFilter{BoardID: boardID, Name: strings.TrimSpace(names[i]), Query: strings.TrimSpace(queries[i]), Position: i}
	}
	if err := s.repo.ReplaceQuickFilters(ctx, boardID, items); err != nil {
		return nil, err
	}
	return s.reloaded(ctx, boardID, actorID, globalRole)
}

// ReplaceSwimlanes replaces the query swimlanes of the board.
func (s *BoardsService) ReplaceSwimlanes(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, lanes []port.QueryInput) (*port.BoardDetail, error) {
	if _, err := s.manage(ctx, boardID, actorID, globalRole); err != nil {
		return nil, err
	}
	names, queries := splitQueries(lanes)
	if err := domain.ValidateQueries("swimlanes", names, queries); err != nil {
		return nil, err
	}
	items := make([]domain.Swimlane, len(lanes))
	for i := range lanes {
		items[i] = domain.Swimlane{BoardID: boardID, Name: strings.TrimSpace(names[i]), Query: strings.TrimSpace(queries[i]), Position: i}
	}
	if err := s.repo.ReplaceSwimlanes(ctx, boardID, items); err != nil {
		return nil, err
	}
	return s.reloaded(ctx, boardID, actorID, globalRole)
}

func splitQueries(in []port.QueryInput) (names, queries []string) {
	for _, q := range in {
		names, queries = append(names, q.Name), append(queries, q.Query)
	}
	return names, queries
}

// View assembles the board: columns with cards, swimlanes, counts and WIP states.
func (s *BoardsService) View(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, query port.ViewQuery) (*port.ViewResult, error) {
	board, err := s.load(ctx, boardID, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	if query.Sprint.Mode == "" {
		query.Sprint.Mode = "all"
		if board.Type == domain.TypeScrum {
			query.Sprint.Mode = "active"
		}
	}
	cards, sprint, err := s.repo.Cards(ctx, port.CardQuery{ProjectID: board.ProjectID, Sprint: query.Sprint,
		DoneRetentionDays: board.DoneRetentionDays, Now: s.clock.Now(), Limit: maxViewCards})
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]domain.QuickFilter{}
	for _, f := range board.QuickFilters {
		byID[f.ID] = f
	}
	var quick []domain.Matcher
	for _, id := range query.QuickFilterIDs {
		f, ok := byID[id]
		if !ok {
			return nil, apperror.WithCode(apperror.KindValidation, domain.ErrInvalidQuery.Code, domain.ErrInvalidQuery.Message).
				WithDetails(apperror.FieldError{Field: "quick_filters", Message: "el filtro rápido " + id.String() + " no existe en el tablero"})
		}
		m, err := s.filter.Compile(f.Query)
		if err != nil {
			return nil, domain.ErrInvalidQuery
		}
		quick = append(quick, m)
	}
	var laneQueries []domain.Matcher
	if board.SwimlaneMode == domain.SwimlanesQuery {
		for _, lane := range board.Swimlanes {
			m, err := s.filter.Compile(lane.Query)
			if err != nil {
				return nil, domain.ErrInvalidQuery
			}
			laneQueries = append(laneQueries, m)
		}
	}
	view := domain.AssembleView(board, cards, quick, laneQueries, domain.QueryContext{CurrentUser: actorID})
	if query.Swimlane != "" {
		for i := range view.Columns {
			view.Columns[i].Cards = onlyLane(view.Columns[i].Cards, query.Swimlane)
		}
		view.Unmapped.Cards = onlyLane(view.Unmapped.Cards, query.Swimlane)
	}
	detail, err := s.detail(ctx, board, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	return &port.ViewResult{Detail: detail, View: view, Sprint: sprint}, nil
}

func onlyLane(cards []domain.PlacedCard, lane string) []domain.PlacedCard {
	out := make([]domain.PlacedCard, 0, len(cards))
	for _, c := range cards {
		if c.SwimlaneID == lane {
			out = append(out, c)
		}
	}
	return out
}

// MoveCard moves a card to a column through the workflow engine, ranks it between its neighbors and applies
// the effects of the swimlane it was dropped on. The WIP limit is advisory: the result flags it.
func (s *BoardsService) MoveCard(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, in port.MoveInput) (*port.MoveResult, error) {
	board, err := s.load(ctx, boardID, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	// The permission goes before any detail about the board or the card.
	if err := s.authorizer.Authorize(ctx, port.ActionMoveCards, actorID, globalRole, board.ProjectID); err != nil {
		return nil, err
	}
	if in.BeforeIssueID != uuid.Nil && in.AfterIssueID != uuid.Nil {
		return nil, domain.ErrInvalidMove
	}
	var target *domain.Column
	for i := range board.Columns {
		if board.Columns[i].ID == in.ColumnID {
			target = &board.Columns[i]
		}
	}
	if target == nil {
		return nil, domain.ErrColumnNotFound
	}
	card, err := s.repo.CardPlace(ctx, board.ProjectID, in.IssueID)
	if err != nil {
		return nil, err
	}
	inTarget := false
	for _, id := range target.StatusIDs {
		inTarget = inTarget || id == card.StatusID
	}
	result := &port.MoveResult{IssueID: in.IssueID, ColumnID: target.ID}
	if !inTarget {
		option, err := s.pickTransition(ctx, actorID, globalRole, in, target)
		if err != nil {
			return nil, err
		}
		if err := s.transitions.Run(ctx, actorID, globalRole, in.IssueID, option.ID, card.StatusID, in.Input); err != nil {
			return nil, err
		}
		result.Transitioned, result.TransitionID = true, option.ID
	}
	if in.BeforeIssueID != uuid.Nil || in.AfterIssueID != uuid.Nil {
		if result.Rank, err = s.rankCard(ctx, board.ProjectID, in); err != nil {
			return nil, err
		}
	}
	if err := s.laneEffects(ctx, board, card, actorID, globalRole, in); err != nil {
		return nil, err
	}
	after, err := s.repo.CardPlace(ctx, board.ProjectID, in.IssueID)
	if err != nil {
		return nil, err
	}
	statuses, err := s.repo.WorkflowStatuses(ctx, board.ProjectID)
	if err != nil {
		return nil, err
	}
	for _, st := range statuses {
		if st.ID == after.StatusID {
			result.Status = st
		}
	}
	count, err := s.repo.CountInStatuses(ctx, board.ProjectID, target.StatusIDs)
	if err != nil {
		return nil, err
	}
	state := domain.EvaluateWIP(count, target.MinIssues, target.WIPLimit)
	result.WIP = port.WIPFlag{ColumnID: target.ID, Count: count, Min: target.MinIssues, Max: target.WIPLimit, State: state, Exceeded: state == domain.WIPOverMax}
	return result, nil
}

// pickTransition resolves the transition that leads the card into the target column.
func (s *BoardsService) pickTransition(ctx context.Context, actorID uuid.UUID, globalRole string, in port.MoveInput, target *domain.Column) (port.TransitionOption, error) {
	options, err := s.transitions.Options(ctx, actorID, globalRole, in.IssueID)
	if err != nil {
		return port.TransitionOption{}, err
	}
	var candidates []port.TransitionOption
	for _, option := range options {
		for _, id := range target.StatusIDs {
			if option.ToStatusID == id {
				candidates = append(candidates, option)
				break
			}
		}
	}
	if in.TransitionID != "" {
		for _, c := range candidates {
			if c.ID == in.TransitionID {
				candidates = []port.TransitionOption{c}
				break
			}
		}
		if len(candidates) != 1 || candidates[0].ID != in.TransitionID {
			return port.TransitionOption{}, errTransitionUnavailable()
		}
	}
	switch len(candidates) {
	case 0:
		return port.TransitionOption{}, errTransitionUnavailable()
	case 1:
	default:
		details := make([]apperror.FieldError, 0, len(candidates))
		for _, c := range candidates {
			details = append(details, apperror.FieldError{Field: c.ID, Message: c.Name})
		}
		return port.TransitionOption{}, domain.NewAmbiguousTransition(details...)
	}
	option := candidates[0]
	if option.NeedsInput() && in.Input.Empty() {
		var fields []apperror.FieldError
		for _, f := range option.RequiredFields {
			fields = append(fields, apperror.FieldError{Field: "fields." + f, Message: "el campo es obligatorio"})
		}
		if option.RequiresComment {
			fields = append(fields, apperror.FieldError{Field: "comment", Message: "el comentario es obligatorio"})
		}
		if option.RequiresResolution {
			fields = append(fields, apperror.FieldError{Field: "resolution", Message: "la resolución es obligatoria"})
		}
		if option.RequiresAssignee {
			fields = append(fields, apperror.FieldError{Field: "assignee", Message: "el responsable es obligatorio"})
		}
		return port.TransitionOption{}, domain.NewRequiredFieldMissing(option.ID, fields...)
	}
	return option, nil
}

// rankCard stores a rank between the neighbors of the anchor card.
func (s *BoardsService) rankCard(ctx context.Context, projectID uuid.UUID, in port.MoveInput) (string, error) {
	anchorID, before := in.BeforeIssueID, true
	if in.AfterIssueID != uuid.Nil {
		anchorID, before = in.AfterIssueID, false
	}
	if anchorID == in.IssueID {
		return "", domain.ErrInvalidMove
	}
	anchor, lower, upper, err := s.repo.RankNeighbors(ctx, projectID, anchorID, in.IssueID)
	if err != nil {
		return "", err
	}
	a, b := anchor, upper
	if before {
		a, b = lower, anchor
	}
	value, err := rank.Between(a, b)
	if err != nil {
		if errors.Is(err, rank.ErrInvalid) || errors.Is(err, rank.ErrOrder) {
			return "", domain.ErrRankConflict
		}
		return "", err
	}
	if err := s.repo.SetRank(ctx, projectID, in.IssueID, value, s.clock.Now()); err != nil {
		return "", err
	}
	return value, nil
}

// laneEffects reassigns the card when it is dropped on an assignee lane, or sets its epic on an epic lane.
func (s *BoardsService) laneEffects(ctx context.Context, board *domain.Board, card *port.CardPlace, actorID uuid.UUID, globalRole string, in port.MoveInput) error {
	if in.Swimlane == "" || s.editor == nil {
		return nil
	}
	switch board.SwimlaneMode {
	case domain.SwimlanesAssignee:
		if in.Swimlane == domain.LaneUnassigned {
			if card.AssigneeID != nil {
				return s.editor.SetAssignee(ctx, actorID, globalRole, in.IssueID, nil)
			}
			return nil
		}
		lane, err := uuid.Parse(in.Swimlane)
		if err != nil {
			return domain.ErrInvalidMove
		}
		if card.AssigneeID == nil || *card.AssigneeID != lane {
			return s.editor.SetAssignee(ctx, actorID, globalRole, in.IssueID, &lane)
		}
	case domain.SwimlanesEpic:
		lane, err := uuid.Parse(in.Swimlane)
		if err != nil {
			if in.Swimlane == domain.LaneNoEpic {
				return nil
			}
			return domain.ErrInvalidMove
		}
		if card.EpicID == nil || *card.EpicID != lane {
			return s.editor.SetEpic(ctx, actorID, globalRole, in.IssueID, lane)
		}
	}
	return nil
}

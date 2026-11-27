package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	projectdomain "github.com/jhonsferg/corelog/internal/project/domain"
)

// v2Repo is an in-memory BoardRepository.
type v2Repo struct {
	boards   map[uuid.UUID]*domain.Board
	statuses []port.WorkflowStatus
	cards    []domain.Card
	places   map[uuid.UUID]*port.CardPlace
	ranks    map[uuid.UUID]string
	lastCard port.CardQuery
	sprint   *port.SprintInfo
	replaced []domain.ColumnSpec
	count    int
}

func newV2Repo() *v2Repo {
	return &v2Repo{boards: map[uuid.UUID]*domain.Board{}, places: map[uuid.UUID]*port.CardPlace{}, ranks: map[uuid.UUID]string{}}
}

func (r *v2Repo) ListByProject(_ context.Context, projectID uuid.UUID) ([]domain.Board, error) {
	var out []domain.Board
	for _, b := range r.boards {
		if b.ProjectID == projectID {
			out = append(out, *b)
		}
	}
	return out, nil
}
func (r *v2Repo) FindByID(_ context.Context, id uuid.UUID) (*domain.Board, error) {
	b, ok := r.boards[id]
	if !ok {
		return nil, domain.ErrBoardNotFound
	}
	copyOf := *b
	return &copyOf, nil
}
func (r *v2Repo) Create(_ context.Context, b *domain.Board) error {
	b.ID = uuid.New()
	for i := range b.Columns {
		b.Columns[i].ID = uuid.New()
	}
	r.boards[b.ID] = b
	return nil
}
func (r *v2Repo) UpdateSettings(_ context.Context, b *domain.Board) error {
	r.boards[b.ID] = b
	return nil
}
func (r *v2Repo) Delete(_ context.Context, id uuid.UUID) error { delete(r.boards, id); return nil }
func (r *v2Repo) ReplaceColumns(_ context.Context, _ uuid.UUID, specs []domain.ColumnSpec, _ time.Time) error {
	r.replaced = specs
	return nil
}
func (r *v2Repo) ReplaceQuickFilters(_ context.Context, id uuid.UUID, f []domain.QuickFilter) error {
	r.boards[id].QuickFilters = f
	return nil
}
func (r *v2Repo) ReplaceSwimlanes(_ context.Context, id uuid.UUID, l []domain.Swimlane) error {
	r.boards[id].Swimlanes = l
	return nil
}
func (r *v2Repo) WorkflowStatuses(context.Context, uuid.UUID) ([]port.WorkflowStatus, error) {
	return r.statuses, nil
}
func (r *v2Repo) Cards(_ context.Context, q port.CardQuery) ([]domain.Card, *port.SprintInfo, error) {
	r.lastCard = q
	return r.cards, r.sprint, nil
}
func (r *v2Repo) CardPlace(_ context.Context, _, issueID uuid.UUID) (*port.CardPlace, error) {
	p, ok := r.places[issueID]
	if !ok {
		return nil, domain.ErrCardNotFound
	}
	copyOf := *p
	return &copyOf, nil
}
func (r *v2Repo) RankNeighbors(_ context.Context, _, anchorID, _ uuid.UUID) (string, string, string, error) {
	switch anchorID {
	case v2RankB:
		return "m", "g", "t", nil
	case v2RankFirst:
		return "g", "", "m", nil
	}
	return "", "", "", domain.ErrCardNotFound
}
func (r *v2Repo) SetRank(_ context.Context, _, issueID uuid.UUID, rank string, _ time.Time) error {
	r.ranks[issueID] = rank
	return nil
}
func (r *v2Repo) CountInStatuses(context.Context, uuid.UUID, []uuid.UUID) (int, error) {
	return r.count, nil
}

var (
	v2RankB     = uuid.New()
	v2RankFirst = uuid.New()
)

// roleAuthorizer applies the project matrix by actor: lead manages and moves, contributor moves, viewer reads.
type v2Roles struct {
	lead, contributor, viewer, outsider uuid.UUID
}

func (r v2Roles) Get(_ context.Context, actor uuid.UUID, _ string, _ uuid.UUID) (*projectdomain.Project, error) {
	if actor == r.outsider {
		return nil, apperror.WithCode(apperror.KindNotFound, "project.not_found", "proyecto no encontrado")
	}
	return &projectdomain.Project{Type: projectdomain.ProjectTypeKanban}, nil
}

func (r v2Roles) Authorize(_ context.Context, action string, actor uuid.UUID, _ string, _ uuid.UUID) error {
	allowed := (action == port.ActionManageBoard && actor == r.lead) ||
		(action == port.ActionMoveCards && (actor == r.lead || actor == r.contributor))
	if !allowed {
		return apperror.WithCode(apperror.KindForbidden, "project.forbidden", "sin permiso")
	}
	return nil
}

type v2Transitions struct {
	options []port.TransitionOption
	ran     []string
	input   port.TransitionInput
	expect  uuid.UUID
	repo    *v2Repo
	toState uuid.UUID
}

func (t *v2Transitions) Options(context.Context, uuid.UUID, string, uuid.UUID) ([]port.TransitionOption, error) {
	return t.options, nil
}
func (t *v2Transitions) Run(_ context.Context, _ uuid.UUID, _ string, issueID uuid.UUID, id string, expected uuid.UUID, in port.TransitionInput) error {
	t.ran, t.input, t.expect = append(t.ran, id), in, expected
	for _, o := range t.options {
		if o.ID == id {
			t.repo.places[issueID].StatusID = o.ToStatusID
		}
	}
	return nil
}

type v2Editor struct {
	assigned []*uuid.UUID
	epics    []uuid.UUID
}

func (e *v2Editor) SetAssignee(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, a *uuid.UUID) error {
	e.assigned = append(e.assigned, a)
	return nil
}
func (e *v2Editor) SetEpic(_ context.Context, _ uuid.UUID, _ string, _, epic uuid.UUID) error {
	e.epics = append(e.epics, epic)
	return nil
}

type v2Fixture struct {
	svc         *BoardsService
	repo        *v2Repo
	trans       *v2Transitions
	editor      *v2Editor
	roles       v2Roles
	board       *domain.Board
	issue       uuid.UUID
	todo, doing uuid.UUID
	review      uuid.UUID
	done        uuid.UUID
	colTodo     uuid.UUID
	colDoing    uuid.UUID
	colDone     uuid.UUID
}

func newV2Fixture() *v2Fixture {
	f := &v2Fixture{repo: newV2Repo(), editor: &v2Editor{}, issue: uuid.New(),
		roles: v2Roles{lead: uuid.New(), contributor: uuid.New(), viewer: uuid.New(), outsider: uuid.New()},
		todo:  uuid.New(), doing: uuid.New(), review: uuid.New(), done: uuid.New()}
	f.repo.statuses = []port.WorkflowStatus{
		{ID: f.todo, Name: "Por hacer", Category: "todo"}, {ID: f.doing, Name: "En curso", Category: "in_progress"},
		{ID: f.review, Name: "En revisión", Category: "in_progress"}, {ID: f.done, Name: "Hecho", Category: "done"},
	}
	project := uuid.New()
	f.board = &domain.Board{ID: uuid.New(), ProjectID: project, Name: "Tablero", Type: domain.TypeScrum, IsDefault: true,
		SwimlaneMode: domain.SwimlanesNone, Estimation: domain.EstimationStoryPoints,
		Columns: []domain.Column{
			{ID: uuid.New(), Name: "Por hacer", StatusIDs: []uuid.UUID{f.todo}},
			{ID: uuid.New(), Name: "En curso", StatusIDs: []uuid.UUID{f.doing, f.review}, WIPLimit: intPtr(2)},
			{ID: uuid.New(), Name: "Hecho", StatusIDs: []uuid.UUID{f.done}},
		}}
	f.colTodo, f.colDoing, f.colDone = f.board.Columns[0].ID, f.board.Columns[1].ID, f.board.Columns[2].ID
	f.repo.boards[f.board.ID] = f.board
	f.repo.places[f.issue] = &port.CardPlace{StatusID: f.todo, Rank: "p"}
	f.trans = &v2Transitions{repo: f.repo}
	f.svc = NewBoardsService(f.repo, f.roles, f.roles, f.trans, f.editor, MinimalQueryFilter{}, clock.NewFixed(time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)))
	return f
}

func intPtr(n int) *int { return &n }

func (f *v2Fixture) move(in port.MoveInput) (*port.MoveResult, error) {
	return f.svc.MoveCard(context.Background(), f.board.ID, f.roles.contributor, "user", in)
}

func requireCode(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	if err == nil || apperror.KindOf(err) != kind || apperror.CodeOf(err) != code {
		t.Fatalf("error = %v (%s/%s), want %s/%s", err, apperror.KindOf(err), apperror.CodeOf(err), kind, code)
	}
}

func TestV2MoveSingleTransition(t *testing.T) {
	f := newV2Fixture()
	f.trans.options = []port.TransitionOption{{ID: "start", Name: "Iniciar", ToStatusID: f.doing}, {ID: "complete", Name: "Completar", ToStatusID: f.done}}
	f.repo.count = 3
	res, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colDoing})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.trans.ran) != 1 || f.trans.ran[0] != "start" || f.trans.expect != f.todo || !res.Transitioned || res.Status.Name != "En curso" {
		t.Fatalf("ran=%v expect=%s result=%+v", f.trans.ran, f.trans.expect, res)
	}
	if !res.WIP.Exceeded || res.WIP.State != domain.WIPOverMax || res.WIP.Count != 3 || *res.WIP.Max != 2 {
		t.Fatalf("wip advisory flag = %+v", res.WIP)
	}
}

func TestV2MoveWithinTheSameColumnOnlyReranks(t *testing.T) {
	f := newV2Fixture()
	res, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, AfterIssueID: v2RankB})
	if err != nil || res.Transitioned || len(f.trans.ran) != 0 {
		t.Fatalf("same-column move: %v %+v ran=%v", err, res, f.trans.ran)
	}
	if got := f.repo.ranks[f.issue]; !(got > "m" && got < "t") || got != res.Rank {
		t.Fatalf("rank after anchor = %q, want between m and t", got)
	}
}

func TestV2MoveRankNeighbors(t *testing.T) {
	f := newV2Fixture()
	if _, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, BeforeIssueID: v2RankB}); err != nil {
		t.Fatal(err)
	}
	if got := f.repo.ranks[f.issue]; !(got > "g" && got < "m") {
		t.Fatalf("rank before anchor = %q, want between g and m", got)
	}
	if _, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, BeforeIssueID: v2RankFirst}); err != nil {
		t.Fatal(err)
	}
	if got := f.repo.ranks[f.issue]; !(got < "g") {
		t.Fatalf("rank before the first card = %q, want below g", got)
	}
	_, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, BeforeIssueID: uuid.New()})
	requireCode(t, err, apperror.KindNotFound, "board.card_not_found")
	_, err = f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, BeforeIssueID: v2RankB, AfterIssueID: v2RankFirst})
	requireCode(t, err, apperror.KindValidation, "board.invalid_move")
	_, err = f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, BeforeIssueID: f.issue})
	requireCode(t, err, apperror.KindValidation, "board.invalid_move")
}

func TestV2MoveAmbiguousTransition(t *testing.T) {
	f := newV2Fixture()
	f.trans.options = []port.TransitionOption{{ID: "start", Name: "Iniciar", ToStatusID: f.doing}, {ID: "review", Name: "Revisar", ToStatusID: f.review}}
	_, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colDoing})
	requireCode(t, err, apperror.KindConflict, "board.ambiguous_transition")
	details := apperror.DetailsOf(err)
	if len(details) != 2 || details[0].Field != "start" || details[1].Message != "Revisar" {
		t.Fatalf("candidates = %+v", details)
	}
	if len(f.trans.ran) != 0 {
		t.Fatal("an ambiguous move ran a transition")
	}
	// The client picks one.
	if _, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colDoing, TransitionID: "review"}); err != nil || f.trans.ran[0] != "review" {
		t.Fatalf("explicit transition: %v ran=%v", err, f.trans.ran)
	}
	// A transition that does not lead into the column is refused.
	_, err = f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colDone, TransitionID: "review"})
	requireCode(t, err, apperror.KindConflict, "issue.transition_unavailable")
}

func TestV2MoveWithoutTransition(t *testing.T) {
	f := newV2Fixture()
	f.trans.options = []port.TransitionOption{{ID: "start", ToStatusID: f.doing}}
	_, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colDone})
	requireCode(t, err, apperror.KindConflict, "issue.transition_unavailable")
	f.board.Columns[2].StatusIDs = nil
	_, err = f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colDone})
	requireCode(t, err, apperror.KindConflict, "issue.transition_unavailable")
}

func TestV2MoveNeedingInputReturnsTheFieldList(t *testing.T) {
	f := newV2Fixture()
	f.trans.options = []port.TransitionOption{{ID: "complete", Name: "Completar", ToStatusID: f.done, RequiredFields: []string{"story_points"}, RequiresComment: true}}
	_, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colDone})
	requireCode(t, err, apperror.KindValidation, "issue.required_field_missing")
	details := apperror.DetailsOf(err)
	if len(details) != 3 || details[0].Field != "transition_id" || details[0].Message != "complete" || details[1].Field != "fields.story_points" || details[2].Field != "comment" {
		t.Fatalf("details = %+v", details)
	}
	if len(f.trans.ran) != 0 {
		t.Fatal("transition ran without its input")
	}
	// With the input the transition runs and receives it.
	in := port.MoveInput{IssueID: f.issue, ColumnID: f.colDone, TransitionID: "complete",
		Input: port.TransitionInput{Fields: map[string]string{"story_points": "3"}, Comment: "listo"}}
	if _, err := f.move(in); err != nil || f.trans.input.Comment != "listo" || f.trans.input.Fields["story_points"] != "3" {
		t.Fatalf("move with input: %v %+v", err, f.trans.input)
	}
}

func TestV2MoveUnknownColumnAndCard(t *testing.T) {
	f := newV2Fixture()
	_, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: uuid.New()})
	requireCode(t, err, apperror.KindNotFound, "board.column_not_found")
	_, err = f.move(port.MoveInput{IssueID: uuid.New(), ColumnID: f.colTodo})
	requireCode(t, err, apperror.KindNotFound, "board.card_not_found")
}

func TestV2MoveSwimlaneEffects(t *testing.T) {
	f := newV2Fixture()
	lane := uuid.New()
	f.board.SwimlaneMode = domain.SwimlanesAssignee
	if _, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, Swimlane: lane.String()}); err != nil {
		t.Fatal(err)
	}
	if len(f.editor.assigned) != 1 || *f.editor.assigned[0] != lane {
		t.Fatalf("assigned = %v, want [%s]", f.editor.assigned, lane)
	}
	f.repo.places[f.issue].AssigneeID = &lane
	if _, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, Swimlane: lane.String()}); err != nil || len(f.editor.assigned) != 1 {
		t.Fatalf("same assignee reassigned: %v %v", err, f.editor.assigned)
	}
	if _, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, Swimlane: domain.LaneUnassigned}); err != nil || len(f.editor.assigned) != 2 || f.editor.assigned[1] != nil {
		t.Fatalf("unassign: %v %v", err, f.editor.assigned)
	}
	_, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, Swimlane: "nonsense"})
	requireCode(t, err, apperror.KindValidation, "board.invalid_move")

	f.board.SwimlaneMode = domain.SwimlanesEpic
	epic := uuid.New()
	if _, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, Swimlane: epic.String()}); err != nil || len(f.editor.epics) != 1 || f.editor.epics[0] != epic {
		t.Fatalf("epic lane: %v %v", err, f.editor.epics)
	}
	// Lanes of other modes and the no-epic lane have no effect.
	if _, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, Swimlane: domain.LaneNoEpic}); err != nil || len(f.editor.epics) != 1 {
		t.Fatalf("no-epic lane: %v %v", err, f.editor.epics)
	}
	f.board.SwimlaneMode = domain.SwimlanesNone
	if _, err := f.move(port.MoveInput{IssueID: f.issue, ColumnID: f.colTodo, Swimlane: lane.String()}); err != nil || len(f.editor.epics) != 1 || len(f.editor.assigned) != 2 {
		t.Fatalf("no swimlane mode must not edit the card: %v", err)
	}
}

func TestV2PermissionsPerRole(t *testing.T) {
	f := newV2Fixture()
	f.trans.options = []port.TransitionOption{{ID: "start", ToStatusID: f.doing}}
	ctx := context.Background()
	move := port.MoveInput{IssueID: f.issue, ColumnID: f.colDoing}
	settings := port.UpdateBoardInput{Name: strPtr("Nuevo")}

	for _, tc := range []struct {
		name         string
		actor        uuid.UUID
		view, manage bool
		move         bool
		hidden       bool
	}{
		{"lead", f.roles.lead, true, true, true, false},
		{"contributor", f.roles.contributor, true, false, true, false},
		{"viewer", f.roles.viewer, true, false, false, false},
		{"outsider", f.roles.outsider, false, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.repo.places[f.issue].StatusID = f.todo
			_, err := f.svc.View(ctx, f.board.ID, tc.actor, "user", port.ViewQuery{})
			if (err == nil) != tc.view {
				t.Errorf("view: %v", err)
			}
			if tc.hidden {
				requireCode(t, err, apperror.KindNotFound, "project.not_found")
			}
			_, err = f.svc.Update(ctx, f.board.ID, tc.actor, "user", settings)
			if (err == nil) != tc.manage {
				t.Errorf("settings: %v", err)
			}
			_, err = f.svc.MoveCard(ctx, f.board.ID, tc.actor, "user", move)
			if (err == nil) != tc.move {
				t.Errorf("move: %v", err)
			}
			if !tc.hidden && !tc.move {
				requireCode(t, err, apperror.KindForbidden, "project.forbidden")
			}
			if tc.view {
				detail, err := f.svc.Get(ctx, f.board.ID, tc.actor, "user")
				if err != nil || detail.CanManage != tc.manage || detail.CanMoveCards != tc.move {
					t.Errorf("capabilities = %+v %v", detail, err)
				}
			}
		})
	}
}

func strPtr(s string) *string { return &s }

func TestV2CreateBoard(t *testing.T) {
	f := newV2Fixture()
	ctx := context.Background()
	f.repo.boards = map[uuid.UUID]*domain.Board{f.board.ID: f.board}
	detail, err := f.svc.Create(ctx, f.board.ProjectID, f.roles.lead, "user", port.CreateBoardInput{Settings: domain.Settings{Name: " Kanban 2 ", ShowUnmappedLane: true}})
	if err != nil {
		t.Fatal(err)
	}
	b := detail.Board
	if b.IsDefault || b.Name != "Kanban 2" || b.Type != domain.TypeKanban || len(b.Columns) != 3 || b.Estimation != domain.EstimationStoryPoints {
		t.Fatalf("board = %+v", b)
	}
	if b.Columns[1].Name != "En curso" || len(b.Columns[1].StatusIDs) != 2 || b.Columns[2].StatusIDs[0] != f.done {
		t.Fatalf("default columns = %+v", b.Columns)
	}
	_, err = f.svc.Create(ctx, f.board.ProjectID, f.roles.lead, "user", port.CreateBoardInput{Settings: domain.Settings{Name: "kanban 2"}})
	requireCode(t, err, apperror.KindConflict, "board.name_taken")
	_, err = f.svc.Create(ctx, f.board.ProjectID, f.roles.contributor, "user", port.CreateBoardInput{Settings: domain.Settings{Name: "Otro"}})
	requireCode(t, err, apperror.KindForbidden, "project.forbidden")
	_, err = f.svc.Create(ctx, f.board.ProjectID, f.roles.lead, "user", port.CreateBoardInput{Settings: domain.Settings{Name: ""}})
	requireCode(t, err, apperror.KindValidation, "board.invalid_settings")
	_, err = f.svc.Create(ctx, f.board.ProjectID, f.roles.lead, "user", port.CreateBoardInput{Settings: domain.Settings{Name: "Mal"}, Type: "gantt"})
	requireCode(t, err, apperror.KindValidation, "board.invalid")
	_, err = f.svc.Create(ctx, f.board.ProjectID, f.roles.lead, "user", port.CreateBoardInput{Settings: domain.Settings{Name: "Dup"},
		Columns: []domain.ColumnSpec{{Name: "A", StatusIDs: []uuid.UUID{f.todo}}, {Name: "B", StatusIDs: []uuid.UUID{f.todo}}}})
	requireCode(t, err, apperror.KindValidation, "board.invalid_columns")
	for i := 0; len(f.repo.boards) < domain.MaxBoardsPerProject; i++ {
		id := uuid.New()
		f.repo.boards[id] = &domain.Board{ID: id, ProjectID: f.board.ProjectID, Name: "b" + string(rune('a'+i))}
	}
	_, err = f.svc.Create(ctx, f.board.ProjectID, f.roles.lead, "user", port.CreateBoardInput{Settings: domain.Settings{Name: "Una mas"}})
	requireCode(t, err, apperror.KindConflict, "board.too_many")
}

func TestV2ReplaceColumnsValidatesMappingAndOwnership(t *testing.T) {
	f := newV2Fixture()
	ctx := context.Background()
	good := []domain.ColumnSpec{{ID: f.colTodo, Name: "Pendiente", StatusIDs: []uuid.UUID{f.todo}}, {Name: "Resto", StatusIDs: []uuid.UUID{f.doing, f.review, f.done}}}
	if _, err := f.svc.ReplaceColumns(ctx, f.board.ID, f.roles.lead, "user", good); err != nil || len(f.repo.replaced) != 2 {
		t.Fatalf("replace: %v", err)
	}
	for name, specs := range map[string][]domain.ColumnSpec{
		"none":           nil,
		"twice":          {{Name: "A", StatusIDs: []uuid.UUID{f.todo}}, {Name: "B", StatusIDs: []uuid.UUID{f.todo}}},
		"foreign status": {{Name: "A", StatusIDs: []uuid.UUID{uuid.New()}}},
		"foreign column": {{ID: uuid.New(), Name: "A"}},
	} {
		_, err := f.svc.ReplaceColumns(ctx, f.board.ID, f.roles.lead, "user", specs)
		requireCode(t, err, apperror.KindValidation, "board.invalid_columns")
		_ = name
	}
	_, err := f.svc.ReplaceColumns(ctx, f.board.ID, f.roles.contributor, "user", good)
	requireCode(t, err, apperror.KindForbidden, "project.forbidden")
}

func TestV2QuickFiltersSwimlanesDeleteAndView(t *testing.T) {
	f := newV2Fixture()
	ctx := context.Background()
	_, err := f.svc.ReplaceQuickFilters(ctx, f.board.ID, f.roles.lead, "user", []port.QueryInput{{Name: "Mal", Query: "nope = 1"}})
	requireCode(t, err, apperror.KindValidation, "board.invalid_query")
	detail, err := f.svc.ReplaceQuickFilters(ctx, f.board.ID, f.roles.lead, "user", []port.QueryInput{{Name: "Mías", Query: "assignee = currentUser()"}})
	if err != nil || len(detail.Board.QuickFilters) != 1 {
		t.Fatalf("quick filters: %v", err)
	}
	filterID := uuid.New()
	f.board.QuickFilters = []domain.QuickFilter{{ID: filterID, Name: "Mías", Query: "assignee = currentUser()"}}
	me := f.roles.contributor
	f.repo.cards = []domain.Card{
		{ID: uuid.New(), StatusID: f.todo, AssigneeID: &me}, {ID: uuid.New(), StatusID: f.todo}, {ID: uuid.New(), StatusID: f.doing},
	}
	view, err := f.svc.View(ctx, f.board.ID, me, "user", port.ViewQuery{QuickFilterIDs: []uuid.UUID{filterID}})
	if err != nil {
		t.Fatal(err)
	}
	if view.View.Columns[0].Count != 1 || view.View.Columns[0].Total != 2 || view.View.Columns[1].Count != 0 {
		t.Fatalf("filtered view = %+v", view.View.Columns)
	}
	// Scrum boards default to the active sprint, and the card limit is bounded.
	if f.repo.lastCard.Sprint.Mode != "active" || f.repo.lastCard.Limit != maxViewCards || !f.repo.lastCard.Now.Equal(time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)) {
		t.Fatalf("card query = %+v", f.repo.lastCard)
	}
	f.board.Type = domain.TypeKanban
	if _, err := f.svc.View(ctx, f.board.ID, me, "user", port.ViewQuery{}); err != nil || f.repo.lastCard.Sprint.Mode != "all" {
		t.Fatalf("kanban default sprint mode = %q (%v)", f.repo.lastCard.Sprint.Mode, err)
	}
	_, err = f.svc.View(ctx, f.board.ID, me, "user", port.ViewQuery{QuickFilterIDs: []uuid.UUID{uuid.New()}})
	requireCode(t, err, apperror.KindValidation, "board.invalid_query")

	lanes, err := f.svc.ReplaceSwimlanes(ctx, f.board.ID, f.roles.lead, "user", []port.QueryInput{{Name: "Bugs", Query: "type = bug"}})
	if err != nil || len(lanes.Board.Swimlanes) != 1 {
		t.Fatalf("swimlanes: %v", err)
	}

	// The default board cannot be deleted; another one can, but only by a manager.
	requireDefault := f.svc.Delete(ctx, f.board.ID, f.roles.lead, "user")
	requireCode(t, requireDefault, apperror.KindConflict, "board.default_not_deletable")
	extra := &domain.Board{ID: uuid.New(), ProjectID: f.board.ProjectID, Name: "Extra"}
	f.repo.boards[extra.ID] = extra
	requireCode(t, f.svc.Delete(ctx, extra.ID, f.roles.contributor, "user"), apperror.KindForbidden, "project.forbidden")
	if err := f.svc.Delete(ctx, extra.ID, f.roles.lead, "user"); err != nil || f.repo.boards[extra.ID] != nil {
		t.Fatalf("delete extra: %v", err)
	}
	_, err = f.svc.Get(ctx, uuid.New(), f.roles.lead, "user")
	requireCode(t, err, apperror.KindNotFound, "board.not_found")
}

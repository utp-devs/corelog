package postgres

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

const defaultOrgID = "00000000-0000-4000-8000-000000000002"

// countingDB counts the statements the repository sends.
type countingDB struct {
	*sqlx.DB
	n *atomic.Int32
}

func (c countingDB) QueryxContext(ctx context.Context, query string, args ...any) (*sqlx.Rows, error) {
	c.n.Add(1)
	return c.DB.QueryxContext(ctx, query, args...)
}

func (c countingDB) QueryRowxContext(ctx context.Context, query string, args ...any) *sqlx.Row {
	c.n.Add(1)
	return c.DB.QueryRowxContext(ctx, query, args...)
}

func (c countingDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	c.n.Add(1)
	return c.DB.QueryContext(ctx, query, args...)
}

type v2Fixture struct {
	t        *testing.T
	db       *sqlx.DB
	repo     *BoardRepository
	counter  *atomic.Int32
	project  uuid.UUID
	user     uuid.UUID
	statuses map[string]port.WorkflowStatus
	board    *domain.Board
}

func (f *v2Fixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.db.Exec(query, args...); err != nil {
		f.t.Fatalf("%v\n%s", err, query)
	}
}

func newV2Fixture(t *testing.T) *v2Fixture {
	t.Helper()
	db := testdb.New(t)
	counter := &atomic.Int32{}
	repo := NewBoardRepository(db)
	repo.q = countingDB{DB: db, n: counter}
	f := &v2Fixture{t: t, db: db, repo: repo, counter: counter, project: uuid.New(), user: uuid.New(), statuses: map[string]port.WorkflowStatus{}}
	f.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'Ada', 'Ada L', 'ada@example.test', 'x', 'user')`, f.user)
	f.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'BRD', 'Board', 'kanban', 'scrum', $3, 'private')`, f.project, defaultOrgID, f.user)
	f.exec(`SELECT corelog_ensure_workflow($1)`, f.project)
	statuses, err := repo.WorkflowStatuses(context.Background(), f.project)
	if err != nil || len(statuses) < 4 {
		t.Fatalf("workflow statuses: %v %v", statuses, err)
	}
	for _, s := range statuses {
		f.statuses[s.Name] = s
	}
	now := time.Now().UTC()
	board := &domain.Board{
		ProjectID: f.project, Name: "Tablero", Type: domain.TypeKanban, IsDefault: true, SwimlaneMode: domain.SwimlanesNone,
		Estimation: domain.EstimationStoryPoints, CardFields: []string{"key"}, ShowUnmappedLane: true, CreatedAt: now, UpdatedAt: now,
		Columns: []domain.Column{
			{Name: "Por hacer", StatusIDs: []uuid.UUID{f.statuses["Por hacer"].ID}},
			{Name: "En curso", StatusIDs: []uuid.UUID{f.statuses["En curso"].ID, f.statuses["En revisión"].ID}},
		},
	}
	if err := repo.Create(context.Background(), board); err != nil {
		t.Fatalf("create board: %v", err)
	}
	f.board = board
	return f
}

// issue inserts an issue in the status and returns its id.
func (f *v2Fixture) issue(summary, status string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, key_number, issue_type_id, priority_id, status_id)
		SELECT $1, $2, $3, '', 'open', 'medium', $4,
		       (SELECT COALESCE(max(key_number), 0) + 1 FROM issues WHERE project_id = $2),
		       (SELECT it.id FROM issue_types it WHERE it.organization_id = $5 AND it.is_default),
		       (SELECT pr.id FROM priorities pr WHERE pr.organization_id = $5 AND pr.is_default), $6`,
		id, f.project, summary, f.user, defaultOrgID, f.statuses[status].ID)
	return id
}

func TestBoardsV2RepositoryViewReadsAreBoundedPostgres(t *testing.T) {
	f := newV2Fixture(t)
	ctx := context.Background()

	measure := func() (int32, []domain.Card) {
		f.counter.Store(0)
		board, err := f.repo.FindByID(ctx, f.board.ID)
		if err != nil || len(board.Columns) != 2 {
			t.Fatalf("find board: %v %+v", err, board)
		}
		cards, _, err := f.repo.Cards(ctx, port.CardQuery{ProjectID: f.project, Sprint: port.SprintSelector{Mode: "all"}, Now: time.Now(), Limit: 5000})
		if err != nil {
			t.Fatal(err)
		}
		return f.counter.Load(), cards
	}

	f.issue("uno", "Por hacer")
	small, cards := measure()
	if len(cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(cards))
	}
	label := uuid.New()
	f.exec(`INSERT INTO labels (id, project_id, name) VALUES ($1, $2, 'backend')`, label, f.project)
	for i := 0; i < 60; i++ {
		id := f.issue("carga", "En curso")
		f.exec(`INSERT INTO issue_labels (issue_id, label_id) VALUES ($1, $2)`, id, label)
		f.exec(`UPDATE issues SET assignee_id = $2 WHERE id = $1`, id, f.user)
	}
	large, cards := measure()
	if len(cards) != 61 {
		t.Fatalf("cards = %d, want 61", len(cards))
	}
	if small != large {
		t.Fatalf("statements grew with the cards: %d with 1 card, %d with 61", small, large)
	}
	if large > 6 {
		t.Fatalf("view reads = %d statements, want at most 6", large)
	}
	var withLabels int
	for _, c := range cards {
		if len(c.Labels) == 1 && c.Labels[0] == "backend" && c.AssigneeID != nil && c.AssigneeName == "Ada L" && c.Key != "" {
			withLabels++
		}
	}
	if withLabels != 60 {
		t.Fatalf("cards with labels and assignee = %d, want 60", withLabels)
	}
}

func TestBoardsV2RepositoryCardsEpicsSubtasksAndSoftDeletePostgres(t *testing.T) {
	f := newV2Fixture(t)
	ctx := context.Background()
	story := f.issue("historia", "Por hacer")
	epic := f.issue("epica", "Por hacer")
	f.exec(`UPDATE issues SET issue_type_id = (SELECT id FROM issue_types WHERE organization_id = $2 AND hierarchy_level = 1) WHERE id = $1`, epic, defaultOrgID)
	f.exec(`UPDATE issues SET parent_id = $2 WHERE id = $1`, story, epic)
	sub1 := f.issue("sub 1", "Por hacer")
	sub2 := f.issue("sub 2", "Por hacer")
	f.exec(`UPDATE issues SET parent_id = $2 WHERE id IN ($1, $3)`, sub1, story, sub2)
	f.exec(`UPDATE issues SET status_id = $2 WHERE id = $1`, sub2, f.statuses["Hecho"].ID)
	gone := f.issue("borrada", "Por hacer")
	f.exec(`UPDATE issues SET deleted_at = now() WHERE id = $1`, gone)

	cards, _, err := f.repo.Cards(ctx, port.CardQuery{ProjectID: f.project, Sprint: port.SprintSelector{Mode: "all"}, Now: time.Now(), Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[uuid.UUID]domain.Card{}
	for _, c := range cards {
		byID[c.ID] = c
	}
	if _, ok := byID[epic]; ok {
		t.Error("an epic must not be a card")
	}
	if _, ok := byID[gone]; ok {
		t.Error("a soft-deleted issue must not be a card")
	}
	got := byID[story]
	if got.Epic == nil || got.Epic.ID != epic || got.SubtaskTotal != 2 || got.SubtaskDone != 1 {
		t.Errorf("story card = epic %+v subtasks %d/%d, want epic %s and 1/2", got.Epic, got.SubtaskDone, got.SubtaskTotal, epic)
	}
	if s := byID[sub1]; s.Parent == nil || s.Parent.ID != story || s.Epic == nil || s.Epic.ID != epic {
		t.Errorf("subtask card parent=%+v epic=%+v, want story and inherited epic", s.Parent, s.Epic)
	}
	place, err := f.repo.CardPlace(ctx, f.project, story)
	if err != nil || place.EpicID == nil || *place.EpicID != epic {
		t.Errorf("card place = %+v %v", place, err)
	}
	if _, err := f.repo.CardPlace(ctx, uuid.New(), story); !errors.Is(err, domain.ErrCardNotFound) {
		t.Errorf("card of another project: %v, want ErrCardNotFound", err)
	}
	if _, err := f.repo.CardPlace(ctx, f.project, gone); !errors.Is(err, domain.ErrCardNotFound) {
		t.Errorf("deleted card: %v, want ErrCardNotFound", err)
	}
}

func TestBoardsV2RepositoryReplaceColumnsAndRankPostgres(t *testing.T) {
	f := newV2Fixture(t)
	ctx := context.Background()
	todo, doing := f.board.Columns[0], f.board.Columns[1]

	// Swap the order, keep one column, drop none, add one: positions swap without violating the unique key.
	specs := []domain.ColumnSpec{
		{ID: doing.ID, Name: "En curso", StatusIDs: doing.StatusIDs, Color: "blue"},
		{ID: todo.ID, Name: "Pendiente", StatusIDs: todo.StatusIDs},
		{Name: "Hecho", StatusIDs: []uuid.UUID{f.statuses["Hecho"].ID}, MaxIssues: ptr(3), MinIssues: ptr(1)},
	}
	if err := f.repo.ReplaceColumns(ctx, f.board.ID, specs, time.Now().UTC()); err != nil {
		t.Fatalf("replace columns: %v", err)
	}
	board, err := f.repo.FindByID(ctx, f.board.ID)
	if err != nil || len(board.Columns) != 3 {
		t.Fatalf("board after replace: %v %+v", err, board)
	}
	if board.Columns[0].ID != doing.ID || board.Columns[1].ID != todo.ID || board.Columns[1].Name != "Pendiente" || board.Columns[0].Color != "blue" {
		t.Errorf("columns after replace = %+v", board.Columns)
	}
	if last := board.Columns[2]; last.WIPLimit == nil || *last.WIPLimit != 3 || last.MinIssues == nil || *last.MinIssues != 1 || len(last.StatusIDs) != 1 {
		t.Errorf("new column = %+v", last)
	}
	// Dropping the new column deletes it and its bindings.
	if err := f.repo.ReplaceColumns(ctx, f.board.ID, specs[:2], time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var bindings int
	if err := f.db.Get(&bindings, `SELECT count(*) FROM board_column_statuses WHERE status_id = $1`, f.statuses["Hecho"].ID); err != nil || bindings != 0 {
		t.Errorf("bindings of the dropped column = %d %v, want 0", bindings, err)
	}
	// A column of another board cannot be updated through this board.
	other := &domain.Board{ProjectID: f.project, Name: "Otro", Type: domain.TypeKanban, SwimlaneMode: domain.SwimlanesNone, Estimation: domain.EstimationIssueCount,
		CardFields: []string{}, CreatedAt: time.Now().UTC(), Columns: []domain.Column{{Name: "A"}}}
	if err := f.repo.Create(ctx, other); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.ReplaceColumns(ctx, other.ID, []domain.ColumnSpec{{ID: todo.ID, Name: "Robada"}}, time.Now().UTC()); !errors.Is(err, domain.ErrColumnNotFound) {
		t.Errorf("foreign column replace: %v, want ErrColumnNotFound", err)
	}
	dup := &domain.Board{ProjectID: f.project, Name: "tablero", Type: domain.TypeKanban, SwimlaneMode: domain.SwimlanesNone, Estimation: domain.EstimationIssueCount,
		CardFields: []string{}, CreatedAt: time.Now().UTC(), Columns: []domain.Column{{Name: "A"}}}
	if err := f.repo.Create(ctx, dup); !errors.Is(err, domain.ErrBoardNameTaken) {
		t.Errorf("duplicate name: %v, want ErrBoardNameTaken", err)
	}

	// Quick filters and swimlanes round trip.
	if err := f.repo.ReplaceQuickFilters(ctx, f.board.ID, []domain.QuickFilter{{Name: "Mías", Query: "assignee = currentUser()"}, {Name: "Altas", Query: "priority in (high)"}}); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.ReplaceSwimlanes(ctx, f.board.ID, []domain.Swimlane{{Name: "Bugs", Query: "type = bug"}}); err != nil {
		t.Fatal(err)
	}
	board, _ = f.repo.FindByID(ctx, f.board.ID)
	if len(board.QuickFilters) != 2 || board.QuickFilters[1].Name != "Altas" || len(board.Swimlanes) != 1 {
		t.Errorf("filters=%+v lanes=%+v", board.QuickFilters, board.Swimlanes)
	}

	// Rank neighbors ignore the moved issue and soft-deleted rows.
	a, b, c := f.issue("a", "Por hacer"), f.issue("b", "Por hacer"), f.issue("c", "Por hacer")
	f.exec(`UPDATE issues SET rank = 'g' WHERE id = $1`, a)
	f.exec(`UPDATE issues SET rank = 'm' WHERE id = $1`, b)
	f.exec(`UPDATE issues SET rank = 't' WHERE id = $1`, c)
	anchor, lower, upper, err := f.repo.RankNeighbors(ctx, f.project, b, c)
	if err != nil || anchor != "m" || lower != "g" || upper != "" {
		t.Errorf("neighbors of b ignoring c = %q %q %q %v, want m g empty", anchor, lower, upper, err)
	}
	if _, _, _, err := f.repo.RankNeighbors(ctx, uuid.New(), b, c); !errors.Is(err, domain.ErrCardNotFound) {
		t.Errorf("neighbors in another project: %v", err)
	}
	if err := f.repo.SetRank(ctx, f.project, c, "h", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if n, err := f.repo.CountInStatuses(ctx, f.project, []uuid.UUID{f.statuses["Por hacer"].ID}); err != nil || n != 3 {
		t.Errorf("count = %d %v, want 3", n, err)
	}
}

func ptr(n int) *int { return &n }

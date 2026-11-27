package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"database/sql"
	"errors"
	"github.com/jhonsferg/corelog/internal/board/domain"
)

func TestBoardRepositoryFindDefaultByProjectPostgres(t *testing.T) {
	dsn := os.Getenv("CORELOG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CORELOG_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	schema := "board_read_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE users (id UUID PRIMARY KEY, name VARCHAR(120) NOT NULL)`,
		`CREATE TABLE projects (id UUID PRIMARY KEY, deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE boards (id UUID PRIMARY KEY, project_id UUID NOT NULL, name VARCHAR(80) NOT NULL, type VARCHAR(10) NOT NULL, is_default BOOLEAN NOT NULL)`,
		`CREATE TABLE board_columns (id UUID PRIMARY KEY, board_id UUID NOT NULL, name VARCHAR(80) NOT NULL, status VARCHAR(20) NOT NULL, position INTEGER NOT NULL, wip_limit INTEGER)`,
		`CREATE TABLE issues (id UUID PRIMARY KEY, project_id UUID NOT NULL, issue_type VARCHAR(20) NOT NULL, summary VARCHAR(200) NOT NULL, status VARCHAR(20) NOT NULL, priority VARCHAR(20) NOT NULL, assignee_id UUID, story_points NUMERIC(4,1), rank VARCHAR(64) NOT NULL, created_at TIMESTAMPTZ NOT NULL, deleted_at TIMESTAMPTZ, status_id UUID)`,
		`CREATE TABLE board_column_statuses (column_id UUID NOT NULL, status_id UUID NOT NULL, PRIMARY KEY (column_id, status_id))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	projectID, boardID, openColumnID, progressColumnID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	issueID, assigneeID := uuid.New(), uuid.New()
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users (id, name) VALUES ($1, 'Ada Lovelace')`, []any{assigneeID}},
		{`INSERT INTO projects (id) VALUES ($1)`, []any{projectID}},
		{`INSERT INTO boards (id, project_id, name, type, is_default) VALUES ($1, $2, 'Default', 'kanban', true)`, []any{boardID, projectID}},
		{`INSERT INTO board_columns (id, board_id, name, status, position) VALUES ($1, $2, 'Open', 'open', 0), ($3, $2, 'In progress', 'in_progress', 1)`, []any{openColumnID, boardID, progressColumnID}},
		{`INSERT INTO issues (id, project_id, issue_type, summary, status, priority, assignee_id, story_points, rank, created_at) VALUES ($1, $2, 'story', 'Board read card', 'open', 'high', $3, 5, 'm', now())`, []any{issueID, projectID, assigneeID}},
	} {
		if _, err := db.Exec(statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}

	repository := NewBoardRepository(db)
	board, err := repository.FindDefaultByProject(context.Background(), projectID)
	if err != nil {
		t.Fatal(err)
	}
	if board.ID != boardID || len(board.Columns) != 2 || board.Columns[0].ID != openColumnID || board.Columns[1].ID != progressColumnID {
		t.Fatalf("board columns were not returned in their configured order: %#v", board)
	}
	if len(board.Columns[0].Cards) != 1 || len(board.Columns[1].Cards) != 0 {
		t.Fatalf("cards were not grouped by status: %#v", board.Columns)
	}
	card := board.Columns[0].Cards[0]
	if card.ID != issueID || card.AssigneeID == nil || *card.AssigneeID != assigneeID || card.Assignee == nil || *card.Assignee != "Ada Lovelace" || card.StoryPoints == nil || *card.StoryPoints != 5 {
		t.Fatalf("card details were not loaded: %#v", card)
	}
	if _, err := repository.FindDefaultByProject(context.Background(), uuid.New()); err != domain.ErrBoardNotFound {
		t.Fatalf("missing board error = %v, want %v", err, domain.ErrBoardNotFound)
	}

	// A column bound to statuses shows the issues in those statuses, whatever their legacy value; a column
	// without bindings still shows the issues by their legacy value.
	mappedStatus, mappedIssue := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO board_column_statuses (column_id, status_id) VALUES ($1, $2)`, progressColumnID, mappedStatus); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, issue_type, summary, status, priority, rank, created_at, status_id) VALUES ($1, $2, 'task', 'Mapped card', 'open', 'low', 'n', now(), $3)`, mappedIssue, projectID, mappedStatus); err != nil {
		t.Fatal(err)
	}
	board, err = repository.FindDefaultByProject(context.Background(), projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Columns[1].Cards) != 1 || board.Columns[1].Cards[0].ID != mappedIssue || len(board.Columns[0].Cards) != 2 {
		t.Fatalf("column bound to a status must show its issues: open=%d progress=%d", len(board.Columns[0].Cards), len(board.Columns[1].Cards))
	}
	if _, err := db.Exec(`DELETE FROM issues WHERE id = $1`, mappedIssue); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM board_column_statuses`); err != nil {
		t.Fatal(err)
	}

	// A soft-deleted issue is no longer a card, while the live one stays.
	deletedIssue := uuid.New()
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, issue_type, summary, status, priority, rank, created_at, deleted_at) VALUES ($1, $2, 'task', 'Gone card', 'open', 'low', 'z', now(), now())`, deletedIssue, projectID); err != nil {
		t.Fatal(err)
	}
	board, err = repository.FindDefaultByProject(context.Background(), projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Columns[0].Cards) != 1 || board.Columns[0].Cards[0].ID != issueID {
		t.Fatalf("soft-deleted issue must not appear as a card: %#v", board.Columns[0].Cards)
	}

	// A soft-deleted project has no readable board, and so no cards.
	if _, err := db.Exec(`UPDATE projects SET deleted_at = now() WHERE id = $1`, projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindDefaultByProject(context.Background(), projectID); err != domain.ErrBoardNotFound {
		t.Fatalf("board of deleted project error = %v, want %v", err, domain.ErrBoardNotFound)
	}
}

// TestBoardRepositoryUpdateWIPLimitPostgres comprueba que se fija y se quita el limite WIP de una columna,
// y que una columna inexistente responde no encontrada.
func TestBoardRepositoryUpdateWIPLimitPostgres(t *testing.T) {
	dsn := os.Getenv("CORELOG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CORELOG_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	schema := "board_wip_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE board_columns (id UUID PRIMARY KEY, wip_limit INTEGER, updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	column := uuid.New()
	if _, err := db.Exec(`INSERT INTO board_columns (id) VALUES ($1)`, column); err != nil {
		t.Fatal(err)
	}

	repo := NewBoardRepository(db)
	ctx := context.Background()
	three := 3
	if err := repo.UpdateWIPLimit(ctx, column, &three); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	var stored sql.NullInt64
	if err := db.Get(&stored, `SELECT wip_limit FROM board_columns WHERE id = $1`, column); err != nil || !stored.Valid || stored.Int64 != 3 {
		t.Fatalf("stored limit = %+v, %v; want 3", stored, err)
	}
	if err := repo.UpdateWIPLimit(ctx, column, nil); err != nil {
		t.Fatalf("clear limit: %v", err)
	}
	if err := db.Get(&stored, `SELECT wip_limit FROM board_columns WHERE id = $1`, column); err != nil || stored.Valid {
		t.Fatalf("limit after clear = %+v, %v; want NULL", stored, err)
	}
	if err := repo.UpdateWIPLimit(ctx, uuid.New(), &three); !errors.Is(err, domain.ErrColumnNotFound) {
		t.Fatalf("unknown column: got %v, want ErrColumnNotFound", err)
	}
}

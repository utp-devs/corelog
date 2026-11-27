package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// failingHistoryCollab falla al guardar el historial; el resto de operaciones usa el repositorio real.
type failingHistoryCollab struct {
	*CollaborationRepository
}

func (failingHistoryCollab) SaveActivity(context.Context, *domain.Activity) error {
	return apperror.New(apperror.KindInternal, "history store down")
}

type allowAllAuthorizer struct{}

func (allowAllAuthorizer) Authorize(context.Context, string, uuid.UUID, string, uuid.UUID) error {
	return nil
}

type sameTeamDirectory struct{}

func (sameTeamDirectory) SharesTeam(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func (sameTeamDirectory) NamesOf(context.Context, []uuid.UUID) (map[uuid.UUID]string, error) {
	return map[uuid.UUID]string{}, nil
}

// newAssignTxDB crea un esquema aislado con las tablas que tocan asignar y desasignar.
func newAssignTxDB(t *testing.T) *sqlx.DB {
	t.Helper()
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

	schema := "assign_tx_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE projects (id UUID PRIMARY KEY, key VARCHAR(10) NOT NULL DEFAULT 'TST', next_issue_number INT NOT NULL DEFAULT 1, deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE issues (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			project_id UUID NOT NULL,
			key_number INT NOT NULL DEFAULT 0,
			summary VARCHAR(200) NOT NULL,
			description TEXT NOT NULL,
			status VARCHAR(20) NOT NULL,
			priority VARCHAR(20) NOT NULL,
			issue_type VARCHAR(20) NOT NULL DEFAULT 'task',
			parent_id UUID,
			due_date TIMESTAMPTZ,
			story_points NUMERIC(4,1),
			rank VARCHAR(64) NOT NULL DEFAULT 'm',
			closed_at TIMESTAMPTZ,
			reporter_id UUID NOT NULL,
			assignee_id UUID,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE watchers (
			issue_id UUID NOT NULL,
			user_id UUID NOT NULL,
			PRIMARY KEY (issue_id, user_id)
		)`,
		`CREATE TABLE activities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			issue_id UUID NOT NULL,
			actor_id UUID NOT NULL,
			action VARCHAR(40) NOT NULL,
			field_name VARCHAR(40),
			old_value TEXT,
			new_value TEXT,
			created_at TIMESTAMPTZ NOT NULL
		)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// newAssignTxService arma el servicio real sobre Postgres con el TxManager real y un historial inyectable.
func newAssignTxService(db *sqlx.DB, collab port.CollaborationRepository) *usecase.IssueService {
	return usecase.NewIssueService(
		NewIssueRepository(db),
		sameTeamDirectory{},
		collab,
		NewWatcherRepository(db),
		allowAllAuthorizer{},
	).WithTxManager(database.NewTxManager(db))
}

func TestAssignAndUnassignAreAtomicWithHistoryPostgres(t *testing.T) {
	ctx := context.Background()

	t.Run("assign stores assignee, follower and history together", func(t *testing.T) {
		db := newAssignTxDB(t)
		svc := newAssignTxService(db, NewCollaborationRepository(db))
		actor, assignee, projectID := uuid.New(), uuid.New(), uuid.New()
		insertProjectRow(t, db, projectID)
		issue, err := svc.Create(ctx, port.CreateInput{Summary: "Atomica", Description: "Desc", Priority: domain.PriorityLow, ReporterID: actor, ProjectID: &projectID})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := svc.Assign(ctx, issue.ID, actor, assignee, "admin"); err != nil {
			t.Fatalf("assign: %v", err)
		}
		var storedAssignee uuid.UUID
		if err := db.GetContext(ctx, &storedAssignee, `SELECT assignee_id FROM issues WHERE id = $1`, issue.ID); err != nil {
			t.Fatal(err)
		}
		if storedAssignee != assignee {
			t.Errorf("stored assignee = %v, want %v", storedAssignee, assignee)
		}
		var watching int
		if err := db.GetContext(ctx, &watching, `SELECT count(*) FROM watchers WHERE issue_id = $1 AND user_id = $2`, issue.ID, assignee); err != nil {
			t.Fatal(err)
		}
		if watching != 1 {
			t.Errorf("assignee follows issue = %d rows, want 1", watching)
		}
		var newValue string
		var actions int
		if err := db.GetContext(ctx, &actions, `SELECT count(*) FROM activities WHERE issue_id = $1 AND action = 'assignee_changed'`, issue.ID); err != nil {
			t.Fatal(err)
		}
		if err := db.GetContext(ctx, &newValue, `SELECT new_value FROM activities WHERE issue_id = $1`, issue.ID); err != nil {
			t.Fatal(err)
		}
		if actions != 1 || newValue != assignee.String() {
			t.Errorf("history rows = %d new_value = %q, want one row with %v", actions, newValue, assignee)
		}
	})

	t.Run("failing history on assign rolls back the assignee", func(t *testing.T) {
		db := newAssignTxDB(t)
		healthy := newAssignTxService(db, NewCollaborationRepository(db))
		actor, assignee, projectID := uuid.New(), uuid.New(), uuid.New()
		insertProjectRow(t, db, projectID)
		issue, err := healthy.Create(ctx, port.CreateInput{Summary: "Rollback", Description: "Desc", Priority: domain.PriorityLow, ReporterID: actor, ProjectID: &projectID})
		if err != nil {
			t.Fatal(err)
		}

		broken := newAssignTxService(db, failingHistoryCollab{NewCollaborationRepository(db)})
		_, err = broken.Assign(ctx, issue.ID, actor, assignee, "admin")
		if err == nil {
			t.Fatal("assign succeeded although history failed")
		}
		var assigned bool
		if err := db.GetContext(ctx, &assigned, `SELECT assignee_id IS NOT NULL FROM issues WHERE id = $1`, issue.ID); err != nil {
			t.Fatal(err)
		}
		if assigned {
			t.Error("assignee was persisted despite the failed history write")
		}
		var watching int
		if err := db.GetContext(ctx, &watching, `SELECT count(*) FROM watchers WHERE issue_id = $1`, issue.ID); err != nil {
			t.Fatal(err)
		}
		if watching != 0 {
			t.Errorf("watchers = %d after rollback, want 0", watching)
		}
	})

	t.Run("failing history on unassign keeps the assignee", func(t *testing.T) {
		db := newAssignTxDB(t)
		healthy := newAssignTxService(db, NewCollaborationRepository(db))
		actor, assignee, projectID := uuid.New(), uuid.New(), uuid.New()
		insertProjectRow(t, db, projectID)
		issue, err := healthy.Create(ctx, port.CreateInput{Summary: "Unassign rollback", Description: "Desc", Priority: domain.PriorityLow, ReporterID: actor, ProjectID: &projectID})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := healthy.Assign(ctx, issue.ID, actor, assignee, "admin"); err != nil {
			t.Fatalf("assign: %v", err)
		}

		broken := newAssignTxService(db, failingHistoryCollab{NewCollaborationRepository(db)})
		_, err = broken.Unassign(ctx, issue.ID, actor, "admin")
		if err == nil {
			t.Fatal("unassign succeeded although history failed")
		}
		var stored uuid.UUID
		if err := db.GetContext(ctx, &stored, `SELECT assignee_id FROM issues WHERE id = $1`, issue.ID); err != nil {
			t.Fatal(err)
		}
		if stored != assignee {
			t.Errorf("assignee = %v after rollback, want %v", stored, assignee)
		}
	})
}

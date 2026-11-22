package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

type BoardRepository struct {
	db *sqlx.DB
}

func NewBoardRepository(db *sqlx.DB) *BoardRepository {
	return &BoardRepository{db: db}
}

var _ port.Repository = (*BoardRepository)(nil)

type boardColumnIssueRow struct {
	BoardID        uuid.UUID  `db:"board_id"`
	ProjectID      uuid.UUID  `db:"project_id"`
	BoardName      string     `db:"board_name"`
	BoardType      string     `db:"board_type"`
	ColumnID       uuid.UUID  `db:"column_id"`
	ColumnName     string     `db:"column_name"`
	ColumnStatus   string     `db:"column_status"`
	ColumnPosition int        `db:"column_position"`
	WIPLimit       *int       `db:"wip_limit"`
	IssueID        *uuid.UUID `db:"issue_id"`
	IssueType      *string    `db:"issue_type"`
	Summary        *string    `db:"summary"`
	IssueStatus    *string    `db:"issue_status"`
	Priority       *string    `db:"priority"`
	AssigneeID     *uuid.UUID `db:"assignee_id"`
	AssigneeName   *string    `db:"assignee_name"`
	StoryPoints    *float64   `db:"story_points"`
	Rank           *string    `db:"rank"`
}

func (r *BoardRepository) FindDefaultByProject(ctx context.Context, projectID uuid.UUID) (*domain.Board, error) {
	const query = `
		SELECT b.id AS board_id, b.project_id, b.name AS board_name, b.type AS board_type,
		       c.id AS column_id, c.name AS column_name, c.status AS column_status,
		       c.position AS column_position, c.wip_limit,
		       i.id AS issue_id, i.issue_type, i.title AS summary, i.status AS issue_status,
		       i.priority, i.assignee_id, assignee.name AS assignee_name,
		       i.story_points, i.rank
		FROM boards b
	JOIN board_columns c ON c.board_id = b.id
	LEFT JOIN issues i ON i.project_id = b.project_id AND i.status = c.status
	LEFT JOIN users assignee ON assignee.id = i.assignee_id
	WHERE b.project_id = $1 AND b.is_default = true
	ORDER BY c.position ASC, i.rank ASC NULLS LAST, i.created_at ASC, i.id ASC`

	var rows []boardColumnIssueRow
	if err := r.db.SelectContext(ctx, &rows, query, projectID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read project board", err)
	}
	if len(rows) == 0 {
		return nil, domain.ErrBoardNotFound
	}

	first := rows[0]
	board := &domain.Board{
		ID: first.BoardID, ProjectID: first.ProjectID, Name: first.BoardName,
		Type: domain.Type(first.BoardType), IsDefault: true,
		Columns: make([]domain.Column, 0),
	}
	columnIndexes := make(map[uuid.UUID]int)
	for _, row := range rows {
		columnIndex, ok := columnIndexes[row.ColumnID]
		if !ok {
			columnIndex = len(board.Columns)
			columnIndexes[row.ColumnID] = columnIndex
			board.Columns = append(board.Columns, domain.Column{
				ID: row.ColumnID, BoardID: board.ID, Name: row.ColumnName,
				Status: domain.IssueStatus(row.ColumnStatus), Position: row.ColumnPosition,
				WIPLimit: row.WIPLimit, Cards: make([]domain.IssueCard, 0),
			})
		}
		if row.IssueID == nil {
			continue
		}
		if row.IssueType == nil || row.Summary == nil || row.IssueStatus == nil || row.Priority == nil || row.Rank == nil {
			return nil, apperror.New(apperror.KindInternal, "incomplete issue data for project board")
		}
		board.Columns[columnIndex].Cards = append(board.Columns[columnIndex].Cards, domain.IssueCard{
			ID: *row.IssueID, Type: *row.IssueType, Summary: *row.Summary,
			Status: domain.IssueStatus(*row.IssueStatus), Priority: *row.Priority,
			AssigneeID: row.AssigneeID, Assignee: row.AssigneeName,
			StoryPoints: row.StoryPoints, Rank: *row.Rank,
		})
	}
	return board, nil
}

// UpdateWIPLimit fija o quita (nil) el limite WIP de una columna.
func (r *BoardRepository) UpdateWIPLimit(ctx context.Context, columnID uuid.UUID, limit *int) error {
	result, err := r.db.ExecContext(ctx, `UPDATE board_columns SET wip_limit = $2, updated_at = now() WHERE id = $1`, columnID, limit)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update column", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update column", err)
	}
	if affected == 0 {
		return domain.ErrColumnNotFound
	}
	return nil
}

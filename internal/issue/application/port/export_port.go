package port

import (
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// ExportBatch is one page of the export with the display data its rows need.
type ExportBatch struct {
	Issues []*domain.Issue
	Labels map[uuid.UUID][]string
	Names  map[uuid.UUID]string
}

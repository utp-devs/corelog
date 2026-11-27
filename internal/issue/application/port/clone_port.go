package port

import (
	"context"

	"github.com/google/uuid"
)

// CloneInput is the request of a clone. ProjectID Nil means the project of the original. Include names the
// options the caller asked for (attachments, links, subtasks, watchers), validated by the domain.
type CloneInput struct {
	Summary   string
	ProjectID uuid.UUID
	Include   []string
}

// CloneRepository copies the relations of an issue to its clone.
type CloneRepository interface {
	// CopyWatchers makes every watcher of from also watch to.
	CopyWatchers(ctx context.Context, fromID, toID uuid.UUID) error
	// CopyProjectScoped copies the labels and components of from to to. Only valid inside one project.
	CopyProjectScoped(ctx context.Context, fromID, toID uuid.UUID) error
	// CopyLinks copies the live links of from to to, keeping their kind and direction. Clones links are not copied:
	// the clone gets its own link back to the original.
	CopyLinks(ctx context.Context, fromID, toID, actorID uuid.UUID) error
}

// ExportLabelReader reads the label names of many issues at once for the CSV export.
type ExportLabelReader interface {
	LabelNamesByIssue(ctx context.Context, issueIDs []uuid.UUID) (map[uuid.UUID][]string, error)
}

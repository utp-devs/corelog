package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/legal/defaults"
)

// ensureLegalDocuments publishes the drafted legal documents as version 1 for every slug that has no
// document yet, so a database that was not created by the seed still serves the legal texts. It is
// idempotent and never touches a slug that already has a published version, whatever its version.
func ensureLegalDocuments(ctx context.Context, db *sqlx.DB, log *slog.Logger) error {
	for _, d := range defaults.Documents() {
		result, err := db.ExecContext(ctx, `
			INSERT INTO legal_documents (slug, version, title, body_markdown, summary, required, effective_from)
			SELECT $1, 1, $2, $3, $4, $5, now()
			WHERE NOT EXISTS (SELECT 1 FROM legal_documents WHERE slug = $1)`,
			string(d.Slug), d.Title, d.Body, d.Summary, d.Required)
		if err != nil {
			return fmt.Errorf("publish default legal document %s: %w", d.Slug, err)
		}
		if rows, _ := result.RowsAffected(); rows > 0 {
			log.Info("published default legal document", "slug", string(d.Slug), "version", 1)
		}
	}
	return nil
}

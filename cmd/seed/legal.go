package main

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/legal/defaults"
)

// seedLegalDocuments publishes the drafted legal documents as version 1 when a slug has no document yet. It is
// idempotent: documents that already exist, whatever their version, are left alone.
func seedLegalDocuments(ctx context.Context, db *sqlx.DB) error {
	for _, d := range defaults.Documents() {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO legal_documents (slug, version, title, body_markdown, summary, required, effective_from)
			SELECT $1, 1, $2, $3, $4, $5, now()
			WHERE NOT EXISTS (SELECT 1 FROM legal_documents WHERE slug = $1)`,
			string(d.Slug), d.Title, d.Body, d.Summary, d.Required); err != nil {
			return fmt.Errorf("sembrar documento legal %s: %w", d.Slug, err)
		}
	}
	return nil
}

// seedLegalConsents records that every seeded account accepted the current required legal documents, so
// demo and test accounts start without pending consents. Accounts that already accepted are skipped.
func seedLegalConsents(ctx context.Context, db *sqlx.DB) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO user_consents (user_id, document_slug, document_version, accepted_at, source)
		SELECT u.id, d.slug, d.version, now(), 'register'
		FROM users u
		CROSS JOIN LATERAL (
			SELECT DISTINCT ON (slug) slug, version FROM legal_documents
			WHERE required AND effective_from <= now() ORDER BY slug, version DESC
		) d
		WHERE NOT EXISTS (
			SELECT 1 FROM user_consents c
			WHERE c.user_id = u.id AND c.document_slug = d.slug AND c.document_version = d.version AND c.revoked_at IS NULL
		)`); err != nil {
		return fmt.Errorf("sembrar consentimientos: %w", err)
	}
	return nil
}

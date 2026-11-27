package port

import (
	"context"

	"github.com/google/uuid"
)

// CatalogSeeder creates the default catalogs of a new organization. It runs inside the transaction that
// creates the organization. The issue hexagon implements it; this hexagon only declares the need.
type CatalogSeeder interface {
	SeedDefaults(ctx context.Context, orgID uuid.UUID) error
}

package usecase

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/audit/application/port"
	"github.com/jhonsferg/corelog/internal/audit/domain"
)

// MaxExportRows caps one CSV export.
const MaxExportRows = 50000

// Service implements the read side of the audit log. There is no write path: entries come from the consumer.
type Service struct {
	repo port.Repository
	orgs port.OrgAccess
}

var _ port.Service = (*Service)(nil)

// NewService returns the audit read service.
func NewService(repo port.Repository, orgs port.OrgAccess) *Service {
	return &Service{repo: repo, orgs: orgs}
}

func (s *Service) ListPlatform(ctx context.Context, f port.ListFilter) (port.ListPage, error) {
	return s.repo.List(ctx, f)
}

func (s *Service) ListOrganization(ctx context.Context, actorID, orgID uuid.UUID, f port.ListFilter) (port.ListPage, error) {
	if err := s.orgs.RequireAdmin(ctx, actorID, orgID); err != nil {
		return port.ListPage{}, err
	}
	f.OrganizationID = &orgID // the scope always wins over any filter of the caller
	return s.repo.List(ctx, f)
}

func (s *Service) ExportPlatform(ctx context.Context, f port.ListFilter, w io.Writer) error {
	return s.export(ctx, f, w)
}

func (s *Service) ExportOrganization(ctx context.Context, actorID, orgID uuid.UUID, f port.ListFilter, w io.Writer) error {
	if err := s.orgs.RequireAdmin(ctx, actorID, orgID); err != nil {
		return err
	}
	f.OrganizationID = &orgID
	return s.export(ctx, f, w)
}

var csvHeader = []string{"id", "fecha", "actor", "accion", "entidad", "id_entidad", "resumen", "ip", "id_peticion"}

func (s *Service) export(ctx context.Context, f port.ListFilter, w io.Writer) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return fmt.Errorf("audit: write csv header: %w", err)
	}
	f.Limit = port.MaxLimit
	f.Cursor = 0
	written := 0
	for {
		page, err := s.repo.List(ctx, f)
		if err != nil {
			return err
		}
		for _, e := range page.Entries {
			if written >= MaxExportRows {
				cw.Flush()
				return cw.Error()
			}
			if err := cw.Write(csvRow(e)); err != nil {
				return fmt.Errorf("audit: write csv row: %w", err)
			}
			written++
		}
		if page.NextCursor == 0 {
			break
		}
		f.Cursor = page.NextCursor
	}
	cw.Flush()
	return cw.Error()
}

func csvRow(e domain.Entry) []string {
	entityID := ""
	if e.EntityID != nil {
		entityID = e.EntityID.String()
	}
	return []string{
		strconv.FormatInt(e.ID, 10), e.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		safeCell(e.ActorName), safeCell(e.Action), safeCell(e.EntityType), entityID,
		safeCell(e.Summary), e.IP, safeCell(e.RequestID),
	}
}

// safeCell neutralises spreadsheet formula injection: cells starting with a formula character get a leading quote.
func safeCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

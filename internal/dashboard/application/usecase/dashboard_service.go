package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
)

type DashboardService struct {
	reader port.Reader
	now    func() time.Time
}

func NewDashboardService(reader port.Reader) *DashboardService {
	return &DashboardService{reader: reader, now: func() time.Time { return time.Now().UTC() }}
}

var _ port.Service = (*DashboardService)(nil)

// Me devuelve el resumen personal con la hora actual, que decide que incidencias estan vencidas.
func (s *DashboardService) Me(ctx context.Context, userID uuid.UUID) (port.Summary, error) {
	return s.reader.Summary(ctx, userID, s.now())
}

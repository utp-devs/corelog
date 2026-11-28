// Package usecase implements the roadmap use cases.
package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/roadmap/application/port"
	"github.com/jhonsferg/corelog/internal/roadmap/domain"
)

// Service implements port.Service.
type Service struct {
	repo       port.Repository
	authorizer port.Authorizer
}

var _ port.Service = (*Service)(nil)

// NewService builds the service.
func NewService(repo port.Repository, authorizer port.Authorizer) *Service {
	return &Service{repo: repo, authorizer: authorizer}
}

// Roadmap returns the bars of the project for the window, the version flags and sprint bands, and the epic
// dependencies. It needs the permission to browse the project.
func (s *Service) Roadmap(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, q domain.Query) (port.Roadmap, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return port.Roadmap{}, err
	}
	if err := domain.ValidateRange(q.From, q.To); err != nil {
		return port.Roadmap{}, err
	}
	if q.Group == "" {
		q.Group = domain.GroupEpic
	}
	if q.Filter == "" {
		q.Filter = domain.FilterAll
	}
	epics, err := s.repo.Epics(ctx, projectID)
	if err != nil {
		return port.Roadmap{}, err
	}
	versionRows, err := s.repo.Versions(ctx, projectID)
	if err != nil {
		return port.Roadmap{}, err
	}
	sprintRows, err := s.repo.Sprints(ctx, projectID)
	if err != nil {
		return port.Roadmap{}, err
	}
	deps, err := s.repo.Dependencies(ctx, projectID)
	if err != nil {
		return port.Roadmap{}, err
	}
	epicBars := make([]domain.Bar, len(epics))
	for i, e := range epics {
		epicBars[i] = domain.EpicBar(e)
	}
	versions := make([]domain.Bar, len(versionRows))
	for i, v := range versionRows {
		versions[i] = domain.PeriodBar(v)
	}
	sprints := make([]domain.Bar, len(sprintRows))
	for i, sp := range sprintRows {
		sprints[i] = domain.PeriodBar(sp)
	}
	out := port.Roadmap{From: q.From, To: q.To, Group: q.Group}
	switch q.Group {
	case domain.GroupVersion:
		out.Bars = domain.FilterBars(versions, q)
	case domain.GroupSprint:
		out.Bars = domain.FilterBars(sprints, q)
	default:
		out.Bars = domain.FilterBars(epicBars, q)
	}
	// Flags and bands only need the window, not the completion filter or the text of the epics.
	window := domain.Query{From: q.From, To: q.To, Filter: domain.FilterAll}
	out.Versions = domain.FilterBars(versions, window)
	out.Sprints = domain.FilterBars(sprints, window)
	// Keep the dependencies whose two ends are in the result.
	shown := map[uuid.UUID]bool{}
	for _, b := range out.Bars {
		shown[b.ID] = true
	}
	out.Dependencies = make([]domain.Dependency, 0, len(deps))
	if q.Group == domain.GroupEpic {
		for _, d := range deps {
			if shown[d.From] && shown[d.To] {
				out.Dependencies = append(out.Dependencies, d)
			}
		}
	}
	return out, nil
}

// Dependencies lists every blocks link between epics of the project.
func (s *Service) Dependencies(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]domain.Dependency, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	deps, err := s.repo.Dependencies(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if deps == nil {
		deps = []domain.Dependency{}
	}
	return deps, nil
}

// Rank stores the manual order of the epics. It needs the permission to edit issues.
func (s *Service) Rank(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, epicIDs []uuid.UUID) error {
	if err := s.authorizer.Authorize(ctx, port.ActionViewProject, actorID, globalRole, projectID); err != nil {
		return err
	}
	if err := s.authorizer.Authorize(ctx, port.ActionEditIssue, actorID, globalRole, projectID); err != nil {
		return err
	}
	if err := domain.ValidateRankIDs(epicIDs, port.MaxRankedEpics); err != nil {
		return err
	}
	return s.repo.SaveRank(ctx, projectID, epicIDs)
}

// Package usecase orchestrates the reports: it authorizes, reads the rows and runs the pure computation.
package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/report/application/port"
	"github.com/jhonsferg/corelog/internal/report/domain"
)

// Service runs the project reports.
type Service struct {
	reader     port.Reader
	authorizer port.Authorizer
	filter     port.IssueFilter
	sprints    port.SprintSummarizer
	clock      clock.Clock
}

var _ port.Service = (*Service)(nil)

// NewService builds the service. filter and sprints may be nil: a filter is then rejected and the sprint
// summary is unavailable.
func NewService(reader port.Reader, authorizer port.Authorizer, filter port.IssueFilter, sprints port.SprintSummarizer, clk clock.Clock) *Service {
	return &Service{reader: reader, authorizer: authorizer, filter: filter, sprints: sprints, clock: clk}
}

// List returns the catalog once the actor may view reports of the project.
func (s *Service) List(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]domain.Descriptor, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewReports, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	return domain.Catalog(), nil
}

// Run computes one report.
func (s *Service) Run(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, kindName string, raw domain.RawParams) (*port.Result, error) {
	if err := s.authorizer.Authorize(ctx, port.ActionViewReports, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	kind, ok := domain.ParseKind(kindName)
	if !ok {
		return nil, domain.ErrKindUnknown
	}
	project, err := s.reader.Project(ctx, projectID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	loc := project.Location()
	params, err := domain.ResolveParams(kind, raw, loc, now)
	if err != nil {
		return nil, err
	}
	var only *[]uuid.UUID
	if params.Filter != "" && domain.AcceptsFilter(kind) {
		if s.filter == nil {
			return nil, domain.ErrFilterInvalid
		}
		ids, err := s.filter.MatchIssues(ctx, actorID, globalRole, projectID, params.Filter, port.MaxFilterIssues)
		if err != nil {
			return nil, err
		}
		if len(ids) > port.MaxFilterIssues {
			return nil, domain.ErrFilterTooBroad
		}
		only = &ids
	}
	report, params, err := s.compute(ctx, actorID, globalRole, projectID, params, only, now)
	if err != nil {
		return nil, err
	}
	return &port.Result{Report: report, GeneratedAt: now, Params: params.Echo()}, nil
}

func (s *Service) compute(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, p domain.Params, only *[]uuid.UUID, now time.Time) (*domain.Report, domain.Params, error) {
	switch p.Kind {
	case domain.KindBurndown, domain.KindBurnup:
		sprint, err := s.reader.Sprint(ctx, projectID, p.SprintID)
		if err != nil {
			return nil, p, err
		}
		p.SprintID = &sprint.ID
		events, err := s.reader.ScopeEvents(ctx, sprint.ID)
		if err != nil {
			return nil, p, err
		}
		ids := distinctIssues(events)
		if only != nil {
			ids = intersect(ids, *only)
		}
		issues, err := s.reader.Issues(ctx, port.IssueQuery{ProjectID: projectID, OnlyIDs: &ids})
		if err != nil {
			return nil, p, err
		}
		history, err := s.reader.StatusHistory(ctx, port.HistoryQuery{ProjectID: projectID, OnlyIDs: &ids, Before: now})
		if err != nil {
			return nil, p, err
		}
		return domain.BuildBurn(sprint, events, issues, history, p.GroupBy, p.Kind == domain.KindBurnup, p.Loc, now), p, nil

	case domain.KindVelocity:
		sprints, err := s.reader.RecentSprints(ctx, projectID, p.Limit)
		if err != nil {
			return nil, p, err
		}
		return domain.BuildVelocity(sprints), p, nil

	case domain.KindCumulativeFlow:
		issues, err := s.reader.Issues(ctx, port.IssueQuery{ProjectID: projectID, OnlyIDs: only, CreatedBefore: &p.Hi})
		if err != nil {
			return nil, p, err
		}
		history, err := s.reader.StatusHistory(ctx, port.HistoryQuery{ProjectID: projectID, OnlyIDs: only, Before: p.Hi})
		if err != nil {
			return nil, p, err
		}
		return domain.BuildCumulativeFlow(p, issues, history), p, nil

	case domain.KindControl:
		issues, err := s.reader.Issues(ctx, port.IssueQuery{ProjectID: projectID, OnlyIDs: only, CreatedBefore: &p.Hi, TouchedSince: &p.Lo})
		if err != nil {
			return nil, p, err
		}
		ids := make([]uuid.UUID, 0, len(issues))
		for _, i := range issues {
			ids = append(ids, i.ID)
		}
		history, err := s.reader.StatusHistory(ctx, port.HistoryQuery{ProjectID: projectID, OnlyIDs: &ids, Before: p.Hi})
		if err != nil {
			return nil, p, err
		}
		return domain.BuildControl(p, issues, history), p, nil

	case domain.KindCreatedVsRes, domain.KindResolutionTime:
		issues, err := s.reader.Issues(ctx, port.IssueQuery{ProjectID: projectID, OnlyIDs: only, CreatedBefore: &p.Hi, TouchedSince: &p.Lo})
		if err != nil {
			return nil, p, err
		}
		if p.Kind == domain.KindCreatedVsRes {
			return domain.BuildCreatedVsResolved(p, issues), p, nil
		}
		return domain.BuildResolutionTime(p, issues), p, nil

	case domain.KindTimeTracking:
		rows, err := s.reader.Worklogs(ctx, port.WorklogQuery{ProjectID: projectID, Lo: p.Lo, Hi: p.Hi, OnlyIDs: only, SprintID: p.SprintID})
		if err != nil {
			return nil, p, err
		}
		return domain.BuildTimeTracking(p, rows), p, nil

	case domain.KindWorkload:
		issues, err := s.reader.Issues(ctx, port.IssueQuery{ProjectID: projectID, OnlyIDs: only, OpenOnly: true})
		if err != nil {
			return nil, p, err
		}
		return domain.BuildWorkload(issues), p, nil

	case domain.KindVersion:
		version, err := s.reader.Version(ctx, projectID, *p.VersionID)
		if err != nil {
			return nil, p, err
		}
		issues, err := s.reader.Issues(ctx, port.IssueQuery{ProjectID: projectID, OnlyIDs: only, VersionID: p.VersionID})
		if err != nil {
			return nil, p, err
		}
		return domain.BuildVersion(version, issues, p.Loc, now), p, nil

	case domain.KindSprintReport:
		sprint, err := s.reader.Sprint(ctx, projectID, p.SprintID)
		if err != nil {
			return nil, p, err
		}
		p.SprintID = &sprint.ID
		if s.sprints == nil {
			return nil, p, domain.ErrKindUnknown
		}
		summary, err := s.sprints.SprintSummary(ctx, actorID, globalRole, projectID, sprint.ID)
		if err != nil {
			return nil, p, err
		}
		return domain.BuildSprintSummary(summary), p, nil
	}
	return nil, p, domain.ErrKindUnknown
}

func distinctIssues(events []domain.ScopeEvent) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	out := []uuid.UUID{}
	for _, e := range events {
		if !seen[e.IssueID] {
			seen[e.IssueID] = true
			out = append(out, e.IssueID)
		}
	}
	return out
}

func intersect(a, b []uuid.UUID) []uuid.UUID {
	in := make(map[uuid.UUID]bool, len(b))
	for _, id := range b {
		in[id] = true
	}
	out := []uuid.UUID{}
	for _, id := range a {
		if in[id] {
			out = append(out, id)
		}
	}
	return out
}

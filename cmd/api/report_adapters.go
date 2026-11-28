package main

import (
	"context"

	"github.com/google/uuid"

	reportport "github.com/jhonsferg/corelog/internal/report/application/port"
	reportdomain "github.com/jhonsferg/corelog/internal/report/domain"
	searchport "github.com/jhonsferg/corelog/internal/search/application/port"
	sprintport "github.com/jhonsferg/corelog/internal/sprint/application/port"
	sprintdomain "github.com/jhonsferg/corelog/internal/sprint/domain"
)

// reportSearchPageSize is the page size used to collect the issues a report filter matches.
const reportSearchPageSize = 100

// reportIssueFilter resolves a CoreQL text of a report to issue ids through the search module, so the
// usual visibility rules of the search apply.
type reportIssueFilter struct {
	search searchport.IssueSearchService
}

var _ reportport.IssueFilter = reportIssueFilter{}

func (f reportIssueFilter) MatchIssues(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, text string, limit int) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	for page := 1; len(ids) <= limit; page++ {
		out, err := f.search.Search(ctx, searchport.SearchInput{
			Viewer:    searchport.Viewer{UserID: actorID, PlatformAdmin: globalRole == "admin"},
			Query:     text,
			ProjectID: &projectID,
			Page:      page,
			PageSize:  reportSearchPageSize,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range out.Page.Items {
			ids = append(ids, item.ID)
		}
		if len(out.Page.Items) < reportSearchPageSize {
			break
		}
	}
	return ids, nil
}

// reportSprintSummarizer reuses the sprint report of the sprint module.
type reportSprintSummarizer struct {
	sprints sprintport.Service
}

var _ reportport.SprintSummarizer = reportSprintSummarizer{}

func (s reportSprintSummarizer) SprintSummary(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) (reportdomain.SprintSummary, error) {
	rep, err := s.sprints.Report(ctx, actorID, globalRole, projectID, sprintID)
	if err != nil {
		return reportdomain.SprintSummary{}, err
	}
	out := reportdomain.SprintSummary{
		Name: rep.Sprint.Name, Status: string(rep.Sprint.Status), Committed: rep.CommittedPoints, Completed: rep.CompletedPoints,
		Incomplete: rep.IncompletePoints, Added: rep.AddedPoints, Removed: rep.RemovedPoints,
	}
	add := func(outcome string, items []sprintdomain.ReportIssue) {
		for _, it := range items {
			out.Items = append(out.Items, reportdomain.SprintItem{Key: it.Key, Summary: it.Summary, Points: it.Points, Outcome: outcome, Added: it.AddedAfterStart})
		}
	}
	add("completed", rep.Completed)
	add("incomplete", rep.Incomplete)
	add("removed", rep.Removed)
	return out, nil
}

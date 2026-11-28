package main

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	calendarport "github.com/jhonsferg/corelog/internal/calendar/application/port"
	dashboardport "github.com/jhonsferg/corelog/internal/dashboard/application/port"
	dashboarddomain "github.com/jhonsferg/corelog/internal/dashboard/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/search"
	searchport "github.com/jhonsferg/corelog/internal/search/application/port"
	searchdomain "github.com/jhonsferg/corelog/internal/search/domain"
)

// dashboardIssueRunner lets the dashboard gadgets run CoreQL queries and saved filters as the viewer through the
// search module, so the result is limited to the projects the viewer can browse.
type dashboardIssueRunner struct {
	search *search.Module
}

var _ dashboardport.IssueRunner = dashboardIssueRunner{}

func searchViewer(v dashboardport.Viewer) searchport.Viewer {
	return searchport.Viewer{UserID: v.UserID, PlatformAdmin: v.PlatformAdmin}
}

func (a dashboardIssueRunner) RunQuery(ctx context.Context, viewer dashboardport.Viewer, query string, limit int) (dashboardport.IssueList, error) {
	if a.search.Issues == nil {
		return dashboardport.IssueList{}, apperror.WithCode(apperror.KindInternal, dashboarddomain.CodeSourceFailed, "la búsqueda de incidencias no está disponible")
	}
	out, err := a.search.Issues.Search(ctx, searchport.SearchInput{Viewer: searchViewer(viewer), Query: query, Page: 1, PageSize: limit, WithTotal: true})
	if err != nil {
		return dashboardport.IssueList{}, gadgetQueryError(err)
	}
	return issueList(out, query), nil
}

func (a dashboardIssueRunner) RunFilter(ctx context.Context, viewer dashboardport.Viewer, id uuid.UUID, limit int) (dashboardport.IssueList, error) {
	if a.search.Filters == nil {
		return dashboardport.IssueList{}, apperror.WithCode(apperror.KindInternal, dashboarddomain.CodeSourceFailed, "los filtros guardados no están disponibles")
	}
	out, err := a.search.Filters.Run(ctx, searchViewer(viewer), id, 1, limit, true)
	if err != nil {
		return dashboardport.IssueList{}, gadgetQueryError(err)
	}
	return issueList(out, out.Normalized), nil
}

// gadgetQueryError reports a query that no longer parses as a validation error of the gadget; every other error
// (not found, forbidden, internal) keeps its kind.
func gadgetQueryError(err error) error {
	var qe *searchdomain.QueryError
	if errors.As(err, &qe) {
		return apperror.WithCode(apperror.KindValidation, "dashboard.invalid_query", "la consulta del gadget no es válida: "+qe.Error())
	}
	return err
}

func issueList(out searchport.SearchOutput, query string) dashboardport.IssueList {
	list := dashboardport.IssueList{Total: out.Page.Total, Query: query, Items: make([]dashboardport.IssueRow, 0, len(out.Page.Items))}
	if !out.Page.HasTotal {
		list.Total = len(out.Page.Items)
	}
	for _, it := range out.Page.Items {
		row := dashboardport.IssueRow{ID: it.ID, Key: it.Key, Summary: it.Summary, ProjectKey: it.ProjectKey, Due: it.DueDate, Updated: it.UpdatedAt}
		if it.Status != nil {
			row.Status, row.StatusCategory = it.Status.Name, it.Status.Category
		}
		if it.Priority != nil {
			row.Priority = it.Priority.Name
		}
		if it.IssueType != nil {
			row.Type = it.IssueType.Name
		}
		if it.AssigneeName != nil {
			row.Assignee = *it.AssigneeName
		}
		list.Items = append(list.Items, row)
	}
	return list
}

// dashboardUpcoming lists the calendar entries of the viewer for the upcoming gadget through the calendar module.
type dashboardUpcoming struct {
	events calendarport.EventService
}

var _ dashboardport.UpcomingSource = dashboardUpcoming{}

func (a dashboardUpcoming) Upcoming(ctx context.Context, viewer dashboardport.Viewer, from, to time.Time, limit int) ([]dashboardport.UpcomingItem, error) {
	role := "user"
	if viewer.PlatformAdmin {
		role = "admin"
	}
	res, err := a.events.Items(ctx, calendarport.Viewer{UserID: viewer.UserID, GlobalRole: role}, calendarport.ItemsQuery{From: from, To: to})
	if err != nil {
		return nil, err
	}
	out := make([]dashboardport.UpcomingItem, 0, limit)
	for _, it := range res.Items {
		if !it.EndsAt.After(from) && !it.StartsAt.Equal(it.EndsAt) {
			continue // already over
		}
		out = append(out, dashboardport.UpcomingItem{Kind: it.Kind, Title: it.Title, StartsAt: it.StartsAt, EndsAt: it.EndsAt, AllDay: it.AllDay, Link: it.Link, Project: it.ProjectKey})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

package http

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
)

// utf8BOM starts the CSV so spreadsheets read the accents as UTF-8.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// parseIssueFilter reads the filters shared by the issue list and the CSV export.
func parseIssueFilter(q url.Values) (port.ListFilter, error) {
	filter := port.ListFilter{
		Status:    domain.Status(q.Get("status")),
		Priority:  domain.Priority(q.Get("priority")),
		Query:     q.Get("q"),
		IssueType: domain.IssueType(q.Get("issue_type")),
		Page:      atoiOrDefault(q.Get("page"), 1),
		PageSize:  atoiOrDefault(q.Get("page_size"), 20),
	}

	if labelID := q.Get("label_id"); labelID != "" {
		id, err := uuid.Parse(labelID)
		if err != nil {
			return filter, apperror.New(apperror.KindValidation, "invalid label id")
		}
		filter.LabelID = id
	}
	if raw := q.Get("fix_version"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return filter, apperror.New(apperror.KindValidation, "invalid fix version id")
		}
		filter.FixVersionID = id
	}
	if raw := q.Get("component"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return filter, apperror.New(apperror.KindValidation, "invalid component id")
		}
		filter.ComponentID = id
	}
	switch assigneeID := q.Get("assignee_id"); assigneeID {
	case "":
	case unassignedFilterValue:
		filter.Unassigned = true
	default:
		id, err := uuid.Parse(assigneeID)
		if err != nil {
			return filter, apperror.New(apperror.KindValidation, "invalid assignee id")
		}
		filter.AssigneeID = &id
	}
	if dueFrom := q.Get("due_from"); dueFrom != "" {
		value, err := time.Parse(time.RFC3339, dueFrom)
		if err != nil {
			return filter, apperror.New(apperror.KindValidation, "invalid due_from")
		}
		filter.DueFrom = &value
	}
	if dueTo := q.Get("due_to"); dueTo != "" {
		value, err := time.Parse(time.RFC3339, dueTo)
		if err != nil {
			return filter, apperror.New(apperror.KindValidation, "invalid due_to")
		}
		filter.DueTo = &value
	}
	return filter, nil
}

// Export streams the issues matching the list filters as CSV. The columns parameter selects and orders the
// columns; at most domain.MaxExportRows rows are written and X-Export-Truncated says when rows were left out.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := h.actorOf(w, r)
	if !ok {
		return
	}
	if h.ops == nil {
		h.operationsUnavailable(w)
		return
	}
	q := r.URL.Query()
	filter, err := parseIssueFilter(q)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	columns, err := domain.ParseExportColumns(q.Get("columns"))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	var writer *csv.Writer
	started := false
	start := func(truncated bool) error {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, h.ops.ExportFileName()))
		if truncated {
			w.Header().Set("X-Export-Truncated", "true")
		}
		w.WriteHeader(http.StatusOK)
		started = true
		if _, err := w.Write(utf8BOM); err != nil {
			return err
		}
		writer = csv.NewWriter(w)
		headers := make([]string, 0, len(columns))
		for _, key := range columns {
			headers = append(headers, domain.ExportHeader(key))
		}
		return writer.Write(headers)
	}
	emit := func(batch port.ExportBatch) error {
		for _, issue := range batch.Issues {
			row := make([]string, 0, len(columns))
			for _, key := range columns {
				row = append(row, domain.SafeCSVCell(exportCell(key, issue, batch)))
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	}
	if err := h.ops.ExportIssues(r.Context(), actorID, globalRole, filter, start, emit); err != nil {
		if !started {
			httpresponse.Error(w, err)
		}
		return
	}
	if writer != nil {
		writer.Flush()
	}
}

// exportCell renders one column of one issue. Rows only hold the data of the batch they came with.
func exportCell(column string, issue *domain.Issue, batch port.ExportBatch) string {
	switch column {
	case "key":
		return issue.Key()
	case "project":
		return issue.ProjectKey
	case "summary":
		return issue.Summary
	case "description":
		return issue.Description
	case "type":
		if issue.IssueTypeRef != nil {
			return issue.IssueTypeRef.Name
		}
		return string(issue.IssueType)
	case "status":
		return string(issue.Status)
	case "priority":
		if issue.PriorityRef != nil {
			return issue.PriorityRef.Name
		}
		return string(issue.Priority)
	case "resolution":
		if issue.ResolutionRef != nil {
			return issue.ResolutionRef.Name
		}
		return ""
	case "assignee":
		if issue.AssigneeID == nil {
			return ""
		}
		return batch.Names[*issue.AssigneeID]
	case "reporter":
		return batch.Names[issue.ReporterID]
	case "created":
		return issue.CreatedAt.UTC().Format(time.RFC3339)
	case "updated":
		return issue.UpdatedAt.UTC().Format(time.RFC3339)
	case "due_date":
		return domain.FormatDate(issue.DueDate)
	case "start_date":
		return domain.FormatDate(issue.StartDate)
	case "story_points":
		if issue.StoryPoints == nil {
			return ""
		}
		return strconv.FormatFloat(*issue.StoryPoints, 'f', -1, 64)
	case "labels":
		return strings.Join(batch.Labels[issue.ID], ", ")
	case "votes":
		return strconv.Itoa(issue.VoteCount)
	case "flagged":
		return strconv.FormatBool(issue.Flagged)
	default:
		return ""
	}
}

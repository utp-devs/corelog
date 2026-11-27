package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

// maxBodyBytes bounds a request body.
const maxBodyBytes = 1 << 20

// viewerOf builds the viewer from the authenticated request.
func viewerOf(r *http.Request) port.Viewer {
	id, _ := appmw.UserIDFromContext(r.Context())
	role, _ := appmw.RoleFromContext(r.Context())
	return port.Viewer{UserID: id, PlatformAdmin: role == "admin"}
}

func invalidBody() error {
	return apperror.WithCode(apperror.KindValidation, "validation.failed", "cuerpo de la petición no válido")
}

// decodeStrict reads one JSON object, rejecting unknown fields and trailing data.
func decodeStrict(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		httpresponse.Error(w, invalidBody())
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
		httpresponse.Error(w, invalidBody())
		return false
	}
	return true
}

// writeError writes an error. Rejected queries carry their located problems in details.
func writeError(w http.ResponseWriter, err error) {
	var qe *domain.QueryError
	if errors.As(err, &qe) {
		writeQueryError(w, qe)
		return
	}
	httpresponse.Error(w, err)
}

type problemResponse struct {
	Position    int      `json:"position"`
	Length      int      `json:"length"`
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	Suggestions []string `json:"suggestions"`
	Expected    []string `json:"expected,omitempty"`
}

func problemsOf(in []domain.Problem) []problemResponse {
	out := make([]problemResponse, 0, len(in))
	for _, p := range in {
		sug := p.Suggestions
		if sug == nil {
			sug = []string{}
		}
		out = append(out, problemResponse{Position: p.Pos, Length: p.Len, Code: p.Code, Message: p.Message, Suggestions: sug, Expected: p.Expected})
	}
	return out
}

func writeQueryError(w http.ResponseWriter, qe *domain.QueryError) {
	code := apperror.CodeOf(qe)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
		"code": code, "message": qe.Error(), "details": problemsOf(qe.Problems),
	}})
}

// searchRequest is the body of POST /issues/search.
type searchRequest struct {
	JQL       string   `json:"jql"`
	Order     string   `json:"order"`
	Page      int      `json:"page"`
	PageSize  int      `json:"page_size"`
	WithTotal *bool    `json:"with_total"`
	ProjectID *string  `json:"project_id"`
	Fields    []string `json:"fields"`
}

func readSearchRequest(w http.ResponseWriter, r *http.Request) (searchRequest, bool) {
	var req searchRequest
	if r.Method == http.MethodPost {
		if !decodeStrict(w, r, &req) {
			return req, false
		}
		return req, true
	}
	q := r.URL.Query()
	req.JQL, req.Order = q.Get("jql"), q.Get("order")
	for name, dst := range map[string]*int{"page": &req.Page, "page_size": &req.PageSize} {
		if raw := q.Get(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", name+" no es válido"))
				return req, false
			}
			*dst = n
		}
	}
	if raw := q.Get("total"); raw != "" {
		v := raw != "false" && raw != "0"
		req.WithTotal = &v
	}
	if raw := q.Get("project_id"); raw != "" {
		req.ProjectID = &raw
	}
	if raw := q.Get("fields"); raw != "" {
		req.Fields = strings.Split(raw, ",")
	}
	return req, true
}

// SearchIssues runs a CoreQL query over the issues the caller can browse.
func (h *Handler) SearchIssues(w http.ResponseWriter, r *http.Request) {
	req, ok := readSearchRequest(w, r)
	if !ok {
		return
	}
	var project *uuid.UUID
	if req.ProjectID != nil && *req.ProjectID != "" {
		id, err := uuid.Parse(*req.ProjectID)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "project_id no es válido"))
			return
		}
		project = &id
	}
	withTotal := req.WithTotal == nil || *req.WithTotal
	out, err := h.issues.Search(r.Context(), port.SearchInput{
		Viewer: viewerOf(r), Query: req.JQL, Order: req.Order, ProjectID: project,
		Page: req.Page, PageSize: req.PageSize, WithTotal: withTotal,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeIssuePage(w, out, req.Fields)
}

type warningResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type issueListMeta struct {
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Total    *int              `json:"total"`
	Query    string            `json:"query"`
	Warnings []warningResponse `json:"warnings"`
}

// writeIssuePage writes {data, meta} with the issue list shape plus the total and the parse warnings.
func writeIssuePage(w http.ResponseWriter, out port.SearchOutput, fields []string) {
	meta := issueListMeta{Page: out.PageNumber, PageSize: out.PageSize, Query: out.Normalized, Warnings: []warningResponse{}}
	if out.Page.HasTotal {
		t := out.Page.Total
		meta.Total = &t
	}
	for _, wn := range out.Warnings {
		meta.Warnings = append(meta.Warnings, warningResponse{Code: wn.Code, Message: wn.Message})
	}
	items := toIssueResponses(out.Page.Items)
	var data any = items
	if len(fields) > 0 {
		data = projectFields(items, fields)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "meta": meta})
}

// projectFields keeps id, key and the requested top-level fields of each issue.
func projectFields(items []issueResponse, fields []string) []map[string]any {
	keep := map[string]bool{"id": true, "key": true}
	for _, f := range fields {
		keep[strings.TrimSpace(f)] = true
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		raw, _ := json.Marshal(it)
		var full map[string]any
		_ = json.Unmarshal(raw, &full)
		row := make(map[string]any, len(keep))
		for k, v := range full {
			if keep[k] {
				row[k] = v
			}
		}
		out = append(out, row)
	}
	return out
}

type validateRequest struct {
	JQL       string  `json:"jql"`
	ProjectID *string `json:"project_id"`
}

// Validate reports the problems of a query text with positions and suggestions, without running it.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	var project *uuid.UUID
	if req.ProjectID != nil && *req.ProjectID != "" {
		id, err := uuid.Parse(*req.ProjectID)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "project_id no es válido"))
			return
		}
		project = &id
	}
	report, err := h.issues.Validate(r.Context(), viewerOf(r), req.JQL, project)
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, map[string]any{
		"valid": report.Valid, "query": report.Normalized, "errors": problemsOf(report.Problems),
	})
}

type completionResponse struct {
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type valueResponse struct {
	Value  string `json:"value"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

// Suggest returns autocomplete data: values of a field (field and q), or completions of a query at a cursor
// (jql and cursor).
func (h *Handler) Suggest(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	in := port.SuggestInput{Viewer: viewerOf(r), Query: q.Get("jql"), Field: strings.TrimSpace(q.Get("field")), Prefix: q.Get("q"), Cursor: -1}
	if raw := q.Get("cursor"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "cursor no es válido"))
			return
		}
		in.Cursor = n
	}
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "limit no es válido"))
			return
		}
		in.Limit = n
	}
	out, err := h.issues.Suggest(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	completions := make([]completionResponse, 0, len(out.Completions))
	for _, c := range out.Completions {
		completions = append(completions, completionResponse{Label: c.Label, Kind: string(c.Kind), Detail: c.Detail})
	}
	values := make([]valueResponse, 0, len(out.Values))
	for _, v := range out.Values {
		values = append(values, valueResponse{Value: v.Value, Label: v.Label, Detail: v.Detail})
	}
	httpresponse.JSON(w, http.StatusOK, map[string]any{"completions": completions, "values": values})
}

// issueResponse mirrors the entries of the issue list.
type issueResponse struct {
	ID                       string             `json:"id"`
	Key                      string             `json:"key"`
	ProjectID                string             `json:"project_id"`
	ProjectKey               string             `json:"project_key"`
	Summary                  string             `json:"summary"`
	Description              string             `json:"description"`
	Status                   string             `json:"status"`
	WorkflowStatus           *workflowStatusRef `json:"workflow_status"`
	IssueType                *issueTypeRef      `json:"issue_type"`
	Priority                 *simpleRef         `json:"priority"`
	Resolution               *resolutionRef     `json:"resolution"`
	ResolvedAt               *time.Time         `json:"resolved_at"`
	ParentID                 *string            `json:"parent_id,omitempty"`
	StartDate                *string            `json:"start_date"`
	DueDate                  *string            `json:"due_date"`
	StoryPoints              *float64           `json:"story_points"`
	OriginalEstimateSeconds  *int64             `json:"original_estimate_seconds"`
	RemainingEstimateSeconds *int64             `json:"remaining_estimate_seconds"`
	Flagged                  bool               `json:"flagged"`
	VoteCount                int                `json:"vote_count"`
	Rank                     string             `json:"rank"`
	ClosedAt                 *time.Time         `json:"closed_at,omitempty"`
	ReporterID               string             `json:"reporter_id"`
	ReporterName             string             `json:"reporter_name"`
	AssigneeID               *string            `json:"assignee_id,omitempty"`
	AssigneeName             *string            `json:"assignee_name,omitempty"`
	Labels                   []string           `json:"labels"`
	Custom                   map[string]any     `json:"custom"`
	CreatedAt                time.Time          `json:"created_at"`
	UpdatedAt                time.Time          `json:"updated_at"`
}

type workflowStatusRef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type issueTypeRef struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Icon           string `json:"icon"`
	Color          string `json:"color"`
	HierarchyLevel int    `json:"hierarchy_level"`
}

type simpleRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

type resolutionRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func toIssueResponses(items []port.IssueRecord) []issueResponse {
	out := make([]issueResponse, 0, len(items))
	for _, it := range items {
		resp := issueResponse{
			ID: it.ID.String(), Key: it.Key, ProjectID: it.ProjectID.String(), ProjectKey: it.ProjectKey,
			Summary: it.Summary, Description: it.Description, Status: it.LegacyStatus,
			ResolvedAt: it.ResolvedAt, StartDate: it.StartDate, DueDate: it.DueDate, StoryPoints: it.StoryPoints,
			OriginalEstimateSeconds: it.OriginalEstimateSeconds, RemainingEstimateSeconds: it.RemainingEstimateSeconds,
			Flagged: it.Flagged, VoteCount: it.VoteCount, Rank: it.Rank, ClosedAt: it.ClosedAt,
			ReporterID: it.ReporterID.String(), ReporterName: it.ReporterName, AssigneeName: it.AssigneeName,
			Labels: it.Labels, Custom: it.Custom, CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt,
		}
		if it.Status != nil {
			resp.WorkflowStatus = &workflowStatusRef{ID: it.Status.ID.String(), Name: it.Status.Name, Category: it.Status.Category}
		}
		if it.IssueType != nil {
			resp.IssueType = &issueTypeRef{ID: it.IssueType.ID.String(), Name: it.IssueType.Name, Icon: it.IssueType.Icon, Color: it.IssueType.Color, HierarchyLevel: it.IssueType.HierarchyLevel}
		}
		if it.Priority != nil {
			resp.Priority = &simpleRef{ID: it.Priority.ID.String(), Name: it.Priority.Name, Icon: it.Priority.Icon, Color: it.Priority.Color}
		}
		if it.Resolution != nil {
			resp.Resolution = &resolutionRef{ID: it.Resolution.ID.String(), Name: it.Resolution.Name}
		}
		if it.ParentID != nil {
			s := it.ParentID.String()
			resp.ParentID = &s
		}
		if it.AssigneeID != nil {
			s := it.AssigneeID.String()
			resp.AssigneeID = &s
		}
		out = append(out, resp)
	}
	return out
}

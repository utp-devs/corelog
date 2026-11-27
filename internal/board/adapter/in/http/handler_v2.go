package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// WithBoards enables the board v2 routes.
func (h *Handler) WithBoards(service port.BoardsService) *Handler {
	h.boards = service
	return h
}

func (h *Handler) registerBoardRoutes(r chi.Router) {
	r.Route("/projects/{id}/boards", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.listBoards)
		r.Post("/", h.createBoard)
	})
	r.Route("/boards/{id}", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.getBoard)
		r.Patch("/", h.updateBoard)
		r.Delete("/", h.deleteBoard)
		r.Put("/columns", h.replaceColumns)
		r.Put("/quick-filters", h.replaceQuickFilters)
		r.Put("/swimlanes", h.replaceSwimlanes)
		r.Get("/view", h.viewBoard)
		r.Post("/cards/{issue_id}/move", h.moveBoardCard)
	})
}

// Response shapes.

type columnDTO struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	Color     string      `json:"color"`
	Position  int         `json:"position"`
	MinIssues *int        `json:"min_issues"`
	MaxIssues *int        `json:"max_issues"`
	StatusIDs []uuid.UUID `json:"status_ids"`
}

type queryDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Query string    `json:"query"`
}

type workflowStatusDTO struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Category string    `json:"category"`
}

type boardDetailDTO struct {
	ID                uuid.UUID           `json:"id"`
	ProjectID         uuid.UUID           `json:"project_id"`
	Name              string              `json:"name"`
	Type              string              `json:"type"`
	IsDefault         bool                `json:"is_default"`
	SwimlaneMode      string              `json:"swimlane_mode"`
	Estimation        string              `json:"estimation"`
	CardFields        []string            `json:"card_fields"`
	ShowUnmappedLane  bool                `json:"show_unmapped_lane"`
	DoneRetentionDays *int                `json:"done_retention_days"`
	Columns           []columnDTO         `json:"columns"`
	QuickFilters      []queryDTO          `json:"quick_filters"`
	Swimlanes         []queryDTO          `json:"swimlanes"`
	WorkflowStatuses  []workflowStatusDTO `json:"workflow_statuses"`
	UnmappedStatusIDs []uuid.UUID         `json:"unmapped_status_ids"`
	CanManageBoard    bool                `json:"can_manage_board"`
	CanMoveCards      bool                `json:"can_move_cards"`
}

type boardSummaryDTO struct {
	ID           uuid.UUID `json:"id"`
	ProjectID    uuid.UUID `json:"project_id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	IsDefault    bool      `json:"is_default"`
	ColumnCount  int       `json:"column_count"`
	SwimlaneMode string    `json:"swimlane_mode"`
}

func toColumnDTO(c domain.Column) columnDTO {
	ids := c.StatusIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return columnDTO{ID: c.ID, Name: c.Name, Color: c.Color, Position: c.Position, MinIssues: c.MinIssues, MaxIssues: c.WIPLimit, StatusIDs: ids}
}

func toDetailDTO(d *port.BoardDetail) boardDetailDTO {
	b := d.Board
	dto := boardDetailDTO{
		ID: b.ID, ProjectID: b.ProjectID, Name: b.Name, Type: string(b.Type), IsDefault: b.IsDefault,
		SwimlaneMode: string(b.SwimlaneMode), Estimation: string(b.Estimation), CardFields: b.CardFields,
		ShowUnmappedLane: b.ShowUnmappedLane, DoneRetentionDays: b.DoneRetentionDays,
		Columns: make([]columnDTO, 0, len(b.Columns)), QuickFilters: make([]queryDTO, 0, len(b.QuickFilters)),
		Swimlanes: make([]queryDTO, 0, len(b.Swimlanes)), WorkflowStatuses: make([]workflowStatusDTO, 0, len(d.WorkflowStatuses)),
		UnmappedStatusIDs: []uuid.UUID{}, CanManageBoard: d.CanManage, CanMoveCards: d.CanMoveCards,
	}
	if dto.CardFields == nil {
		dto.CardFields = []string{}
	}
	for _, c := range b.Columns {
		dto.Columns = append(dto.Columns, toColumnDTO(c))
	}
	for _, f := range b.QuickFilters {
		dto.QuickFilters = append(dto.QuickFilters, queryDTO{ID: f.ID, Name: f.Name, Query: f.Query})
	}
	for _, l := range b.Swimlanes {
		dto.Swimlanes = append(dto.Swimlanes, queryDTO{ID: l.ID, Name: l.Name, Query: l.Query})
	}
	order := make([]uuid.UUID, 0, len(d.WorkflowStatuses))
	for _, s := range d.WorkflowStatuses {
		dto.WorkflowStatuses = append(dto.WorkflowStatuses, workflowStatusDTO{ID: s.ID, Name: s.Name, Category: s.Category})
		order = append(order, s.ID)
	}
	if unmapped := domain.UnmappedStatuses(order, b.Columns); unmapped != nil {
		dto.UnmappedStatusIDs = unmapped
	}
	return dto
}

type refDTO struct {
	ID      uuid.UUID `json:"id"`
	Key     string    `json:"key"`
	Summary string    `json:"summary"`
}

type namedDTO struct {
	Name  string `json:"name"`
	Key   string `json:"key"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

type personDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type subtasksDTO struct {
	Total int `json:"total"`
	Done  int `json:"done"`
}

type viewCardDTO struct {
	ID               uuid.UUID         `json:"id"`
	Key              string            `json:"key"`
	Summary          string            `json:"summary"`
	Type             namedDTO          `json:"type"`
	Priority         namedDTO          `json:"priority"`
	Status           workflowStatusDTO `json:"status"`
	Assignee         *personDTO        `json:"assignee"`
	Labels           []string          `json:"labels"`
	StoryPoints      *float64          `json:"story_points"`
	RemainingSeconds *int64            `json:"remaining_estimate_seconds"`
	Flagged          bool              `json:"flagged"`
	Rank             string            `json:"rank"`
	SprintID         *uuid.UUID        `json:"sprint_id"`
	Epic             *refDTO           `json:"epic"`
	Parent           *refDTO           `json:"parent"`
	Subtasks         subtasksDTO       `json:"subtasks"`
	SwimlaneID       string            `json:"swimlane_id"`
}

type viewColumnDTO struct {
	columnDTO
	Count    int           `json:"count"`
	Total    int           `json:"total"`
	Estimate float64       `json:"estimate"`
	WIPState string        `json:"wip_state"`
	Cards    []viewCardDTO `json:"cards"`
}

// unmappedColumnDTO is the implicit column of the statuses no column maps.
type unmappedColumnDTO struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	StatusIDs []uuid.UUID   `json:"status_ids"`
	Count     int           `json:"count"`
	Total     int           `json:"total"`
	Estimate  float64       `json:"estimate"`
	Cards     []viewCardDTO `json:"cards"`
}

type laneDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

type sprintDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type viewDTO struct {
	Board          boardDetailDTO     `json:"board"`
	Sprint         *sprintDTO         `json:"sprint"`
	Columns        []viewColumnDTO    `json:"columns"`
	UnmappedColumn *unmappedColumnDTO `json:"unmapped_column"`
	UnmappedCount  int                `json:"unmapped_count"`
	Swimlanes      []laneDTO          `json:"swimlanes"`
}

func toCardDTO(c domain.PlacedCard) viewCardDTO {
	dto := viewCardDTO{
		ID: c.ID, Key: c.Key, Summary: c.Summary,
		Type:     namedDTO{Name: c.Type, Key: c.TypeKey, Icon: c.TypeIcon, Color: c.TypeColor},
		Priority: namedDTO{Name: c.Priority, Key: c.PriorityKey, Icon: c.PriorityIcon, Color: c.PriorityColor},
		Status:   workflowStatusDTO{ID: c.StatusID, Name: c.StatusName, Category: c.StatusCategory},
		Labels:   c.Labels, StoryPoints: c.StoryPoints, RemainingSeconds: c.RemainingSeconds, Flagged: c.Flagged,
		Rank: c.Rank, SprintID: c.SprintID, Subtasks: subtasksDTO{Total: c.SubtaskTotal, Done: c.SubtaskDone}, SwimlaneID: c.SwimlaneID,
	}
	if dto.Labels == nil {
		dto.Labels = []string{}
	}
	if c.AssigneeID != nil {
		dto.Assignee = &personDTO{ID: *c.AssigneeID, Name: c.AssigneeName}
	}
	if c.Epic != nil {
		dto.Epic = &refDTO{ID: c.Epic.ID, Key: c.Epic.Key, Summary: c.Epic.Summary}
	}
	if c.Parent != nil {
		dto.Parent = &refDTO{ID: c.Parent.ID, Key: c.Parent.Key, Summary: c.Parent.Summary}
	}
	return dto
}

func toViewColumnDTO(c domain.ViewColumn) viewColumnDTO {
	dto := viewColumnDTO{columnDTO: toColumnDTO(c.Column), Count: c.Count, Total: c.Total, Estimate: c.Estimate,
		WIPState: string(c.WIP), Cards: make([]viewCardDTO, 0, len(c.Cards))}
	for _, card := range c.Cards {
		dto.Cards = append(dto.Cards, toCardDTO(card))
	}
	return dto
}

func toViewDTO(v *port.ViewResult) viewDTO {
	dto := viewDTO{Board: toDetailDTO(v.Detail), Columns: make([]viewColumnDTO, 0, len(v.View.Columns)),
		UnmappedCount: v.View.UnmappedCount, Swimlanes: make([]laneDTO, 0, len(v.View.Swimlanes))}
	if v.Sprint != nil {
		dto.Sprint = &sprintDTO{ID: v.Sprint.ID, Name: v.Sprint.Name}
	}
	for _, c := range v.View.Columns {
		dto.Columns = append(dto.Columns, toViewColumnDTO(c))
	}
	if v.Detail.Board.ShowUnmappedLane {
		u := toViewColumnDTO(v.View.Unmapped)
		ids := []uuid.UUID{}
		order := make([]uuid.UUID, 0, len(v.Detail.WorkflowStatuses))
		for _, st := range v.Detail.WorkflowStatuses {
			order = append(order, st.ID)
		}
		if unmapped := domain.UnmappedStatuses(order, v.Detail.Board.Columns); unmapped != nil {
			ids = unmapped
		}
		dto.UnmappedColumn = &unmappedColumnDTO{ID: domain.UnmappedColumnID, Name: v.View.Unmapped.Name, StatusIDs: ids,
			Count: u.Count, Total: u.Total, Estimate: u.Estimate, Cards: u.Cards}
	}
	for _, l := range v.View.Swimlanes {
		dto.Swimlanes = append(dto.Swimlanes, laneDTO{ID: l.ID, Name: l.Name, Kind: string(l.Kind), Count: l.Count})
	}
	return dto
}

// Request shapes. Unknown fields are rejected.

type columnInput struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	Color     string      `json:"color"`
	MinIssues *int        `json:"min_issues"`
	MaxIssues *int        `json:"max_issues"`
	StatusIDs []uuid.UUID `json:"status_ids"`
}

func (c columnInput) spec() domain.ColumnSpec {
	return domain.ColumnSpec{ID: c.ID, Name: c.Name, Color: c.Color, MinIssues: c.MinIssues, MaxIssues: c.MaxIssues, StatusIDs: c.StatusIDs}
}

type createBoardRequest struct {
	Name              string        `json:"name"`
	Type              string        `json:"type"`
	SwimlaneMode      string        `json:"swimlane_mode"`
	Estimation        string        `json:"estimation"`
	CardFields        []string      `json:"card_fields"`
	ShowUnmappedLane  *bool         `json:"show_unmapped_lane"`
	DoneRetentionDays *int          `json:"done_retention_days"`
	Columns           []columnInput `json:"columns"`
}

type updateBoardRequest struct {
	Name              *string         `json:"name"`
	SwimlaneMode      *string         `json:"swimlane_mode"`
	Estimation        *string         `json:"estimation"`
	CardFields        *[]string       `json:"card_fields"`
	ShowUnmappedLane  *bool           `json:"show_unmapped_lane"`
	DoneRetentionDays json.RawMessage `json:"done_retention_days"`
}

type queryInput struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

func decodeStrict(r *http.Request, into any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return apperror.New(apperror.KindValidation, "cuerpo de la petición no válido")
	}
	return nil
}

type caller struct {
	actorID uuid.UUID
	role    string
}

func callerOf(w http.ResponseWriter, r *http.Request) (caller, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autenticado"))
		return caller{}, false
	}
	role, _ := appmw.RoleFromContext(r.Context())
	return caller{actorID: actorID, role: role}, true
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "identificador no válido"))
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) listBoards(w http.ResponseWriter, r *http.Request) {
	projectID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, ok := callerOf(w, r)
	if !ok {
		return
	}
	boards, err := h.boards.List(r.Context(), projectID, c.actorID, c.role)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]boardSummaryDTO, 0, len(boards))
	for _, b := range boards {
		out = append(out, boardSummaryDTO{ID: b.ID, ProjectID: b.ProjectID, Name: b.Name, Type: string(b.Type), IsDefault: b.IsDefault,
			ColumnCount: b.ColumnCount, SwimlaneMode: string(b.SwimlaneMode)})
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

func (h *Handler) createBoard(w http.ResponseWriter, r *http.Request) {
	projectID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, ok := callerOf(w, r)
	if !ok {
		return
	}
	var body createBoardRequest
	if err := decodeStrict(r, &body); err != nil {
		httpresponse.Error(w, err)
		return
	}
	in := port.CreateBoardInput{Type: domain.Type(body.Type), Settings: domain.Settings{
		Name: body.Name, SwimlaneMode: domain.SwimlaneMode(body.SwimlaneMode), Estimation: domain.Estimation(body.Estimation),
		CardFields: body.CardFields, ShowUnmappedLane: body.ShowUnmappedLane == nil || *body.ShowUnmappedLane, DoneRetentionDays: body.DoneRetentionDays,
	}}
	for _, col := range body.Columns {
		in.Columns = append(in.Columns, col.spec())
	}
	detail, err := h.boards.Create(r.Context(), projectID, c.actorID, c.role, in)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toDetailDTO(detail))
}

func (h *Handler) getBoard(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, ok := callerOf(w, r)
	if !ok {
		return
	}
	detail, err := h.boards.Get(r.Context(), boardID, c.actorID, c.role)
	h.respondDetail(w, detail, err)
}

func (h *Handler) respondDetail(w http.ResponseWriter, detail *port.BoardDetail, err error) {
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDetailDTO(detail))
}

func (h *Handler) updateBoard(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, ok := callerOf(w, r)
	if !ok {
		return
	}
	var body updateBoardRequest
	if err := decodeStrict(r, &body); err != nil {
		httpresponse.Error(w, err)
		return
	}
	in := port.UpdateBoardInput{Name: body.Name, ShowUnmappedLane: body.ShowUnmappedLane}
	if body.SwimlaneMode != nil {
		mode := domain.SwimlaneMode(*body.SwimlaneMode)
		in.SwimlaneMode = &mode
	}
	if body.Estimation != nil {
		est := domain.Estimation(*body.Estimation)
		in.Estimation = &est
	}
	if body.CardFields != nil {
		in.CardFields, in.CardFieldsSet = *body.CardFields, true
	}
	if body.DoneRetentionDays != nil {
		in.DoneRetentionSet = true
		if string(body.DoneRetentionDays) != "null" {
			var days int
			if err := json.Unmarshal(body.DoneRetentionDays, &days); err != nil {
				httpresponse.Error(w, apperror.New(apperror.KindValidation, "cuerpo de la petición no válido"))
				return
			}
			in.DoneRetentionDays = &days
		}
	}
	detail, err := h.boards.Update(r.Context(), boardID, c.actorID, c.role, in)
	h.respondDetail(w, detail, err)
}

func (h *Handler) deleteBoard(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, ok := callerOf(w, r)
	if !ok {
		return
	}
	if err := h.boards.Delete(r.Context(), boardID, c.actorID, c.role); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) replaceColumns(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, ok := callerOf(w, r)
	if !ok {
		return
	}
	var body struct {
		Columns []columnInput `json:"columns"`
	}
	if err := decodeStrict(r, &body); err != nil {
		httpresponse.Error(w, err)
		return
	}
	specs := make([]domain.ColumnSpec, 0, len(body.Columns))
	for _, col := range body.Columns {
		specs = append(specs, col.spec())
	}
	detail, err := h.boards.ReplaceColumns(r.Context(), boardID, c.actorID, c.role, specs)
	h.respondDetail(w, detail, err)
}

func (h *Handler) replaceQuickFilters(w http.ResponseWriter, r *http.Request) {
	h.replaceQueries(w, r, func(r *http.Request, id uuid.UUID, c caller, in []port.QueryInput) (*port.BoardDetail, error) {
		return h.boards.ReplaceQuickFilters(r.Context(), id, c.actorID, c.role, in)
	}, "quick_filters")
}

func (h *Handler) replaceSwimlanes(w http.ResponseWriter, r *http.Request) {
	h.replaceQueries(w, r, func(r *http.Request, id uuid.UUID, c caller, in []port.QueryInput) (*port.BoardDetail, error) {
		return h.boards.ReplaceSwimlanes(r.Context(), id, c.actorID, c.role, in)
	}, "swimlanes")
}

func (h *Handler) replaceQueries(w http.ResponseWriter, r *http.Request, call func(*http.Request, uuid.UUID, caller, []port.QueryInput) (*port.BoardDetail, error), key string) {
	boardID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, ok := callerOf(w, r)
	if !ok {
		return
	}
	var raw map[string]json.RawMessage
	if err := decodeStrict(r, &raw); err != nil || len(raw) != 1 || raw[key] == nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "cuerpo de la petición no válido"))
		return
	}
	var items []queryInput
	if err := json.Unmarshal(raw[key], &items); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "cuerpo de la petición no válido"))
		return
	}
	in := make([]port.QueryInput, 0, len(items))
	for _, item := range items {
		in = append(in, port.QueryInput{Name: item.Name, Query: item.Query})
	}
	detail, err := call(r, boardID, c, in)
	h.respondDetail(w, detail, err)
}

func (h *Handler) viewBoard(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, ok := callerOf(w, r)
	if !ok {
		return
	}
	query := port.ViewQuery{Swimlane: r.URL.Query().Get("swimlane")}
	if raw := r.URL.Query().Get("quick_filters"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			id, err := uuid.Parse(strings.TrimSpace(part))
			if err != nil {
				httpresponse.Error(w, apperror.New(apperror.KindValidation, "el filtro rápido no es válido"))
				return
			}
			query.QuickFilterIDs = append(query.QuickFilterIDs, id)
		}
	}
	switch sprint := r.URL.Query().Get("sprint"); sprint {
	case "":
	case "active", "all", "none":
		query.Sprint.Mode = sprint
	default:
		id, err := uuid.Parse(sprint)
		if err != nil {
			httpresponse.Error(w, apperror.New(apperror.KindValidation, "sprint debe ser active, all, none o un identificador"))
			return
		}
		query.Sprint = port.SprintSelector{Mode: "id", ID: id}
	}
	view, err := h.boards.View(r.Context(), boardID, c.actorID, c.role, query)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toViewDTO(view))
}

type moveBoardRequest struct {
	ColumnID      uuid.UUID `json:"column_id"`
	BeforeIssueID uuid.UUID `json:"before_issue_id"`
	AfterIssueID  uuid.UUID `json:"after_issue_id"`
	Swimlane      string    `json:"swimlane"`
	TransitionID  string    `json:"transition_id"`
	Input         struct {
		Fields       map[string]string `json:"fields"`
		Comment      string            `json:"comment"`
		ResolutionID uuid.UUID         `json:"resolution_id"`
		AssigneeID   uuid.UUID         `json:"assignee_id"`
	} `json:"input"`
}

type moveResultDTO struct {
	IssueID      uuid.UUID         `json:"issue_id"`
	ColumnID     uuid.UUID         `json:"column_id"`
	Status       workflowStatusDTO `json:"status"`
	TransitionID string            `json:"transition_id"`
	Transitioned bool              `json:"transitioned"`
	Rank         string            `json:"rank"`
	WIP          wipDTO            `json:"wip"`
}

type wipDTO struct {
	ColumnID  uuid.UUID `json:"column_id"`
	Count     int       `json:"count"`
	MinIssues *int      `json:"min_issues"`
	MaxIssues *int      `json:"max_issues"`
	State     string    `json:"state"`
	Exceeded  bool      `json:"exceeded"`
}

func (h *Handler) moveBoardCard(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	issueID, ok := pathID(w, r, "issue_id")
	if !ok {
		return
	}
	c, ok := callerOf(w, r)
	if !ok {
		return
	}
	var body moveBoardRequest
	if err := decodeStrict(r, &body); err != nil || body.ColumnID == uuid.Nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "cuerpo de la petición no válido"))
		return
	}
	result, err := h.boards.MoveCard(r.Context(), boardID, c.actorID, c.role, port.MoveInput{
		IssueID: issueID, ColumnID: body.ColumnID, BeforeIssueID: body.BeforeIssueID, AfterIssueID: body.AfterIssueID,
		Swimlane: body.Swimlane, TransitionID: body.TransitionID,
		Input: port.TransitionInput{Fields: body.Input.Fields, Comment: body.Input.Comment, ResolutionID: body.Input.ResolutionID, AssigneeID: body.Input.AssigneeID},
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, moveResultDTO{
		IssueID: result.IssueID, ColumnID: result.ColumnID,
		Status:       workflowStatusDTO{ID: result.Status.ID, Name: result.Status.Name, Category: result.Status.Category},
		TransitionID: result.TransitionID, Transitioned: result.Transitioned, Rank: result.Rank,
		WIP: wipDTO{ColumnID: result.WIP.ColumnID, Count: result.WIP.Count, MinIssues: result.WIP.Min, MaxIssues: result.WIP.Max,
			State: string(result.WIP.State), Exceeded: result.WIP.Exceeded},
	})
}

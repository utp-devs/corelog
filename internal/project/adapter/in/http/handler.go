package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/validation"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

const (
	defaultListPageSize = 25
	maxListPageSize     = 100
)

type Handler struct {
	service     port.Service
	manager     port.ProjectManager
	jwtSecret   string
	permissions permissionService
}

func NewHandler(service port.Service, manager port.ProjectManager, jwtSecret string) *Handler {
	return &Handler{service: service, manager: manager, jwtSecret: jwtSecret}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	orgID, err := uuid.Parse(req.OrganizationID)
	if err != nil {
		httpresponse.Error(w, organizationRequiredError())
		return
	}
	actorID, _, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	template, err := templateFromRequest(req)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	memberIDs, err := parseUUIDList(req.MemberIDs)
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid member id"))
		return
	}
	teamIDs, err := parseUUIDList(req.TeamIDs)
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid team id"))
		return
	}
	project, err := h.manager.CreateProject(r.Context(), actorID, orgID, port.CreateInput{
		Key:                   req.Key,
		Name:                  req.Name,
		Description:           req.Description,
		Template:              template,
		Visibility:            domain.Visibility(req.Visibility),
		Color:                 req.Color,
		Icon:                  req.Icon,
		DefaultAssigneePolicy: domain.AssigneePolicy(req.DefaultAssigneePolicy),
		MemberIDs:             memberIDs,
		TeamIDs:               teamIDs,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toProjectResponse(project))
}

// List requiere el parametro organization_id; solo devuelve proyectos de esa organizacion visibles para el actor.
// Responde {data, meta} con la pagina y el total de coincidencias.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(r.URL.Query().Get("organization_id"))
	if err != nil {
		httpresponse.Error(w, organizationRequiredError())
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	filter, err := listFilterFromQuery(r.URL.Query())
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	page, err := h.manager.ListProjects(r.Context(), actorID, globalRole, orgID, filter)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.Paginated(w, http.StatusOK, toProjectResponses(page.Items), httpresponse.PageMeta{
		Page: filter.Page, PageSize: filter.PageSize, Total: page.Total,
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	id, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	project, err := h.service.Get(r.Context(), actorID, globalRole, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toProjectResponse(project))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req updateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	leadID, err := uuid.Parse(req.LeadID)
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid lead id"))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	id, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	project, err := h.service.Update(r.Context(), actorID, globalRole, id, domain.UpdateInput{
		Name: req.Name, Description: req.Description, Type: domain.ProjectType(req.Type),
		LeadID: leadID, Visibility: domain.Visibility(req.Visibility),
		Color: req.Color, Icon: req.Icon, DefaultAssigneePolicy: domain.AssigneePolicy(req.DefaultAssigneePolicy),
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toProjectResponse(project))
}

// Delete soft deletes the project. The body must carry {"confirmation": "<exact project name>"}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	id, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	var req deleteProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// A missing or malformed body is a missing confirmation.
		httpresponse.Error(w, domain.ErrConfirmationMismatch)
		return
	}
	if err := h.manager.DeleteProject(r.Context(), actorID, globalRole, id, req.Confirmation); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	h.lifecycle(w, r, h.manager.Archive)
}

func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	h.lifecycle(w, r, h.manager.Restore)
}

// lifecycle runs an archive or restore operation on the project named by {id}.
func (h *Handler) lifecycle(w http.ResponseWriter, r *http.Request, op func(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Project, error)) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	id, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	project, err := op(r.Context(), actorID, globalRole, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toProjectResponse(project))
}

// KeyAvailable reports whether a key is valid and free in the organization given by organization_id.
func (h *Handler) KeyAvailable(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(r.URL.Query().Get("organization_id"))
	if err != nil {
		httpresponse.Error(w, organizationRequiredError())
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	key, err := domain.ValidateKey(r.URL.Query().Get("key"))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	available, err := h.manager.KeyAvailable(r.Context(), actorID, globalRole, orgID, key)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, projectKeyAvailabilityResponse{Available: available, Key: key})
}

// projectIDFromPath resolves the {id} segment. A UUID is used as is. A project key is looked up in
// the organization named by organization_id, among the projects the actor can see. It writes the
// error response and returns false when the segment cannot be resolved.
func (h *Handler) projectIDFromPath(w http.ResponseWriter, r *http.Request, actorID uuid.UUID, globalRole string) (uuid.UUID, bool) {
	segment := chi.URLParam(r, "id")
	if id, err := uuid.Parse(segment); err == nil {
		return id, true
	}
	orgID, ok := optionalOrganizationID(w, r)
	if !ok {
		return uuid.Nil, false
	}
	project, err := h.manager.FindByKey(r.Context(), actorID, globalRole, orgID, segment)
	if err != nil {
		httpresponse.Error(w, err)
		return uuid.Nil, false
	}
	return project.ID, true
}

// optionalOrganizationID reads organization_id when present. An empty value means no organization.
// A value that is not a UUID writes the error response and returns false.
func optionalOrganizationID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	value := r.URL.Query().Get("organization_id")
	if value == "" {
		return uuid.Nil, true
	}
	orgID, err := uuid.Parse(value)
	if err != nil {
		httpresponse.Error(w, organizationRequiredError())
		return uuid.Nil, false
	}
	return orgID, true
}

// templateFromRequest reads template, or derives it from the legacy type when template is absent.
func templateFromRequest(req createProjectRequest) (domain.Template, error) {
	if strings.TrimSpace(req.Template) != "" {
		return domain.ParseTemplate(req.Template)
	}
	if req.Type != "" {
		return domain.TemplateFromProjectType(domain.ProjectType(req.Type)), nil
	}
	return "", domain.ErrInvalidTemplate
}

// listFilterFromQuery builds the listing filter. Page and page size are clamped here so the meta
// of the response describes the page the use case returns.
func listFilterFromQuery(query url.Values) (port.ProjectFilter, error) {
	filter := port.ProjectFilter{
		Q:        strings.TrimSpace(query.Get("q")),
		Template: domain.Template(strings.TrimSpace(query.Get("template"))),
		Archived: port.ArchiveFilter(query.Get("archived")),
		Sort:     port.SortField(query.Get("sort")),
	}
	// The legacy type filter: scrum matches the scrum template, kanban matches every other template.
	if legacy := query.Get("type"); legacy != "" {
		projectType := domain.ProjectType(legacy)
		if projectType != domain.ProjectTypeKanban && projectType != domain.ProjectTypeScrum {
			return port.ProjectFilter{}, domain.ErrInvalidProject
		}
		filter.Type = projectType
	}
	if value := query.Get("lead_id"); value != "" {
		leadID, err := uuid.Parse(value)
		if err != nil {
			return port.ProjectFilter{}, apperror.New(apperror.KindValidation, "invalid lead id")
		}
		filter.LeadID = &leadID
	}
	var err error
	if filter.Starred, err = boolQuery(query, "starred"); err != nil {
		return port.ProjectFilter{}, err
	}
	if filter.Desc, err = boolQuery(query, "desc"); err != nil {
		return port.ProjectFilter{}, err
	}
	if filter.Page, err = intQuery(query, "page"); err != nil {
		return port.ProjectFilter{}, err
	}
	if filter.PageSize, err = intQuery(query, "page_size"); err != nil {
		return port.ProjectFilter{}, err
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	switch {
	case filter.PageSize <= 0:
		filter.PageSize = defaultListPageSize
	case filter.PageSize > maxListPageSize:
		filter.PageSize = maxListPageSize
	}
	return filter, nil
}

func boolQuery(query url.Values, name string) (bool, error) {
	value := query.Get(name)
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, apperror.New(apperror.KindValidation, name+" debe ser true o false")
	}
	return parsed, nil
}

func intQuery(query url.Values, name string) (int, error) {
	value := query.Get(name)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, apperror.New(apperror.KindValidation, name+" debe ser un numero entero")
	}
	return parsed, nil
}

func parseUUIDList(values []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func (h *Handler) ListLabels(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	labels, err := h.service.ListLabels(r.Context(), actorID, globalRole, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toLabelResponses(labels))
}

func (h *Handler) CreateLabel(w http.ResponseWriter, r *http.Request) {
	var req labelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	label, err := h.service.CreateLabel(r.Context(), actorID, globalRole, projectID, req.Name, req.Color)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toLabelResponse(*label))
}

func (h *Handler) DeleteLabel(w http.ResponseWriter, r *http.Request) {
	labelID, err := uuid.Parse(chi.URLParam(r, "label_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid label id"))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	if err := h.service.DeleteLabel(r.Context(), actorID, globalRole, projectID, labelID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) UpdateLabel(w http.ResponseWriter, r *http.Request) {
	labelID, err := uuid.Parse(chi.URLParam(r, "label_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid label id"))
		return
	}
	var req labelUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	label, err := h.service.UpdateLabel(r.Context(), actorID, globalRole, projectID, labelID, req.Name, req.Color)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toLabelResponse(*label))
}

// organizationRequiredError is returned when organization_id is missing or is not a UUID.
func organizationRequiredError() error {
	return apperror.WithCode(apperror.KindValidation, "project.organization_required", "organization_id es obligatorio y debe ser un UUID valido")
}

func actorFromRequest(r *http.Request) (uuid.UUID, string, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		return uuid.Nil, "", false
	}
	role, ok := appmw.RoleFromContext(r.Context())
	if !ok {
		return uuid.Nil, "", false
	}
	return actorID, role, true
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	members, err := h.service.ListMembers(r.Context(), actorID, globalRole, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toProjectMemberEntryResponses(members))
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	var req projectMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid user id"))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	member, err := h.service.AddMember(r.Context(), actorID, globalRole, projectID, userID, req.Role)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toProjectMemberResponse(*member))
}

func (h *Handler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid user id"))
		return
	}
	var req updateProjectMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	member, err := h.service.UpdateMemberRole(r.Context(), actorID, globalRole, projectID, userID, req.Role)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toProjectMemberResponse(*member))
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "user_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid user id"))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	if err := h.service.RemoveMember(r.Context(), actorID, globalRole, projectID, userID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

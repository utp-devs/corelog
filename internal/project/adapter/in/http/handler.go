package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/validation"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

type Handler struct {
	service   port.Service
	jwtSecret string
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
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
	leadID, err := uuid.Parse(req.LeadID)
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid lead id"))
		return
	}
	project, err := h.service.Create(r.Context(), req.Key, req.Name, req.Description, domain.ProjectType(req.Type), leadID, domain.Visibility(req.Visibility))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toProjectResponse(project))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	projects, err := h.service.List(r.Context(), actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toProjectResponses(projects))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
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
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
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
	project, err := h.service.Update(r.Context(), actorID, globalRole, id, domain.UpdateInput{
		Name: req.Name, Description: req.Description, Type: domain.ProjectType(req.Type),
		LeadID: leadID, Visibility: domain.Visibility(req.Visibility),
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toProjectResponse(project))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	if err := h.service.Delete(r.Context(), actorID, globalRole, id); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) ListLabels(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	labels, err := h.service.ListLabels(r.Context(), projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toLabelResponses(labels))
}

func (h *Handler) CreateLabel(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	var req labelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	label, err := h.service.CreateLabel(r.Context(), projectID, req.Name, req.Color)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toLabelResponse(*label))
}

func (h *Handler) DeleteLabel(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	labelID, err := uuid.Parse(chi.URLParam(r, "label_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid label id"))
		return
	}
	if err := h.service.DeleteLabel(r.Context(), projectID, labelID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) ListComponents(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	components, err := h.service.ListComponents(r.Context(), projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toComponentResponses(components))
}

func (h *Handler) CreateComponent(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	var req componentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	component, err := h.service.CreateComponent(r.Context(), projectID, req.Name, req.Description)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toComponentResponse(*component))
}

func (h *Handler) DeleteComponent(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	componentID, err := uuid.Parse(chi.URLParam(r, "component_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid component id"))
		return
	}
	if err := h.service.DeleteComponent(r.Context(), projectID, componentID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
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
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	members, err := h.service.ListMembers(r.Context(), actorID, globalRole, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toProjectMemberResponses(members))
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
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
	member, err := h.service.AddMember(r.Context(), actorID, globalRole, projectID, userID, req.Role)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toProjectMemberResponse(*member))
}

func (h *Handler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
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
	member, err := h.service.UpdateMemberRole(r.Context(), actorID, globalRole, projectID, userID, req.Role)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toProjectMemberResponse(*member))
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
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
	if err := h.service.RemoveMember(r.Context(), actorID, globalRole, projectID, userID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

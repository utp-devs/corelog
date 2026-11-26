package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	"github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/validation"
	"github.com/jhonsferg/corelog/internal/team/application/port"
	"github.com/jhonsferg/corelog/internal/team/domain"
)

type Handler struct {
	service   port.Service
	jwtSecret string
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

func actorFrom(r *http.Request) (uuid.UUID, error) {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return uuid.Nil, apperror.New(apperror.KindUnauthorized, "autenticación requerida")
	}
	return id, nil
}

func pathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperror.WithCode(apperror.KindValidation, "team.id_invalid", "identificador no válido")
	}
	return id, nil
}

// decodeStrict decodes the body into dst, rejecting unknown fields, then validates it.
func decodeStrict(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperror.New(apperror.KindValidation, "cuerpo de la petición no válido")
	}
	if err := validation.Struct(dst); err != nil {
		return validation.Failed(dst, err)
	}
	return nil
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req createTeamRequest
	if err := decodeStrict(r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}
	orgID, err := uuid.Parse(req.OrganizationID)
	if err != nil {
		httpresponse.Error(w, domain.ErrOrganizationRequired)
		return
	}

	team, err := h.service.Create(r.Context(), actor, port.CreateInput{
		OrganizationID: orgID,
		Name:           req.Name,
		Description:    req.Description,
		AvatarColor:    req.AvatarColor,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toTeamResponse(team))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	orgID := uuid.Nil
	if raw := strings.TrimSpace(r.URL.Query().Get("organization_id")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			httpresponse.Error(w, domain.ErrOrganizationRequired)
			return
		}
		orgID = parsed
	}

	memberID := uuid.Nil
	if raw := strings.TrimSpace(r.URL.Query().Get("member_id")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			httpresponse.Error(w, domain.ErrMemberIDInvalid)
			return
		}
		memberID = parsed
	}

	teams, err := h.service.List(r.Context(), actor, orgID, memberID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toTeamResponses(teams))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	id, err := pathUUID(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	team, err := h.service.Get(r.Context(), actor, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toTeamResponse(team))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	id, err := pathUUID(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req updateTeamRequest
	if err := decodeStrict(r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}

	team, err := h.service.Update(r.Context(), actor, id, domain.TeamPatch{
		Name:        req.Name,
		Description: req.Description,
		AvatarColor: req.AvatarColor,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toTeamResponse(team))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	id, err := pathUUID(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if err := h.service.Delete(r.Context(), actor, id); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	teamID, err := pathUUID(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	members, err := h.service.ListMembers(r.Context(), actor, teamID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toMemberResponses(members))
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	teamID, err := pathUUID(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req addMemberRequest
	if err := decodeStrict(r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "team.user_id_invalid", "user_id no válido"))
		return
	}

	member, err := h.service.AddMember(r.Context(), actor, teamID, userID, req.Role)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toMemberResponse(member))
}

func (h *Handler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	teamID, err := pathUUID(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	userID, err := pathUUID(r, "user_id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req updateMemberRequest
	if err := decodeStrict(r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}

	member, err := h.service.UpdateMemberRole(r.Context(), actor, teamID, userID, req.Role)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toMemberResponse(member))
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	teamID, err := pathUUID(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	userID, err := pathUUID(r, "user_id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if err := h.service.RemoveMember(r.Context(), actor, teamID, userID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

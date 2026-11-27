package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/favorite/application/port"
	"github.com/jhonsferg/corelog/internal/favorite/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

type Handler struct {
	service   port.Service
	jwtSecret string
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/favorites", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.ListFavorites)
		r.Put("/{type}/{id}", h.Star)
		r.Delete("/{type}/{id}", h.Unstar)
	})
	r.Route("/recent", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.ListRecent)
	})
}

type entryResponse struct {
	Type      string  `json:"type"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Key       string  `json:"key"`
	Color     string  `json:"color"`
	CreatedAt string  `json:"created_at"`
	ViewedAt  *string `json:"viewed_at,omitempty"`
}

// Star handles PUT /favorites/{type}/{id}. Idempotent: answers 204 whether or not the star existed.
func (h *Handler) Star(w http.ResponseWriter, r *http.Request) {
	userID, entityType, entityID, ok := h.targetContext(w, r)
	if !ok {
		return
	}
	if err := h.service.Star(r.Context(), userID, entityType, entityID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Unstar handles DELETE /favorites/{type}/{id}. Idempotent: answers 204 whether or not the star existed.
func (h *Handler) Unstar(w http.ResponseWriter, r *http.Request) {
	userID, entityType, entityID, ok := h.targetContext(w, r)
	if !ok {
		return
	}
	if err := h.service.Unstar(r.Context(), userID, entityType, entityID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListFavorites handles GET /favorites?type=.
func (h *Handler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUser(w, r)
	if !ok {
		return
	}
	entries, err := h.service.ListFavorites(r.Context(), userID, r.URL.Query().Get("type"))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toEntryResponses(entries))
}

// ListRecent handles GET /recent?type=&limit=.
func (h *Handler) ListRecent(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUser(w, r)
	if !ok {
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			httpresponse.Error(w, domain.ErrInvalidLimit)
			return
		}
		limit = parsed
	}
	entries, err := h.service.ListRecent(r.Context(), userID, r.URL.Query().Get("type"), limit)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toEntryResponses(entries))
}

func currentUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return uuid.Nil, false
	}
	return userID, true
}

// targetContext reads the actor, the entity type and the entity id from the route.
func (h *Handler) targetContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, uuid.UUID, bool) {
	userID, ok := currentUser(w, r)
	if !ok {
		return uuid.Nil, "", uuid.Nil, false
	}
	entityID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, domain.ErrInvalidID)
		return uuid.Nil, "", uuid.Nil, false
	}
	return userID, chi.URLParam(r, "type"), entityID, true
}

func toEntryResponses(entries []port.Entry) []entryResponse {
	response := make([]entryResponse, 0, len(entries))
	for _, entry := range entries {
		item := entryResponse{
			Type: string(entry.Type), ID: entry.ID.String(), Name: entry.Name, Key: entry.Key,
			Color: entry.Color, CreatedAt: entry.CreatedAt.Format(time.RFC3339),
		}
		if entry.ViewedAt != nil {
			viewed := entry.ViewedAt.Format(time.RFC3339)
			item.ViewedAt = &viewed
		}
		response = append(response, item)
	}
	return response
}

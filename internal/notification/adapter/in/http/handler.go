package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
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
	r.Route("/notifications", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.List)
		r.Get("/unread-count", h.UnreadCount)
		r.Post("/read-all", h.MarkAllRead)
		r.Post("/{id}/read", h.MarkRead)
	})
}

type notificationResponse struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Message   string `json:"message"`
	Link      string `json:"link"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"created_at"`
}

func (h *Handler) actor(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return uuid.Nil, false
	}
	return userID, true
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.actor(w, r)
	if !ok {
		return
	}
	items, err := h.service.List(r.Context(), userID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := make([]notificationResponse, 0, len(items))
	for _, item := range items {
		response = append(response, notificationResponse{
			ID: item.ID.String(), Kind: item.Kind, Message: item.Message, Link: item.Link,
			Read: item.ReadAt != nil, CreatedAt: item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

func (h *Handler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.actor(w, r)
	if !ok {
		return
	}
	count, err := h.service.UnreadCount(r.Context(), userID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, map[string]int{"count": count})
}

func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.actor(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid notification id"))
		return
	}
	if err := h.service.MarkRead(r.Context(), userID, id); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.actor(w, r)
	if !ok {
		return
	}
	if err := h.service.MarkAllRead(r.Context(), userID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

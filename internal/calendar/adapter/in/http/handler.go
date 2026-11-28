package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

const dateLayout = "2006-01-02"

type Handler struct {
	service   port.Service
	jwtSecret string
	events    port.EventService
	publicURL string
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/calendar", func(r chi.Router) {
		if h.events != nil {
			h.registerEventRoutes(r)
		}
		r.Group(func(r chi.Router) {
			r.Use(appmw.Authenticate(h.jwtSecret))
			r.Get("/events", h.Events)
		})
	})
	h.RegisterMeRoutes(r)
}

type eventResponse struct {
	Kind     string `json:"kind"`
	Date     string `json:"date"`
	Title    string `json:"title"`
	SourceID string `json:"source_id"`
	Link     string `json:"link"`
}

// Events devuelve los hitos del proyecto entre from y to (AAAA-MM-DD, ambos incluidos).
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	projectID, err := uuid.Parse(r.URL.Query().Get("project_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	from, err := time.Parse(dateLayout, r.URL.Query().Get("from"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "from must use the format YYYY-MM-DD"))
		return
	}
	to, err := time.Parse(dateLayout, r.URL.Query().Get("to"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "to must use the format YYYY-MM-DD"))
		return
	}
	events, err := h.service.Events(r.Context(), actorID, globalRole, projectID, from, to)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := make([]eventResponse, 0, len(events))
	for _, event := range events {
		response = append(response, eventResponse{
			Kind: event.Kind, Date: event.Date.Format(dateLayout), Title: event.Title,
			SourceID: event.SourceID.String(), Link: event.Link,
		})
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

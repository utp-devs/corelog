package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/search/application/port"
)

type Handler struct {
	service   port.Service
	jwtSecret string
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/search", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.Search)
	})
}

type hitResponse struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Link     string `json:"link"`
}

// Search busca proyectos e incidencias visibles para el usuario. q tiene entre 2 y 100 caracteres.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid limit"))
			return
		}
		limit = parsed
	}
	hits, err := h.service.Search(r.Context(), actorID, globalRole, r.URL.Query().Get("q"), limit)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := make([]hitResponse, 0, len(hits))
	for _, hit := range hits {
		response = append(response, hitResponse{Kind: string(hit.Kind), ID: hit.ID, Title: hit.Title, Subtitle: hit.Subtitle, Link: hit.Link})
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

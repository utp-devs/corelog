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
	global    port.GlobalService
	issues    port.IssueSearchService
	filters   port.FilterService
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	auth := appmw.Authenticate(h.jwtSecret)
	r.Route("/search", func(r chi.Router) {
		r.Use(auth)
		r.Get("/", h.Search)
		if h.issues != nil {
			r.Get("/issues", h.SearchIssues)
			r.Post("/issues", h.SearchIssues)
			r.Post("/validate", h.Validate)
			r.Get("/suggest", h.Suggest)
		}
	})
	if h.issues != nil {
		r.With(auth).Get("/issues/search", h.SearchIssues)
		r.With(auth).Post("/issues/search", h.SearchIssues)
	}
	if h.filters != nil {
		r.Route("/filters", func(r chi.Router) {
			r.Use(auth)
			r.Get("/", h.ListFilters)
			r.Post("/", h.CreateFilter)
			r.Get("/{id}", h.GetFilter)
			r.Patch("/{id}", h.UpdateFilter)
			r.Delete("/{id}", h.DeleteFilter)
			r.Put("/{id}/shares", h.SetFilterShares)
			r.Get("/{id}/issues", h.RunFilter)
			r.Get("/{id}/subscription", h.GetSubscription)
			r.Post("/{id}/subscription", h.Subscribe)
			r.Delete("/{id}/subscription", h.Unsubscribe)
		})
	}
}

// WithGlobal enables the global search v2 (more kinds, highlights).
func (h *Handler) WithGlobal(service port.GlobalService) *Handler {
	h.global = service
	return h
}

// WithIssueSearch enables the CoreQL issue search endpoints.
func (h *Handler) WithIssueSearch(service port.IssueSearchService) *Handler {
	h.issues = service
	return h
}

// WithFilters enables the saved filter endpoints.
func (h *Handler) WithFilters(service port.FilterService) *Handler {
	h.filters = service
	return h
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
	if h.global != nil {
		h.searchGlobal(w, r, viewerOf(r), limit)
		return
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

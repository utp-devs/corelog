package http

import (
	"github.com/go-chi/chi/v5"

	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// RegisterRoutes mounts the team routes. Every route requires authentication;
// authorization (organization and team roles) is decided in the use cases.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/teams", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))

		r.Get("/", h.List)
		r.Post("/", h.Create)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.Get)
			r.Patch("/", h.Update)
			r.Delete("/", h.Delete)

			r.Get("/members", h.ListMembers)
			r.Post("/members", h.AddMember)
			r.Patch("/members/{user_id}", h.UpdateMember)
			r.Delete("/members/{user_id}", h.RemoveMember)
		})
	})
}

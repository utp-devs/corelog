package http

import (
	"github.com/go-chi/chi/v5"

	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// RegisterRoutes mounts the organization routes. Only the invitation preview is public. Every other
// route requires a JWT; membership and capabilities are decided in the use cases (404 for non-members).
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Public: anyone holding the token may preview it. No side effects.
	r.Get("/invitations/{token}", h.PreviewInvitation)
	r.With(appmw.Authenticate(h.jwtSecret)).Post("/invitations/{token}/accept", h.AcceptInvitation)

	r.Route("/organizations", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))

		r.Get("/", h.List)
		r.Post("/", h.Create)

		r.Route("/{org}", func(r chi.Router) {
			r.Get("/", h.Get)
			r.Patch("/", h.Update)
			r.Delete("/", h.Delete)
			r.Post("/leave", h.Leave)
			r.Post("/transfer-ownership", h.TransferOwnership)

			r.Get("/members", h.ListMembers)
			r.Get("/people/{user_id}", h.GetPerson)
			r.Patch("/members/{user_id}", h.ChangeMemberRole)
			r.Delete("/members/{user_id}", h.RemoveMember)

			r.Get("/invitations", h.ListInvitations)
			r.Post("/invitations", h.CreateInvitations)
			r.Post("/invitations/{id}/resend", h.ResendInvitation)
			r.Delete("/invitations/{id}", h.RevokeInvitation)
		})
	})
}

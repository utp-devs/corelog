package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/dashboard/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

const maxBodyBytes = 512 << 10

// WithDashboards enables the dashboard, gadget and share endpoints.
func (h *Handler) WithDashboards(svc port.DashboardService) *Handler {
	h.dashboards = svc
	return h
}

func (h *Handler) registerDashboardRoutes(r chi.Router) {
	r.Route("/dashboards", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.ListDashboards)
		r.Post("/", h.CreateDashboard)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetDashboard)
			r.Patch("/", h.UpdateDashboard)
			r.Delete("/", h.DeleteDashboard)
			r.Put("/gadgets", h.ReplaceGadgets)
			r.Put("/shares", h.SetShares)
			r.Post("/duplicate", h.DuplicateDashboard)
			r.Get("/gadgets/{gid}/data", h.GadgetData)
		})
	})
	r.Route("/me/dashboard", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.DefaultDashboard)
	})
}

func validationError(message string) error {
	return apperror.WithCode(apperror.KindValidation, "validation.failed", message)
}

func decodeStrict(w http.ResponseWriter, r *http.Request, dst any, allowEmpty bool) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if allowEmpty && err == io.EOF {
			return true
		}
		httpresponse.Error(w, validationError("el cuerpo de la petición no es válido"))
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
		httpresponse.Error(w, validationError("el cuerpo de la petición no es válido"))
		return false
	}
	return true
}

func viewerOf(w http.ResponseWriter, r *http.Request) (port.Viewer, bool) {
	id, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return port.Viewer{}, false
	}
	role, _ := appmw.RoleFromContext(r.Context())
	return port.Viewer{UserID: id, PlatformAdmin: role == "admin"}, true
}

func idParam(w http.ResponseWriter, r *http.Request, name string, notFound error) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		httpresponse.Error(w, notFound)
		return uuid.Nil, false
	}
	return id, true
}

type ownerResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type gadgetResponse struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Config      json.RawMessage `json:"config"`
	Position    int             `json:"position"`
	ColumnIndex int             `json:"column_index"`
}

type shareResponse struct {
	Type       string `json:"type"`
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
	Permission string `json:"permission"`
}

type dashboardResponse struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Layout         string            `json:"layout"`
	IsDefault      bool              `json:"is_default"`
	Owner          ownerResponse     `json:"owner"`
	OrganizationID string            `json:"organization_id"`
	Access         string            `json:"access"`
	CanEdit        bool              `json:"can_edit"`
	CanManage      bool              `json:"can_manage"`
	IsFavorite     bool              `json:"is_favorite"`
	Shared         bool              `json:"shared"`
	GadgetCount    int               `json:"gadget_count"`
	Gadgets        *[]gadgetResponse `json:"gadgets,omitempty"`
	Shares         *[]shareResponse  `json:"shares,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

func toResponse(v port.DashboardView, withGadgets bool) dashboardResponse {
	out := dashboardResponse{
		ID: v.ID.String(), Name: v.Name, Description: v.Description, Layout: string(v.Layout), IsDefault: v.IsDefault,
		Owner: ownerResponse{ID: v.OwnerID.String(), Name: v.OwnerName}, OrganizationID: v.OrganizationID.String(),
		Access: v.Access.String(), CanEdit: v.Access.CanEdit(), CanManage: v.Access.CanManage(), IsFavorite: v.IsFavorite, Shared: v.Shared,
		GadgetCount: len(v.Gadgets), CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
	if withGadgets {
		gadgets := make([]gadgetResponse, 0, len(v.Gadgets))
		for _, g := range v.Gadgets {
			config := g.Config
			if len(config) == 0 {
				config = json.RawMessage(`{}`)
			}
			gadgets = append(gadgets, gadgetResponse{ID: g.ID.String(), Type: g.Type, Title: g.Title, Config: config, Position: g.Position, ColumnIndex: g.ColumnIndex})
		}
		out.Gadgets = &gadgets
	}
	return out
}

func toDetail(d *port.DashboardDetail) dashboardResponse {
	out := toResponse(d.DashboardView, true)
	if d.Access.CanManage() {
		shares := make([]shareResponse, 0, len(d.Shares))
		for _, s := range d.Shares {
			shares = append(shares, shareResponse{Type: string(s.Type), TargetID: s.TargetID.String(), TargetName: s.TargetName, Permission: string(s.Permission)})
		}
		out.Shares = &shares
	}
	return out
}

func intParam(r *http.Request, name string) int {
	n, _ := strconv.Atoi(r.URL.Query().Get(name))
	return n
}

// ListDashboards lists the dashboards the caller can see: scope=mine|shared|all, q, page, page_size.
func (h *Handler) ListDashboards(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	q := port.ListQuery{Scope: r.URL.Query().Get("scope"), Query: r.URL.Query().Get("q"), Page: intParam(r, "page"), PageSize: intParam(r, "page_size")}
	views, total, err := h.dashboards.List(r.Context(), viewer, q)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]dashboardResponse, len(views))
	for i, v := range views {
		out[i] = toResponse(v, false)
	}
	page, size := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	httpresponse.Paginated(w, http.StatusOK, out, httpresponse.PageMeta{Page: page, PageSize: size, Total: total})
}

type createRequest struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Layout         string  `json:"layout"`
	OrganizationID *string `json:"organization_id"`
}

// CreateDashboard creates an empty dashboard owned by the caller.
func (h *Handler) CreateDashboard(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	var req createRequest
	if !decodeStrict(w, r, &req, false) {
		return
	}
	in := port.CreateInput{Name: req.Name, Description: req.Description, Layout: req.Layout}
	if req.OrganizationID != nil && *req.OrganizationID != "" {
		id, err := uuid.Parse(*req.OrganizationID)
		if err != nil {
			httpresponse.Error(w, validationError("organization_id no es válido"))
			return
		}
		in.OrganizationID = id
	}
	detail, err := h.dashboards.Create(r.Context(), viewer, in)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toDetail(detail))
}

// GetDashboard returns one dashboard with its gadgets (and the shares, for the owner).
func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	detail, err := h.dashboards.Get(r.Context(), viewer, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDetail(detail))
}

type updateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Layout      *string `json:"layout"`
	IsDefault   *bool   `json:"is_default"`
}

// UpdateDashboard changes the name, description, layout or default flag.
func (h *Handler) UpdateDashboard(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	var req updateRequest
	if !decodeStrict(w, r, &req, false) {
		return
	}
	detail, err := h.dashboards.Update(r.Context(), viewer, id, port.UpdateInput{Name: req.Name, Description: req.Description, Layout: req.Layout, IsDefault: req.IsDefault})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDetail(detail))
}

// DeleteDashboard deletes a dashboard; only the owner can.
func (h *Handler) DeleteDashboard(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	if err := h.dashboards.Delete(r.Context(), viewer, id); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type gadgetRequest struct {
	ID          *string         `json:"id"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Config      json.RawMessage `json:"config"`
	ColumnIndex int             `json:"column_index"`
	Position    *int            `json:"position"`
}

type replaceGadgetsRequest struct {
	Layout  *string          `json:"layout"`
	Gadgets *[]gadgetRequest `json:"gadgets"`
}

// ReplaceGadgets replaces the layout and the whole gadget list atomically.
func (h *Handler) ReplaceGadgets(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	var req replaceGadgetsRequest
	if !decodeStrict(w, r, &req, false) {
		return
	}
	if req.Gadgets == nil {
		httpresponse.Error(w, validationError("gadgets es obligatorio"))
		return
	}
	inputs := make([]port.GadgetInput, 0, len(*req.Gadgets))
	for i, g := range *req.Gadgets {
		in := port.GadgetInput{Type: g.Type, Title: g.Title, Config: g.Config, ColumnIndex: g.ColumnIndex, Position: g.Position}
		if g.ID != nil && *g.ID != "" {
			gid, err := uuid.Parse(*g.ID)
			if err != nil {
				httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidGadget, "el id del gadget no es válido").
					WithDetails(apperror.FieldError{Field: "gadgets[" + strconv.Itoa(i) + "].id", Message: "no es un id válido"}))
				return
			}
			in.ID = &gid
		}
		inputs = append(inputs, in)
	}
	detail, err := h.dashboards.ReplaceGadgets(r.Context(), viewer, id, req.Layout, inputs)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDetail(detail))
}

type shareRequest struct {
	Type       string `json:"type"`
	TargetID   string `json:"target_id"`
	Permission string `json:"permission"`
}

type setSharesRequest struct {
	Shares *[]shareRequest `json:"shares"`
}

// SetShares replaces the shares of a dashboard; only the owner can.
func (h *Handler) SetShares(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	var req setSharesRequest
	if !decodeStrict(w, r, &req, false) {
		return
	}
	if req.Shares == nil {
		httpresponse.Error(w, validationError("shares es obligatorio"))
		return
	}
	shares := make([]domain.Share, 0, len(*req.Shares))
	for i, s := range *req.Shares {
		target, err := uuid.Parse(s.TargetID)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidShare, "el destino no es un id válido").
				WithDetails(apperror.FieldError{Field: "shares[" + strconv.Itoa(i) + "].target_id", Message: "no es un id válido"}))
			return
		}
		shares = append(shares, domain.Share{Type: domain.ShareType(s.Type), TargetID: target, Permission: domain.Permission(s.Permission)})
	}
	detail, err := h.dashboards.SetShares(r.Context(), viewer, id, shares)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDetail(detail))
}

type duplicateRequest struct {
	Name string `json:"name"`
}

// DuplicateDashboard copies a visible dashboard into a new one owned by the caller.
func (h *Handler) DuplicateDashboard(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	var req duplicateRequest
	if !decodeStrict(w, r, &req, true) {
		return
	}
	detail, err := h.dashboards.Duplicate(r.Context(), viewer, id, req.Name)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toDetail(detail))
}

// DefaultDashboard returns the personal default dashboard, creating it on first use.
func (h *Handler) DefaultDashboard(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	detail, err := h.dashboards.Default(r.Context(), viewer)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDetail(detail))
}

type gadgetDataResponse struct {
	GadgetID    string    `json:"gadget_id"`
	Type        string    `json:"type"`
	GeneratedAt time.Time `json:"generated_at"`
	Data        any       `json:"data"`
}

// GadgetData returns the data of one gadget computed with the caller's permissions.
func (h *Handler) GadgetData(w http.ResponseWriter, r *http.Request) {
	viewer, ok := viewerOf(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	gid, ok := idParam(w, r, "gid", domain.ErrGadgetNotFound)
	if !ok {
		return
	}
	data, err := h.dashboards.GadgetData(r.Context(), viewer, id, gid)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=15")
	httpresponse.JSON(w, http.StatusOK, gadgetDataResponse{GadgetID: data.GadgetID.String(), Type: data.Type, GeneratedAt: data.GeneratedAt, Data: data.Data})
}

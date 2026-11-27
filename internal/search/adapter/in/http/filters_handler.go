package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

type ownerResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type shareResponse struct {
	Type       string `json:"type"`
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
	Permission string `json:"permission"`
}

type scheduleResponse struct {
	Frequency string `json:"frequency"`
	Hour      int    `json:"hour"`
	Weekday   int    `json:"weekday"`
	Timezone  string `json:"timezone"`
}

type subscriptionResponse struct {
	ID         string           `json:"id"`
	FilterID   string           `json:"filter_id"`
	Schedule   scheduleResponse `json:"schedule"`
	LastSentAt *time.Time       `json:"last_sent_at"`
	CreatedAt  time.Time        `json:"created_at"`
}

type filterResponse struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Description    string                `json:"description"`
	Query          string                `json:"query"`
	Columns        []string              `json:"columns"`
	Sort           string                `json:"sort"`
	Owner          ownerResponse         `json:"owner"`
	OrganizationID string                `json:"organization_id"`
	Access         string                `json:"access"`
	IsFavorite     bool                  `json:"is_favorite"`
	IsSubscribed   bool                  `json:"is_subscribed"`
	Subscription   *subscriptionResponse `json:"subscription,omitempty"`
	Shares         []shareResponse       `json:"shares,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

func accessName(a domain.Access) string {
	switch a {
	case domain.AccessOwner:
		return "owner"
	case domain.AccessEdit:
		return "edit"
	}
	return "view"
}

func toFilterResponse(v port.FilterView) filterResponse {
	cols := v.Columns
	if cols == nil {
		cols = []string{}
	}
	return filterResponse{
		ID: v.ID.String(), Name: v.Name, Description: v.Description, Query: v.QueryText, Columns: cols, Sort: v.Sort,
		Owner: ownerResponse{ID: v.OwnerID.String(), Name: v.OwnerName}, OrganizationID: v.OrganizationID.String(),
		Access: accessName(v.Access), IsFavorite: v.IsFavorite, IsSubscribed: v.HasSubscription,
		CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

func toSubscriptionResponse(s domain.Subscription) subscriptionResponse {
	return subscriptionResponse{
		ID: s.ID.String(), FilterID: s.FilterID.String(), LastSentAt: s.LastSentAt, CreatedAt: s.CreatedAt,
		Schedule: scheduleResponse{Frequency: string(s.Schedule.Frequency), Hour: s.Schedule.Hour, Weekday: s.Schedule.Weekday, Timezone: s.Schedule.Timezone},
	}
}

func toSharesResponse(in []port.ShareView) []shareResponse {
	out := make([]shareResponse, 0, len(in))
	for _, s := range in {
		out = append(out, shareResponse{Type: string(s.Type), TargetID: s.TargetID.String(), TargetName: s.TargetName, Permission: string(s.Permission)})
	}
	return out
}

func toDetailResponse(d *port.FilterDetail) filterResponse {
	resp := toFilterResponse(d.FilterView)
	if d.Subscription != nil {
		s := toSubscriptionResponse(*d.Subscription)
		resp.Subscription = &s
	}
	if d.Access.CanManage() {
		resp.Shares = toSharesResponse(d.Shares)
	}
	return resp
}

// filterID reads the {id} of the path. A malformed id is a filter that does not exist.
func filterID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, domain.ErrFilterNotFound
	}
	return id, nil
}

// ListFilters lists the filters the caller can see: scope=all|mine|shared|favorite, q=name text.
func (h *Handler) ListFilters(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, size := 1, 0
	for name, dst := range map[string]*int{"page": &page, "page_size": &size} {
		if raw := q.Get(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", name+" no es válido"))
				return
			}
			*dst = n
		}
	}
	viewer := viewerOf(r)
	views, total, err := h.filters.List(r.Context(), viewer.UserID, port.FilterListQuery{
		Scope: port.FilterScope(q.Get("scope")), Text: q.Get("q"), Page: page, PageSize: size,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]filterResponse, 0, len(views))
	for _, v := range views {
		out = append(out, toFilterResponse(v))
	}
	if size <= 0 || size > 100 {
		size = 50
	}
	if page < 1 {
		page = 1
	}
	httpresponse.Paginated(w, http.StatusOK, out, httpresponse.PageMeta{Page: page, PageSize: size, Total: total})
}

type createFilterRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Query          string   `json:"query"`
	Columns        []string `json:"columns"`
	Sort           string   `json:"sort"`
	OrganizationID *string  `json:"organization_id"`
}

// CreateFilter saves a filter owned by the caller. The query is validated, including its field and value rules.
func (h *Handler) CreateFilter(w http.ResponseWriter, r *http.Request) {
	var req createFilterRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	org := uuid.Nil
	if req.OrganizationID != nil && *req.OrganizationID != "" {
		id, err := uuid.Parse(*req.OrganizationID)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "organization_id no es válido"))
			return
		}
		org = id
	}
	detail, err := h.filters.Create(r.Context(), viewerOf(r), org, domain.FilterFields{
		Name: req.Name, Description: req.Description, QueryText: req.Query, Columns: req.Columns, Sort: req.Sort,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toDetailResponse(detail))
}

// GetFilter returns one filter. Only the owner receives the shares.
func (h *Handler) GetFilter(w http.ResponseWriter, r *http.Request) {
	id, err := filterID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	detail, err := h.filters.Get(r.Context(), viewerOf(r).UserID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDetailResponse(detail))
}

type updateFilterRequest struct {
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	Query       *string   `json:"query"`
	Columns     *[]string `json:"columns"`
	Sort        *string   `json:"sort"`
}

// UpdateFilter changes a filter. The owner and the editors it is shared with can.
func (h *Handler) UpdateFilter(w http.ResponseWriter, r *http.Request) {
	id, err := filterID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req updateFilterRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	detail, err := h.filters.Update(r.Context(), viewerOf(r), id, port.FilterPatch{
		Name: req.Name, Description: req.Description, QueryText: req.Query, Columns: req.Columns, Sort: req.Sort,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDetailResponse(detail))
}

// DeleteFilter removes a filter. Only the owner can.
func (h *Handler) DeleteFilter(w http.ResponseWriter, r *http.Request) {
	id, err := filterID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if err := h.filters.Delete(r.Context(), viewerOf(r).UserID, id); err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

type shareRequest struct {
	Type       string `json:"type"`
	TargetID   string `json:"target_id"`
	Permission string `json:"permission"`
}

type setSharesRequest struct {
	Shares []shareRequest `json:"shares"`
}

// SetFilterShares replaces the shares of a filter. Only the owner can.
func (h *Handler) SetFilterShares(w http.ResponseWriter, r *http.Request) {
	id, err := filterID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req setSharesRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	shares := make([]domain.Share, 0, len(req.Shares))
	for _, s := range req.Shares {
		target, perr := uuid.Parse(s.TargetID)
		if perr != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, domain.CodeShareInvalid, "target_id no es válido"))
			return
		}
		shares = append(shares, domain.Share{Type: domain.ShareType(s.Type), TargetID: target, Permission: domain.Permission(s.Permission)})
	}
	views, err := h.filters.SetShares(r.Context(), viewerOf(r).UserID, id, shares)
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toSharesResponse(views))
}

// RunFilter runs the saved query for the caller. Whatever the filter says, only browsable issues come back.
func (h *Handler) RunFilter(w http.ResponseWriter, r *http.Request) {
	id, err := filterID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	req, ok := readSearchRequest(w, r)
	if !ok {
		return
	}
	withTotal := req.WithTotal == nil || *req.WithTotal
	out, err := h.filters.Run(r.Context(), viewerOf(r), id, req.Page, req.PageSize, withTotal)
	if err != nil {
		writeError(w, err)
		return
	}
	writeIssuePage(w, out, req.Fields)
}

type subscribeRequest struct {
	Frequency string `json:"frequency"`
	Hour      int    `json:"hour"`
	Weekday   int    `json:"weekday"`
	Timezone  string `json:"timezone"`
}

// Subscribe creates or replaces the caller's subscription to a filter they can see.
func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	id, err := filterID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req subscribeRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	sub, err := h.filters.Subscribe(r.Context(), viewerOf(r), id, domain.Schedule{
		Frequency: domain.Frequency(req.Frequency), Hour: req.Hour, Weekday: req.Weekday, Timezone: req.Timezone,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toSubscriptionResponse(sub))
}

// GetSubscription returns the caller's subscription to a filter.
func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := filterID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	detail, err := h.filters.Get(r.Context(), viewerOf(r).UserID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	if detail.Subscription == nil {
		httpresponse.Error(w, domain.ErrSubscriptionNotFound)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toSubscriptionResponse(*detail.Subscription))
}

// Unsubscribe removes the caller's subscription.
func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	id, err := filterID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if err := h.filters.Unsubscribe(r.Context(), viewerOf(r).UserID, id); err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

package http

import (
	"encoding/json"
	"net/http"

	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

// globalHitResponse extends the original hit with the key, the escaped title with <mark> highlights and an
// escaped excerpt. highlight and snippet are safe to render as HTML.
type globalHitResponse struct {
	hitResponse
	Key       string `json:"key,omitempty"`
	Highlight string `json:"highlight"`
	Snippet   string `json:"snippet,omitempty"`
}

// searchGlobal serves GET /search with the v2 index. The data array keeps the original shape (one entry per
// hit) and gains fields; meta.counts gives the number of hits per kind. The types query parameter selects kinds
// (all, or a comma-separated list of project, issue, person, team, filter, note); without it only projects and
// issues are returned, as before.
func (h *Handler) searchGlobal(w http.ResponseWriter, r *http.Request, viewer port.Viewer, limit int) {
	kinds := domain.ParseKinds(r.URL.Query().Get("types"))
	hits, err := h.global.SearchAll(r.Context(), viewer, r.URL.Query().Get("q"), kinds, limit)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	data := make([]globalHitResponse, 0, len(hits))
	counts := map[string]int{}
	for _, k := range kinds {
		counts[string(k)] = 0
	}
	for _, hit := range hits {
		counts[string(hit.Kind)]++
		data = append(data, globalHitResponse{
			hitResponse: hitResponse{Kind: string(hit.Kind), ID: hit.ID, Title: hit.Title, Subtitle: hit.Subtitle, Link: hit.Link},
			Key:         hit.Key, Highlight: hit.Highlight, Snippet: hit.Snippet,
		})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "meta": map[string]any{"counts": counts}})
}

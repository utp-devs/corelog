package http

import (
	"net/http"

	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	"github.com/jhonsferg/corelog/internal/platform/phone"
)

type countryResponse struct {
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	DialCode string   `json:"dial_code"`
	Example  string   `json:"example,omitempty"`
	Areas    []string `json:"areas,omitempty"`
}

// countriesBody is built once: the table never changes at runtime.
var countriesBody = func() []countryResponse {
	list := phone.Countries()
	out := make([]countryResponse, 0, len(list))
	for _, c := range list {
		out = append(out, countryResponse{Code: c.Code, Name: c.Name, DialCode: "+" + c.DialCode, Example: c.Example, Areas: c.Areas})
	}
	return out
}()

// Countries lists the supported phone countries, sorted by name. It is public and cacheable.
func (h *Handler) Countries(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=86400")
	httpresponse.JSON(w, http.StatusOK, countriesBody)
}

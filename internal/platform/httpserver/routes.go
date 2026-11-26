package httpserver

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Route is one method plus full path registered in a chi router. Path parameters are normalised to "{}"
// so that routes compare equal regardless of the parameter names used in code or in the API contract.
type Route struct {
	Method string
	Path   string
}

var paramPattern = regexp.MustCompile(`\{[^}]*\}`)

// normalisePattern replaces path parameters with "{}" and drops the trailing slash that chi.Walk reports for
// the root of a mounted sub-router (Route("/teams", func(r) { r.Get("/", ...) }) is walked as "/teams/").
// chi serves both spellings with the same handler, so the trim loses nothing.
func normalisePattern(route string) string {
	p := paramPattern.ReplaceAllString(route, "{}")
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

// Routes lists every method and pattern registered in r, including sub-routers mounted with Route or Mount.
// OPTIONS and HEAD are skipped: they are implicit (CORS preflight, HEAD handling) and are not part of the contract.
func Routes(r chi.Routes) ([]Route, error) {
	var routes []Route
	err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method == http.MethodOptions || method == http.MethodHead {
			return nil
		}
		routes = append(routes, Route{Method: method, Path: normalisePattern(route)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return routes, nil
}

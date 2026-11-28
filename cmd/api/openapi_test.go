package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/httpserver"
)

const openAPIPath = "../../openapi.yaml"

// publicOperations are the only operations allowed without a security requirement.
var publicOperations = map[string]bool{
	"POST /api/v1/auth/register":        true,
	"GET /api/v1/invitations/{}":        true,
	"POST /api/v1/auth/login":           true,
	"POST /api/v1/auth/login/2fa":       true,
	"POST /api/v1/auth/password/forgot": true,
	"POST /api/v1/auth/password/reset":  true,
	"POST /api/v1/auth/email/verify":    true,
	"GET /health":                       true,
	"GET /api/v1/meta/countries":        true,
	"GET /api/v1/legal/documents":       true,
	"GET /api/v1/legal/documents/{}":    true,
	"GET /api/v1/calendar/export.ics":   true,
}

var (
	specParamPattern = regexp.MustCompile(`\{[^}]*\}`)
	httpMethods      = []string{"get", "put", "post", "delete", "patch", "options", "head", "trace"}
)

// specOperation is one operation of the contract, keyed by its full path (server base path included).
type specOperation struct {
	Method string
	Path   string
	Op     map[string]any
	Spec   map[string]any // the root document, needed for the global security default
}

func (o specOperation) key() string { return o.Method + " " + o.Path }

func loadSpec(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(openAPIPath)
	if err != nil {
		t.Fatalf("read %s: %v", openAPIPath, err)
	}
	spec, err := parseSpec(raw)
	if err != nil {
		t.Fatalf("parse %s: %v", openAPIPath, err)
	}
	return spec
}

func parseSpec(raw []byte) (map[string]any, error) {
	var spec map[string]any
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, err
	}
	return spec, nil
}

// asMap accepts both map[string]any and map[any]any, which yaml.v3 produces depending on the keys.
func asMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out, true
	}
	return nil, false
}

// specOperations lists every operation of the spec. The full path is the path of the effective server URL
// (operation, then path item, then document level) joined with the path key, so "/api/v1" comes from the
// document server and an operation may override it (for example GET /health lives at the host root).
func specOperations(spec map[string]any) ([]specOperation, error) {
	paths, ok := asMap(spec["paths"])
	if !ok {
		return nil, fmt.Errorf("spec has no paths")
	}
	var ops []specOperation
	for pathKey, rawItem := range paths {
		item, ok := asMap(rawItem)
		if !ok {
			return nil, fmt.Errorf("path %s is not an object", pathKey)
		}
		for _, method := range httpMethods {
			rawOp, found := item[method]
			if !found {
				continue
			}
			op, ok := asMap(rawOp)
			if !ok {
				return nil, fmt.Errorf("%s %s is not an object", strings.ToUpper(method), pathKey)
			}
			base, err := serverBasePath(op, item, spec)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", strings.ToUpper(method), pathKey, err)
			}
			ops = append(ops, specOperation{
				Method: strings.ToUpper(method),
				Path:   normalisePath(base + pathKey),
				Op:     op,
				Spec:   spec,
			})
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].key() < ops[j].key() })
	return ops, nil
}

func serverBasePath(op, item, spec map[string]any) (string, error) {
	for _, scope := range []map[string]any{op, item, spec} {
		servers, ok := scope["servers"].([]any)
		if !ok || len(servers) == 0 {
			continue
		}
		server, ok := asMap(servers[0])
		if !ok {
			return "", fmt.Errorf("server entry is not an object")
		}
		raw, _ := server["url"].(string)
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("server url %q: %w", raw, err)
		}
		return strings.TrimSuffix(u.Path, "/"), nil
	}
	return "", nil
}

func normalisePath(p string) string {
	return specParamPattern.ReplaceAllString(p, "{}")
}

// compareRoutes reports router routes with no operation and operations with no router route.
func compareRoutes(routes []httpserver.Route, ops []specOperation) (missingInSpec, missingInRouter []string) {
	inRouter := make(map[string]bool, len(routes))
	for _, r := range routes {
		inRouter[r.Method+" "+r.Path] = true
	}
	inSpec := make(map[string]bool, len(ops))
	for _, o := range ops {
		inSpec[o.key()] = true
	}
	for k := range inRouter {
		if !inSpec[k] {
			missingInSpec = append(missingInSpec, k)
		}
	}
	for k := range inSpec {
		if !inRouter[k] {
			missingInRouter = append(missingInRouter, k)
		}
	}
	sort.Strings(missingInSpec)
	sort.Strings(missingInRouter)
	return missingInSpec, missingInRouter
}

// checkSecurity returns operations that have no effective security requirement and are not public.
func checkSecurity(ops []specOperation) []string {
	var violations []string
	for _, o := range ops {
		if publicOperations[o.key()] {
			continue
		}
		effective, own := o.Op["security"]
		if !own {
			effective = o.Spec["security"]
		}
		if list, ok := effective.([]any); !ok || len(list) == 0 {
			violations = append(violations, o.key())
		}
	}
	sort.Strings(violations)
	return violations
}

// checkResponses returns operations that lack a 2xx or a 4xx response.
func checkResponses(ops []specOperation) []string {
	var violations []string
	for _, o := range ops {
		responses, ok := asMap(o.Op["responses"])
		if !ok {
			violations = append(violations, o.key()+" (no responses)")
			continue
		}
		has2xx, has4xx := false, false
		for code := range responses {
			switch {
			case strings.HasPrefix(code, "2"):
				has2xx = true
			case strings.HasPrefix(code, "4"):
				has4xx = true
			}
		}
		if !has2xx {
			violations = append(violations, o.key()+" (no 2xx response)")
		}
		if !has4xx {
			violations = append(violations, o.key()+" (no 4xx response)")
		}
	}
	sort.Strings(violations)
	return violations
}

// buildTestRouter assembles the real router without a database and without the session validator.
func buildTestRouter(t *testing.T) []httpserver.Route {
	t.Helper()
	cfg := &config.Config{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router, err := buildRouter(cfg, nil, log)
	if err != nil {
		t.Fatalf("buildRouter: %v", err)
	}
	routes, err := httpserver.Routes(router)
	if err != nil {
		t.Fatalf("httpserver.Routes: %v", err)
	}
	return routes
}

func TestOpenAPIMatchesRouter(t *testing.T) {
	spec := loadSpec(t)
	ops, err := specOperations(spec)
	if err != nil {
		t.Fatalf("specOperations: %v", err)
	}
	routes := buildTestRouter(t)

	missingInSpec, missingInRouter := compareRoutes(routes, ops)
	if len(missingInSpec) > 0 || len(missingInRouter) > 0 {
		t.Errorf("router and openapi.yaml differ\nroutes without an operation in openapi.yaml (%d):\n  %s\noperations without a route (%d):\n  %s",
			len(missingInSpec), strings.Join(missingInSpec, "\n  "),
			len(missingInRouter), strings.Join(missingInRouter, "\n  "))
	}
	if v := checkSecurity(ops); len(v) > 0 {
		t.Errorf("operations without security (and not public): %s", strings.Join(v, ", "))
	}
	if v := checkResponses(ops); len(v) > 0 {
		t.Errorf("operations missing 2xx or 4xx responses: %s", strings.Join(v, ", "))
	}
}

// TestOpenAPIDriftDetectedInMemory mutates a freshly parsed copy of the spec in memory (never the file)
// and asserts that the checks report each kind of drift.
func TestOpenAPIDriftDetectedInMemory(t *testing.T) {
	routes := buildTestRouter(t)

	t.Run("removed path is reported", func(t *testing.T) {
		spec := loadSpec(t)
		paths := spec["paths"].(map[string]any)
		if _, ok := paths["/teams"]; !ok {
			t.Fatal("fixture path /teams missing from spec")
		}
		delete(paths, "/teams")
		ops, err := specOperations(spec)
		if err != nil {
			t.Fatal(err)
		}
		missingInSpec, _ := compareRoutes(routes, ops)
		if !containsPrefix(missingInSpec, "GET /api/v1/teams") {
			t.Errorf("removed /teams not reported as a route without operation; got %v", missingInSpec)
		}
	})

	t.Run("bogus path is reported", func(t *testing.T) {
		spec := loadSpec(t)
		paths := spec["paths"].(map[string]any)
		paths["/bogus"] = map[string]any{
			"get": map[string]any{"responses": map[string]any{"200": map[string]any{}, "404": map[string]any{}}},
		}
		ops, err := specOperations(spec)
		if err != nil {
			t.Fatal(err)
		}
		_, missingInRouter := compareRoutes(routes, ops)
		if !containsPrefix(missingInRouter, "GET /api/v1/bogus") {
			t.Errorf("bogus path not reported as operation without route; got %v", missingInRouter)
		}
	})

	t.Run("missing security is reported", func(t *testing.T) {
		spec := loadSpec(t)
		op := spec["paths"].(map[string]any)["/teams"].(map[string]any)["get"].(map[string]any)
		delete(op, "security")
		delete(spec, "security")
		ops, err := specOperations(spec)
		if err != nil {
			t.Fatal(err)
		}
		if !containsPrefix(checkSecurity(ops), "GET /api/v1/teams") {
			t.Error("operation without security not reported")
		}
	})

	t.Run("missing 4xx response is reported", func(t *testing.T) {
		spec := loadSpec(t)
		op := spec["paths"].(map[string]any)["/teams"].(map[string]any)["get"].(map[string]any)
		op["responses"] = map[string]any{"200": map[string]any{"description": "ok"}}
		ops, err := specOperations(spec)
		if err != nil {
			t.Fatal(err)
		}
		if !containsPrefix(checkResponses(ops), "GET /api/v1/teams") {
			t.Error("operation without 4xx response not reported")
		}
	})
}

func containsPrefix(list []string, prefix string) bool {
	for _, s := range list {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

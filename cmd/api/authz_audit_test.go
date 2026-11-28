package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestAuthzRegistryCoversTheRouter fails when a route has no classification in the registry (or the registry
// lists a route that no longer exists). It needs no database.
func TestAuthzRegistryCoversTheRouter(t *testing.T) {
	registry, err := parseAuthzRegistry()
	if err != nil {
		t.Fatal(err)
	}
	inRouter := map[string]bool{}
	var unclassified []string
	for _, r := range buildTestRouter(t) {
		key := r.Method + " " + r.Path
		inRouter[key] = true
		if _, ok := registry[key]; !ok {
			unclassified = append(unclassified, key)
		}
	}
	var stale []string
	for key := range registry {
		if !inRouter[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(unclassified)
	sort.Strings(stale)
	if len(unclassified) > 0 {
		t.Errorf("routes without an authorization classification: classify them in authz_registry_test.go (scope pub/self/plat/scoped/own/orgR/orgC/orgW/orgO/pR/pW/pM):\n  %s", strings.Join(unclassified, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("registry lines for routes that do not exist (remove them):\n  %s", strings.Join(stale, "\n  "))
	}
}

// routeBodies are the minimal valid bodies of the routes whose handler reads a body before it authorizes, so the
// denial under test is the authorization and not a validation error.
var routeBodies = map[string]string{
	"POST /api/v1/issues":                                     `{"summary":"Probe issue","project_id":"{project}"}`,
	"POST /api/v1/projects":                                   `{"organization_id":"{org}","key":"ZQX","name":"Probe project"}`,
	"POST /api/v1/teams":                                      `{"organization_id":"{org}","name":"Probe team"}`,
	"POST /api/v1/automation-rules/validate":                  `{"project_id":"{project}","name":"x","trigger":{"type":"issue_created"},"actions":[{"type":"add_label","label":"a"}]}`,
	"POST /api/v1/organizations/{}/invitations":               `{"invitations":[{"email":"probe@authz.test","role":"member"}]}`,
	"POST /api/v1/issues/{}/comments":                         `{"body":"probe"}`,
	"POST /api/v1/issues/{}/worklogs":                         `{"time_spent":"1h"}`,
	"POST /api/v1/projects/{}/versions":                       `{"name":"probe-v"}`,
	"POST /api/v1/projects/{}/labels":                         `{"name":"probe","color":"blue"}`,
	"POST /api/v1/projects/{}/components":                     `{"name":"probe"}`,
	"POST /api/v1/sprints":                                    `{"name":"Probe sprint"}`,
	"PATCH /api/v1/organizations/{}":                          `{"description":"probe"}`,
	"POST /api/v1/organizations/{}/webhooks":                  `{"name":"probe","url":"https://example.com/p","events":["issue.created"]}`,
	"POST /api/v1/projects/{}/automation-rules":               `{"name":"probe","trigger":{"type":"issue_created"},"actions":[{"type":"add_label","label":"a"}]}`,
	"POST /api/v1/projects/{}/boards":                         `{"name":"probe"}`,
	"POST /api/v1/organizations/{}/priorities":                `{"name":"probe","icon":"flag","color":"red"}`,
	"POST /api/v1/organizations/{}/statuses":                  `{"name":"probe","category":"todo"}`,
	"POST /api/v1/organizations/{}/custom-fields":             `{"name":"probe","type":"text"}`,
	"POST /api/v1/organizations/{}/issue-types":               `{"name":"probe","icon":"task","color":"blue"}`,
	"POST /api/v1/organizations/{}/resolutions":               `{"name":"probe"}`,
	"PATCH /api/v1/issues/{}":                                 `{"summary":"probe summary"}`,
	"PATCH /api/v1/projects/{}":                               `{"name":"Probe renamed","type":"scrum","lead_id":"{user}","visibility":"private"}`,
	"POST /api/v1/projects/{}/members":                        `{"user_id":"{user}","role":"viewer"}`,
	"POST /api/v1/organizations/{}/transfer-ownership":        `{"user_id":"{user}"}`,
	"PATCH /api/v1/organizations/{}/members/{}":               `{"role":"member"}`,
	"POST /api/v1/issues/{}/links":                            `{"target":"{issue}","kind":"relates"}`,
	"POST /api/v1/issues/{}/transitions":                      `{"transition_id":"start"}`,
	"POST /api/v1/sprints/{}/start":                           `{}`,
	"PATCH /api/v1/sprints/{}":                                `{"name":"Probe renamed"}`,
	"PATCH /api/v1/comments/{}":                               `{"body":"probe edit"}`,
	"PATCH /api/v1/issues/{}/comments/{}":                     `{"body":"probe edit"}`,
	"PATCH /api/v1/versions/{}":                               `{"name":"probe-renamed"}`,
	"PATCH /api/v1/worklogs/{}":                               `{"description":"probe edit"}`,
	"POST /api/v1/organizations/{}/webhooks/{}/test":          `{}`,
	"PUT /api/v1/projects/{}/issue-types":                     `{"issue_type_ids":["{issuetype}"]}`,
	"PUT /api/v1/projects/{}/permissions":                     `{"grants":[]}`,
	"POST /api/v1/organizations/{}/leave":                     `{}`,
	"POST /api/v1/projects/{}/board/cards/{}/move":            `{"status":"in_progress"}`,
	"POST /api/v1/boards/{}/cards/{}/move":                    `{"status":"in_progress"}`,
	"PATCH /api/v1/organizations/{}/webhooks/{}":              `{"name":"probe-renamed"}`,
	"PATCH /api/v1/projects/{}/labels/{}":                     `{"name":"probe-renamed"}`,
	"PATCH /api/v1/projects/{}/components/{}":                 `{"name":"probe-renamed"}`,
	"PATCH /api/v1/automation-rules/{}":                       `{"name":"probe-renamed"}`,
	"PATCH /api/v1/organizations/{}/priorities/{}":            `{"name":"probe-renamed"}`,
	"PATCH /api/v1/organizations/{}/statuses/{}":              `{"name":"probe-renamed"}`,
	"PATCH /api/v1/organizations/{}/resolutions/{}":           `{"name":"probe-renamed"}`,
	"PATCH /api/v1/organizations/{}/issue-types/{}":           `{"name":"probe-renamed"}`,
	"PATCH /api/v1/organizations/{}/custom-fields/{}":         `{"name":"probe-renamed"}`,
	"PATCH /api/v1/teams/{}":                                  `{"name":"Probe renamed"}`,
	"POST /api/v1/teams/{}/members":                           `{"user_id":"{user}","role":"member"}`,
	"POST /api/v1/projects/{}/team-roles":                     `{}`,
	"POST /api/v1/projects/{}/workflow/validate":              `{}`,
	"POST /api/v1/projects/{}/workflow/reset":                 `{}`,
	"POST /api/v1/projects/{}/permissions/reset":              `{}`,
	"POST /api/v1/projects/{}/restore":                        `{}`,
	"POST /api/v1/projects/{}/archive":                        `{}`,
	"POST /api/v1/issues/{}/clone":                            `{}`,
	"POST /api/v1/issues/{}/move":                             `{"project_id":"{project}"}`,
	"POST /api/v1/issues/{}/restore":                          `{}`,
	"POST /api/v1/versions/{}/release":                        `{}`,
	"POST /api/v1/versions/{}/archive":                        `{}`,
	"POST /api/v1/sprints/{}/close":                           `{}`,
	"POST /api/v1/sprints/{}/complete":                        `{}`,
	"PUT /api/v1/issues/{}/estimates":                         `{}`,
	"PATCH /api/v1/issues/{}/details":                         `{}`,
	"PATCH /api/v1/issues/{}/metadata":                        `{}`,
	"PATCH /api/v1/issues/{}/assign":                          `{"assignee_id":null}`,
	"PUT /api/v1/backlog/rank":                                `{}`,
	"DELETE /api/v1/organizations/{}":                         `{"confirm_slug":"zorbax"}`,
	"POST /api/v1/projects/{}/workflow/migrate-issues":        `{}`,
	"POST /api/v1/automation-rules/{}/enable":                 `{}`,
	"POST /api/v1/automation-rules/{}/disable":                `{}`,
	"POST /api/v1/organizations/{}/webhooks/{}/rotate-secret": `{}`,
}

// pathKinds maps the literal segment before a parameter to the kind of resource the parameter names.
var pathKinds = map[string]string{
	"projects": "project", "issues": "issue", "organizations": "org", "boards": "board", "sprints": "sprint",
	"versions": "version", "filters": "filter", "saved-filters": "savedfilter", "dashboards": "dashboard", "notes": "note",
	"worklogs": "worklog", "comments": "comment", "labels": "label", "components": "component", "webhooks": "webhook",
	"deliveries": "delivery", "attachments": "attachment", "members": "member", "teams": "team", "users": "user",
	"people": "user", "invitations": "invitation", "automation-rules": "rule", "custom-fields": "customfield",
	"issue-types": "issuetype", "statuses": "status", "priorities": "priority", "resolutions": "resolution", "links": "link",
	"cards": "issue", "columns": "column", "jobs": "job", "csv": "upload", "api-tokens": "apitoken", "sessions": "session",
	"consents": "consent", "reactions": "emoji", "events": "event", "team-roles": "teamrole", "backlog": "issue",
	"favorites": "favtype", "documents": "document", "watchers": "user",
}

// resolved is a request target with the path kinds that had no real resource behind them.
type resolved struct {
	target string
	random []string
}

// resolve fills the {} parameters of the route pattern with the resources of one world.
func resolve(pattern string, world map[string]string, extraQuery string) resolved {
	pattern = strings.TrimPrefix(pattern, "/api/v1")
	segs := strings.Split(pattern, "/")
	var random []string
	for i, seg := range segs {
		if seg != "{}" {
			continue
		}
		prev := ""
		if i > 0 {
			prev = segs[i-1]
		}
		kind := pathKinds[prev]
		if prev == "{}" {
			kind = "favid"
		}
		switch {
		case kind == "favtype":
			segs[i] = "project"
		case kind == "favid":
			segs[i] = world["project"]
		case kind == "emoji":
			segs[i] = "thumbs_up"
		case world[kind] != "":
			segs[i] = world[kind]
		default:
			segs[i] = uuid.NewString()
			random = append(random, kind)
		}
	}
	target := strings.Join(segs, "/")
	if target == "" {
		target = "/"
	}
	q := "project_id=" + world["project"] + "&organization_id=" + world["org"] + extraQuery
	return resolved{target: target + "?" + q, random: random}
}

func fill(s string, world map[string]string, leadID string) string {
	r := strings.NewReplacer("{project}", world["project"], "{org}", world["org"], "{issue}", world["issue"],
		"{user}", world["user"], "{lead}", leadID, "{issuetype}", world["issuetype"])
	return r.Replace(s)
}

// allowed and denied actors per scope for the private project. Actors in neither list are observed but not asserted
// (a guest of the organization on organization routes, for instance, depends on settings the audit does not pin).
func expectations(scope string) (allow, deny []string) {
	projectDenyBase := []string{actNoOrg, actGuest, actMember, actOutsider}
	switch scope {
	case scopePlat:
		return []string{actPlatform}, []string{actNoOrg, actGuest, actMember, actViewer, actContributor, actLead, actOrgAdmin, actOrgOwner, actOutsider}
	case scopeOwn:
		return []string{actLead}, []string{actNoOrg, actGuest, actMember, actViewer, actContributor, actOrgAdmin, actOrgOwner, actPlatform, actOutsider}
	case scopeOwnA:
		return []string{actLead, actOrgAdmin, actOrgOwner}, []string{actNoOrg, actGuest, actMember, actViewer, actContributor, actPlatform, actOutsider}
	case scopeOrgR:
		return []string{actOrgOwner, actOrgAdmin, actMember, actLead, actContributor, actViewer}, []string{actNoOrg, actOutsider}
	case scopeOrgC:
		return []string{actOrgOwner, actOrgAdmin, actMember}, []string{actNoOrg, actOutsider}
	case scopeOrgW:
		return []string{actOrgOwner, actOrgAdmin}, []string{actNoOrg, actGuest, actMember, actViewer, actContributor, actLead, actOutsider}
	case scopeOrgO:
		return []string{actOrgOwner}, []string{actNoOrg, actGuest, actMember, actViewer, actContributor, actLead, actOrgAdmin, actOutsider}
	case scopeProjR:
		return []string{actLead, actContributor, actViewer, actOrgAdmin, actOrgOwner, actPlatform}, projectDenyBase
	case scopeProjW:
		return []string{actLead, actContributor, actOrgAdmin, actOrgOwner}, append([]string{actViewer, actPlatform}, projectDenyBase...)
	case scopeProjM:
		return []string{actLead, actOrgAdmin, actOrgOwner}, append([]string{actContributor, actViewer, actPlatform}, projectDenyBase...)
	}
	return nil, nil
}

// archiveAllowed are the writes an archived project still accepts (they change the archive state itself).
var archiveAllowed = map[string]bool{
	"POST /api/v1/projects/{}/archive": true, "POST /api/v1/projects/{}/restore": true, "DELETE /api/v1/projects/{}": true,
}

// privateSecrets are strings that exist only inside the private project; nobody without access may see them.
var privateSecrets = []string{"Zorbax Secreto", "ZQPRV", "Zorbax incidencia secreta", "Zorbax incidencia hermana", "Zorbax comentario", "Zorbax Sprint privado", "zorbax-1.0", "zorbax-label", "zorbax-comp", "Zorbax regla", "Zorbax filtro", "Zorbax panel", "Zorbax nota", "Zorbax evento", "Zorbax guardado"}

// leaks returns the protected strings found in a response body.
func (f *authzFixture) leaks(actor string, body []byte) []string {
	text := string(body)
	needles := privateSecrets
	if actor == actOutsider || actor == actNoOrg {
		needles = append([]string{"Zorbax", "ZQOPN", "ZQARC", "zorbax"}, privateSecrets...)
		for _, other := range authzActors {
			if other != actAnonymous && other != actor && other != actOutsider && other != actNoOrg {
				needles = append(needles, "Zq "+other)
			}
		}
	}
	var found []string
	for _, n := range needles {
		if strings.Contains(text, n) {
			found = append(found, n)
		}
	}
	return found
}

type authzCall struct {
	route  authzRoute
	actor  string
	world  string
	status int
	body   []byte
	random []string
	target string
}

func (c authzCall) String() string {
	return fmt.Sprintf("%s %s as %s [%s world] -> %d %s", c.route.Method, c.target, c.actor, c.world, c.status, truncate(c.body, 140))
}

func (f *authzFixture) request(route authzRoute, actor string, worldName string, world map[string]string) authzCall {
	f.t.Helper()
	extra := ""
	if route.Path == "/api/v1/projects/key-available" {
		extra = "&key=ZQNEW"
	}
	extra += "&from=2026-01-01&to=2026-02-01&issue_type_id=" + f.ids["issuetype"]
	res := resolve(route.Path, world, extra)
	var body any
	if route.Method == http.MethodPost || route.Method == http.MethodPut || route.Method == http.MethodPatch {
		raw := routeBodies[route.key()]
		if raw == "" {
			raw = "{}"
		}
		body = jsonRaw(fill(raw, world, f.users[actLead].String()))
	}
	status, resp := f.call(route.Method, res.target, f.tokens[actor], body)
	return authzCall{route: route, actor: actor, world: worldName, status: status, body: resp, random: res.random, target: res.target}
}

type jsonRaw string

func (j jsonRaw) MarshalJSON() ([]byte, error) { return []byte(j), nil }

func isWrite(method string) bool { return method != http.MethodGet }

// checkDenied asserts the response of a caller who must not get through.
func (f *authzFixture) checkDenied(c authzCall, problems *[]string) {
	add := func(format string, args ...any) {
		*problems = append(*problems, fmt.Sprintf(format, args...)+"\n      "+c.String())
	}
	switch {
	case c.status >= 200 && c.status < 300:
		add("DENIED CALLER GOT A SUCCESS")
	case c.status >= 500:
		add("server error instead of a denial")
	case c.status == http.StatusUnauthorized:
		add("authenticated caller got 401 (fixture token rejected?)")
	case !isWrite(c.route.Method) && c.status != http.StatusForbidden && c.status != http.StatusNotFound:
		add("read denied with %d, want 403 or 404", c.status)
	}
	if found := f.leaks(c.actor, c.body); len(found) > 0 {
		add("response leaks %v", found)
	}
}

// checkAllowed asserts the response of a caller who must get through (the call may still fail validation).
func (f *authzFixture) checkAllowed(c authzCall, problems *[]string) {
	add := func(format string, args ...any) {
		*problems = append(*problems, fmt.Sprintf(format, args...)+"\n      "+c.String())
	}
	switch {
	case c.status >= 500:
		add("server error")
	case c.status == http.StatusUnauthorized || c.status == http.StatusForbidden:
		add("allowed caller refused")
	case c.status == http.StatusNotFound && len(c.random) == 0 && !c.route.Soft404:
		add("allowed caller got 404 although every resource exists")
	}
}

// TestAuthzAudit walks every route of the real router and checks it against the registry with the actor profiles.
func TestAuthzAudit(t *testing.T) {
	registry, err := parseAuthzRegistry()
	if err != nil {
		t.Fatal(err)
	}
	f := newAuthzFixture(t)
	routes := sortedRoutes(registry)
	var problems []string
	observed := map[string]int{}

	// Phase 1: every caller who must be turned away, plus anonymous callers. Nothing here may change data.
	for _, route := range routes {
		if route.Scope == scopePublic {
			c := f.request(route, actAnonymous, "private", f.ids)
			if c.status >= 500 {
				problems = append(problems, "public route fails: "+c.String())
			}
			continue
		}
		c := f.request(route, actAnonymous, "private", f.ids)
		if c.status != http.StatusUnauthorized {
			problems = append(problems, "ANONYMOUS NOT REFUSED (want 401)\n      "+c.String())
		}
		_, deny := expectations(route.Scope)
		for _, actor := range deny {
			c := f.request(route, actor, "private", f.ids)
			observed[fmt.Sprintf("denied %d", c.status)]++
			f.checkDenied(c, &problems)
		}
	}

	// Phase 2: reads by every caller who may get through.
	for _, route := range routes {
		if isWrite(route.Method) || route.Scope == scopePublic {
			continue
		}
		allow, _ := expectations(route.Scope)
		if route.Scope == scopeSelf || route.Scope == scopeScoped {
			allow = []string{actLead, actMember, actOrgAdmin}
		}
		for _, actor := range allow {
			c := f.request(route, actor, "private", f.ids)
			f.checkAllowed(c, &problems)
			if (route.Scope == scopeScoped || route.Scope == scopeSelf) && c.status < 300 {
				if found := f.leaks(actor, c.body); len(found) > 0 && actor == actMember {
					problems = append(problems, fmt.Sprintf("member sees private project data %v\n      %s", found, c))
				}
			}
		}
		// Callers outside the organization must only ever see their own data in scoped listings.
		if route.Scope == scopeScoped || route.Scope == scopeSelf {
			for _, actor := range []string{actOutsider, actNoOrg} {
				c := f.request(route, actor, "private", f.ids)
				if c.status >= 500 {
					problems = append(problems, "server error\n      "+c.String())
				}
				if found := f.leaks(actor, c.body); len(found) > 0 {
					problems = append(problems, fmt.Sprintf("CROSS-ORGANIZATION LEAK %v\n      %s", found, c))
				}
			}
		}
	}

	// Phase 2b: the resources of organization B must be out of reach of every caller of organization A, whatever
	// their role there; the open project is readable by organization members only; an archived project accepts no writes.
	for _, route := range routes {
		switch route.Scope {
		case scopePublic, scopeSelf, scopeScoped, scopePlat:
			continue
		}
		for _, actor := range []string{actLead, actOrgOwner, actOrgAdmin, actContributor, actViewer, actMember} {
			c := f.request(route, actor, "other-org", f.other)
			f.checkDenied(c, &problems)
		}
		if route.Scope == scopeProjR || route.Scope == scopeProjW || route.Scope == scopeProjM {
			for _, actor := range []string{actGuest, actNoOrg, actOutsider} {
				f.checkDenied(f.request(route, actor, "open", f.open), &problems)
			}
			member := f.request(route, actMember, "open", f.open)
			if route.Scope == scopeProjR && !isWrite(route.Method) {
				f.checkAllowed(member, &problems)
			} else if route.Scope != scopeProjR {
				f.checkDenied(member, &problems)
			}
		}
	}
	for _, route := range routes {
		if (route.Scope != scopeProjW && route.Scope != scopeProjM) || !isWrite(route.Method) || route.NoPositive || archiveAllowed[route.key()] {
			continue
		}
		for _, actor := range []string{actLead, actOrgAdmin} {
			c := f.request(route, actor, "archived", f.archived)
			if c.status >= 200 && c.status < 300 {
				problems = append(problems, "WRITE ACCEPTED ON AN ARCHIVED PROJECT\n      "+c.String())
			}
		}
	}

	// Phase 3: writes by the weakest caller who may perform them, destructive routes last.
	writes := make([]authzRoute, 0)
	for _, route := range routes {
		if isWrite(route.Method) && route.Scope != scopePublic && !route.NoPositive {
			writes = append(writes, route)
		}
	}
	sort.SliceStable(writes, func(i, j int) bool {
		di, dj := writes[i].Method == http.MethodDelete, writes[j].Method == http.MethodDelete
		if di != dj {
			return dj
		}
		return di && len(writes[i].Path) > len(writes[j].Path)
	})
	for _, route := range writes {
		allow, _ := expectations(route.Scope)
		actor := actLead
		switch route.Scope {
		case scopeSelf, scopeScoped, scopeOwn, scopeProjR, scopeProjW, scopeProjM:
		case scopePlat:
			actor = actPlatform
		default:
			if len(allow) > 0 {
				actor = allow[0]
			}
		}
		c := f.request(route, actor, "private", f.ids)
		f.checkAllowed(c, &problems)
	}

	if len(f.unseeded) > 0 {
		t.Logf("kinds without a seeded resource: %v", f.unseeded)
	}
	t.Logf("denial statuses observed: %v", observed)
	if len(problems) > 0 {
		sort.Strings(problems)
		t.Errorf("%d authorization findings:\n%s", len(problems), strings.Join(problems, "\n"))
	}
}

// TestPlatformAdministratorHasSupportAccessOnly is the regression test of a finding of the audit: a platform
// administrator reads every project (support access) but must not manage one.
func TestPlatformAdministratorHasSupportAccessOnly(t *testing.T) {
	f := newAuthzFixture(t)
	project := f.ids["project"]
	token := f.tokens[actPlatform]
	if status, body := f.call(http.MethodGet, "/projects/"+project, token, nil); status != http.StatusOK {
		t.Fatalf("support read: %d %s", status, truncate(body, 120))
	}
	writes := []struct{ method, path string }{
		{http.MethodPatch, "/projects/" + project},
		{http.MethodPost, "/projects/" + project + "/archive"},
		{http.MethodDelete, "/projects/" + project + "/labels/" + f.ids["label"]},
		{http.MethodDelete, "/projects/" + project + "/components/" + f.ids["component"]},
		{http.MethodDelete, "/projects/" + project + "/members/" + f.ids["member"]},
	}
	for _, w := range writes {
		var body any
		if w.method == http.MethodPatch {
			body = map[string]any{"name": "Taken over", "type": "scrum", "lead_id": f.users[actPlatform].String(), "visibility": "private"}
		}
		if status, resp := f.call(w.method, w.path, token, body); status != http.StatusForbidden {
			t.Errorf("%s %s as platform administrator: %d %s, want 403", w.method, w.path, status, truncate(resp, 120))
		}
	}
	var name string
	if err := f.db.Get(&name, `SELECT name FROM projects WHERE id = $1`, project); err != nil || name != "Zorbax Secreto" {
		t.Errorf("project changed by a support caller: %q %v", name, err)
	}
}

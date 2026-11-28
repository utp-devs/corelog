package main

import (
	"fmt"
	"sort"
	"strings"
)

// Authorization scopes. Every route of the API is classified with exactly one of them; the audit derives from the
// scope who must be let in and who must be turned away, so a new route cannot ship without a decision.
//
//	pub     reachable without a token (a documented allowlist)
//	self    any authenticated user, only touches the caller's own data
//	plat    platform administrators only
//	scoped  any authenticated user; the data returned is limited to what the caller may see
//	own     a private object of its creator (filter, dashboard, note, calendar event...)
//	ownA    like own, and the administrators of the organization may also view it (dashboards)
//	orgR    members of the organization
//	orgC    members of the organization that the organization settings allow to create things
//	orgW    organization owners and administrators
//	orgO    the organization owner
//	pR      project read: any project role, organization administrators and platform support
//	pW      project write: contributors and above
//	pM      project management: leads and organization administrators
const (
	scopePublic  = "pub"
	scopeSelf    = "self"
	scopePlat    = "plat"
	scopeScoped  = "scoped"
	scopeOwn     = "own"
	scopeOwnA    = "ownA"
	scopeOrgR    = "orgR"
	scopeOrgC    = "orgC"
	scopeOrgW    = "orgW"
	scopeOrgO    = "orgO"
	scopeProjR   = "pR"
	scopeProjW   = "pW"
	scopeProjM   = "pM"
	flagNoPosCtl = "nopos"   // the positive control is skipped: the call is destructive or ends the caller's session
	flagSoft404  = "soft404" // a 404 is a legitimate answer for an allowed caller (nothing to return)
)

var validScopes = map[string]bool{
	scopePublic: true, scopeSelf: true, scopePlat: true, scopeScoped: true, scopeOwn: true, scopeOwnA: true, scopeOrgR: true, scopeOrgC: true,
	scopeOrgW: true, scopeOrgO: true, scopeProjR: true, scopeProjW: true, scopeProjM: true,
}

// authzRegistryText is the registry: "METHOD path scope [flags]". Paths are the router patterns with the
// parameters normalised to {}. TestAuthzRegistryCoversTheRouter fails for a route that is not listed here and
// for a line that no longer matches a route, so classify every new route when it is added.
const authzRegistryText = `
GET /api/v1/admin/audit plat
GET /api/v1/admin/audit/export plat
GET /api/v1/admin/email-outbox plat
POST /api/v1/admin/email-outbox/{}/retry plat
GET /api/v1/admin/jobs plat
GET /api/v1/admin/legal/consents plat
GET /api/v1/admin/legal/documents plat
POST /api/v1/admin/legal/documents plat
GET /api/v1/auth/2fa self
POST /api/v1/auth/2fa/confirm self
POST /api/v1/auth/2fa/disable self
POST /api/v1/auth/2fa/enroll self
POST /api/v1/auth/2fa/recovery-codes self
POST /api/v1/auth/email/verify pub
POST /api/v1/auth/email/verify/request self
POST /api/v1/auth/login pub
POST /api/v1/auth/login/2fa pub
POST /api/v1/auth/logout self nopos
POST /api/v1/auth/password/forgot pub
POST /api/v1/auth/password/reset pub
POST /api/v1/auth/register pub
POST /api/v1/automation-rules/validate pM
DELETE /api/v1/automation-rules/{} pM
GET /api/v1/automation-rules/{} pM
PATCH /api/v1/automation-rules/{} pM
POST /api/v1/automation-rules/{}/disable pM
POST /api/v1/automation-rules/{}/enable pM
GET /api/v1/automation-rules/{}/runs pM
GET /api/v1/backlog pR
PUT /api/v1/backlog/rank pW
PUT /api/v1/backlog/{} pW
DELETE /api/v1/boards/{} pM
GET /api/v1/boards/{} pR
PATCH /api/v1/boards/{} pM
POST /api/v1/boards/{}/cards/{}/move pW
PUT /api/v1/boards/{}/columns pM
PUT /api/v1/boards/{}/quick-filters pM
PUT /api/v1/boards/{}/swimlanes pM
GET /api/v1/boards/{}/view pR
GET /api/v1/calendar scoped
GET /api/v1/calendar/events pR
POST /api/v1/calendar/events self
DELETE /api/v1/calendar/events/{} own
GET /api/v1/calendar/events/{} own
PATCH /api/v1/calendar/events/{} own
PUT /api/v1/calendar/events/{}/attendees/me own
GET /api/v1/calendar/export.ics pub
DELETE /api/v1/comments/{} pM soft404
PATCH /api/v1/comments/{} pM
DELETE /api/v1/comments/{}/reactions/{} pW
PUT /api/v1/comments/{}/reactions/{} pW
GET /api/v1/comments/{}/revisions pM
GET /api/v1/dashboard/me self
GET /api/v1/dashboards self
POST /api/v1/dashboards self
DELETE /api/v1/dashboards/{} own
GET /api/v1/dashboards/{} ownA
PATCH /api/v1/dashboards/{} own
POST /api/v1/dashboards/{}/duplicate ownA
PUT /api/v1/dashboards/{}/gadgets own
GET /api/v1/dashboards/{}/gadgets/{}/data ownA
PUT /api/v1/dashboards/{}/shares own
GET /api/v1/favorites self
DELETE /api/v1/favorites/{}/{} self
PUT /api/v1/favorites/{}/{} self
GET /api/v1/filters self
POST /api/v1/filters self
DELETE /api/v1/filters/{} own
GET /api/v1/filters/{} own
PATCH /api/v1/filters/{} own
GET /api/v1/filters/{}/issues own
PUT /api/v1/filters/{}/shares own
DELETE /api/v1/filters/{}/subscription own soft404
GET /api/v1/filters/{}/subscription own soft404
POST /api/v1/filters/{}/subscription own
GET /api/v1/invitations/{} pub
POST /api/v1/invitations/{}/accept self
GET /api/v1/issues scoped
POST /api/v1/issues pW
POST /api/v1/issues/bulk pW
GET /api/v1/issues/export scoped
GET /api/v1/issues/search scoped
POST /api/v1/issues/search scoped
DELETE /api/v1/issues/{} pM nopos
GET /api/v1/issues/{} pR
PATCH /api/v1/issues/{} pW
GET /api/v1/issues/{}/activities pR
GET /api/v1/issues/{}/activity pR
PATCH /api/v1/issues/{}/assign pW
GET /api/v1/issues/{}/attachments pR
POST /api/v1/issues/{}/attachments pW
GET /api/v1/issues/{}/attachments/{} pR soft404
GET /api/v1/issues/{}/children pR
POST /api/v1/issues/{}/clone pW
GET /api/v1/issues/{}/comments pR
POST /api/v1/issues/{}/comments pW
DELETE /api/v1/issues/{}/comments/{} pM
PATCH /api/v1/issues/{}/comments/{} pM
POST /api/v1/issues/{}/convert pW
PATCH /api/v1/issues/{}/details pW
PUT /api/v1/issues/{}/estimates pW
GET /api/v1/issues/{}/links pR
POST /api/v1/issues/{}/links pW
DELETE /api/v1/issues/{}/links/{} pW
PATCH /api/v1/issues/{}/metadata pW
POST /api/v1/issues/{}/move pW
PATCH /api/v1/issues/{}/rank pW
POST /api/v1/issues/{}/restore pM soft404
GET /api/v1/issues/{}/transitions pW
POST /api/v1/issues/{}/transitions pW
DELETE /api/v1/issues/{}/vote pR
PUT /api/v1/issues/{}/vote pR
DELETE /api/v1/issues/{}/watchers pR
GET /api/v1/issues/{}/watchers pR
POST /api/v1/issues/{}/watchers pR
GET /api/v1/issues/{}/worklogs pR
POST /api/v1/issues/{}/worklogs pW
GET /api/v1/legal/documents pub
GET /api/v1/legal/documents/{} pub
GET /api/v1/me/api-tokens self
POST /api/v1/me/api-tokens self
DELETE /api/v1/me/api-tokens/{} self
DELETE /api/v1/me/calendar/feed self
GET /api/v1/me/calendar/feed self
POST /api/v1/me/calendar/feed self
GET /api/v1/me/consents self
POST /api/v1/me/consents self
DELETE /api/v1/me/consents/{} self
GET /api/v1/me/dashboard self
GET /api/v1/me/timesheet self
GET /api/v1/meta/countries pub
GET /api/v1/notes self
POST /api/v1/notes self
GET /api/v1/notes/tags self
DELETE /api/v1/notes/{} own
GET /api/v1/notes/{} own
PATCH /api/v1/notes/{} own
POST /api/v1/notes/{}/pin own
POST /api/v1/notes/{}/unpin own
GET /api/v1/notifications self
GET /api/v1/notifications/preferences self
PUT /api/v1/notifications/preferences self
POST /api/v1/notifications/read-all self
GET /api/v1/notifications/unread-count self
POST /api/v1/notifications/{}/read self
GET /api/v1/organizations scoped
POST /api/v1/organizations self
DELETE /api/v1/organizations/{} orgO nopos
GET /api/v1/organizations/{} orgR
PATCH /api/v1/organizations/{} orgW
GET /api/v1/organizations/{}/audit orgW
GET /api/v1/organizations/{}/audit/export orgW
GET /api/v1/organizations/{}/custom-fields orgR
POST /api/v1/organizations/{}/custom-fields orgW
DELETE /api/v1/organizations/{}/custom-fields/{} orgW
PATCH /api/v1/organizations/{}/custom-fields/{} orgW
PUT /api/v1/organizations/{}/custom-fields/{}/contexts orgW
GET /api/v1/organizations/{}/invitations orgW
POST /api/v1/organizations/{}/invitations orgW
DELETE /api/v1/organizations/{}/invitations/{} orgW
POST /api/v1/organizations/{}/invitations/{}/resend orgW
GET /api/v1/organizations/{}/issue-types orgR
POST /api/v1/organizations/{}/issue-types orgW
PUT /api/v1/organizations/{}/issue-types/reorder orgW
DELETE /api/v1/organizations/{}/issue-types/{} orgW
PATCH /api/v1/organizations/{}/issue-types/{} orgW
POST /api/v1/organizations/{}/leave orgR nopos
DELETE /api/v1/organizations/{}/logo orgW
GET /api/v1/organizations/{}/logo orgR soft404
PUT /api/v1/organizations/{}/logo orgW
GET /api/v1/organizations/{}/members orgR
DELETE /api/v1/organizations/{}/members/{} orgW
PATCH /api/v1/organizations/{}/members/{} orgW
DELETE /api/v1/organizations/{}/notification-scheme orgW
GET /api/v1/organizations/{}/notification-scheme orgR
PUT /api/v1/organizations/{}/notification-scheme orgW
GET /api/v1/organizations/{}/people/{} orgR
GET /api/v1/organizations/{}/priorities orgR
POST /api/v1/organizations/{}/priorities orgW
PUT /api/v1/organizations/{}/priorities/reorder orgW
DELETE /api/v1/organizations/{}/priorities/{} orgW
PATCH /api/v1/organizations/{}/priorities/{} orgW
GET /api/v1/organizations/{}/resolutions orgR
POST /api/v1/organizations/{}/resolutions orgW
PUT /api/v1/organizations/{}/resolutions/reorder orgW
DELETE /api/v1/organizations/{}/resolutions/{} orgW
PATCH /api/v1/organizations/{}/resolutions/{} orgW
GET /api/v1/organizations/{}/statuses orgR
POST /api/v1/organizations/{}/statuses orgW
PUT /api/v1/organizations/{}/statuses/reorder orgW
DELETE /api/v1/organizations/{}/statuses/{} orgW
PATCH /api/v1/organizations/{}/statuses/{} orgW
POST /api/v1/organizations/{}/transfer-ownership orgO nopos
GET /api/v1/organizations/{}/webhooks orgW
POST /api/v1/organizations/{}/webhooks orgW
DELETE /api/v1/organizations/{}/webhooks/{} orgW
GET /api/v1/organizations/{}/webhooks/{} orgW
PATCH /api/v1/organizations/{}/webhooks/{} orgW
GET /api/v1/organizations/{}/webhooks/{}/deliveries orgW
POST /api/v1/organizations/{}/webhooks/{}/deliveries/{}/redeliver orgW
POST /api/v1/organizations/{}/webhooks/{}/rotate-secret orgW
POST /api/v1/organizations/{}/webhooks/{}/test orgW
GET /api/v1/projects scoped
POST /api/v1/projects orgC
GET /api/v1/projects/key-available orgC
DELETE /api/v1/projects/{} pM nopos
GET /api/v1/projects/{} pR
PATCH /api/v1/projects/{} pM
POST /api/v1/projects/{}/archive pM nopos
GET /api/v1/projects/{}/automation-rules pM
POST /api/v1/projects/{}/automation-rules pM
GET /api/v1/projects/{}/backlog pR
GET /api/v1/projects/{}/board pR
POST /api/v1/projects/{}/board/cards/{}/move pW
PATCH /api/v1/projects/{}/board/columns/{} pM
GET /api/v1/projects/{}/boards pR
POST /api/v1/projects/{}/boards pM
GET /api/v1/projects/{}/components pR
POST /api/v1/projects/{}/components pM
DELETE /api/v1/projects/{}/components/{} pM
PATCH /api/v1/projects/{}/components/{} pM
GET /api/v1/projects/{}/export/issues.csv pR
GET /api/v1/projects/{}/export/project.json pM
GET /api/v1/projects/{}/fields pR
PUT /api/v1/projects/{}/fields pM
POST /api/v1/projects/{}/import/csv pM
POST /api/v1/projects/{}/import/csv/{}/start pM
GET /api/v1/projects/{}/import/jobs pM
GET /api/v1/projects/{}/import/jobs/{} pM
GET /api/v1/projects/{}/import/jobs/{}/errors.csv pM
GET /api/v1/projects/{}/issue-types pR
PUT /api/v1/projects/{}/issue-types pM
GET /api/v1/projects/{}/labels pR
POST /api/v1/projects/{}/labels pW
DELETE /api/v1/projects/{}/labels/{} pM
PATCH /api/v1/projects/{}/labels/{} pM
GET /api/v1/projects/{}/members pR
POST /api/v1/projects/{}/members pM
DELETE /api/v1/projects/{}/members/{} pM
PATCH /api/v1/projects/{}/members/{} pM
DELETE /api/v1/projects/{}/notification-scheme pM
GET /api/v1/projects/{}/notification-scheme pM
PUT /api/v1/projects/{}/notification-scheme pM
GET /api/v1/projects/{}/permissions pR
PUT /api/v1/projects/{}/permissions pM
GET /api/v1/projects/{}/permissions/me pR
POST /api/v1/projects/{}/permissions/reset pM
GET /api/v1/projects/{}/reports pR
GET /api/v1/projects/{}/reports/{} pR
POST /api/v1/projects/{}/restore pM
GET /api/v1/projects/{}/roadmap pR
GET /api/v1/projects/{}/roadmap/dependencies pR
PUT /api/v1/projects/{}/roadmap/rank pW
GET /api/v1/projects/{}/team-roles pM
PUT /api/v1/projects/{}/team-roles pM
DELETE /api/v1/projects/{}/team-roles/{} pM
GET /api/v1/projects/{}/time-summary pR
GET /api/v1/projects/{}/versions pR
POST /api/v1/projects/{}/versions pM
GET /api/v1/projects/{}/workflow pR
PUT /api/v1/projects/{}/workflow pM
POST /api/v1/projects/{}/workflow/migrate-issues pM
POST /api/v1/projects/{}/workflow/reset pM
POST /api/v1/projects/{}/workflow/validate pM
GET /api/v1/recent self
GET /api/v1/saved-filters self
POST /api/v1/saved-filters self
DELETE /api/v1/saved-filters/{} own
GET /api/v1/search scoped
GET /api/v1/search/issues scoped
POST /api/v1/search/issues scoped
GET /api/v1/search/suggest scoped
POST /api/v1/search/validate scoped
GET /api/v1/sprints pR
POST /api/v1/sprints pM
GET /api/v1/sprints/velocity pR
DELETE /api/v1/sprints/{} pM
GET /api/v1/sprints/{} pR
PATCH /api/v1/sprints/{} pM
GET /api/v1/sprints/{}/burndown pR
POST /api/v1/sprints/{}/close pM
POST /api/v1/sprints/{}/complete pM
GET /api/v1/sprints/{}/issues pR
DELETE /api/v1/sprints/{}/issues/{} pM
PUT /api/v1/sprints/{}/issues/{} pM
GET /api/v1/sprints/{}/report pR
POST /api/v1/sprints/{}/start pM
GET /api/v1/teams scoped
POST /api/v1/teams orgC
DELETE /api/v1/teams/{} orgW
GET /api/v1/teams/{} orgR
PATCH /api/v1/teams/{} orgW
GET /api/v1/teams/{}/members orgR
POST /api/v1/teams/{}/members orgW
DELETE /api/v1/teams/{}/members/{} orgW
PATCH /api/v1/teams/{}/members/{} orgW
GET /api/v1/users plat
GET /api/v1/users/me self
PATCH /api/v1/users/me self
DELETE /api/v1/users/me/avatar self
PUT /api/v1/users/me/avatar self
POST /api/v1/users/me/deactivate self nopos
POST /api/v1/users/me/password self nopos
POST /api/v1/users/me/phone/verification self
POST /api/v1/users/me/phone/verify self
GET /api/v1/users/me/sessions self
POST /api/v1/users/me/sessions/revoke-others self
DELETE /api/v1/users/me/sessions/{} self
GET /api/v1/users/search scoped
GET /api/v1/users/{}/avatar scoped soft404
PATCH /api/v1/users/{}/team plat
DELETE /api/v1/versions/{} pM
GET /api/v1/versions/{} pR
PATCH /api/v1/versions/{} pM
POST /api/v1/versions/{}/archive pM
GET /api/v1/versions/{}/notes pR
POST /api/v1/versions/{}/release pM
GET /api/v1/workflow-templates scoped
DELETE /api/v1/worklogs/{} pM
PATCH /api/v1/worklogs/{} pM
GET /health pub
`

// authzRoute is one classified route.
type authzRoute struct {
	Method, Path, Scope string
	NoPositive          bool
	Soft404             bool
}

func (r authzRoute) key() string { return r.Method + " " + r.Path }

// parseAuthzRegistry returns the classified routes keyed by "METHOD path".
func parseAuthzRegistry() (map[string]authzRoute, error) {
	out := map[string]authzRoute{}
	for i, line := range strings.Split(authzRegistryText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 || len(parts) > 4 {
			return nil, fmt.Errorf("registry line %d: want METHOD path scope [flags], got %q", i+1, line)
		}
		if !validScopes[parts[2]] {
			return nil, fmt.Errorf("registry line %d: unknown scope %q", i+1, parts[2])
		}
		route := authzRoute{Method: parts[0], Path: parts[1], Scope: parts[2]}
		if len(parts) == 4 {
			for _, flag := range strings.Split(parts[3], ",") {
				switch flag {
				case flagNoPosCtl:
					route.NoPositive = true
				case flagSoft404:
					route.Soft404 = true
				default:
					return nil, fmt.Errorf("registry line %d: unknown flag %q", i+1, flag)
				}
			}
		}
		if _, dup := out[route.key()]; dup {
			return nil, fmt.Errorf("registry line %d: duplicate route %s", i+1, route.key())
		}
		out[route.key()] = route
	}
	return out, nil
}

func sortedRoutes(m map[string]authzRoute) []authzRoute {
	out := make([]authzRoute, 0, len(m))
	for _, r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key() < out[j].key() })
	return out
}

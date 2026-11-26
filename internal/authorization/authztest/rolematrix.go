// Package authztest holds table-driven helpers for authorization tests. It depends only on the
// standard library so that any package's tests (domain, use cases, HTTP) can import it without
// creating an import cycle.
package authztest

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Actor is a subject that authorization rules are evaluated for.
//
// PlatformRole is the global role: "" (anonymous), "admin" or "user".
// OrgRole is the role in the organization that owns the project: "owner", "admin", "member",
// "guest", "none" (not a member) or "" (anonymous).
// ProjectRole is the explicit project membership: "lead", "contributor", "viewer", or "" when the
// actor has no project membership.
type Actor struct {
	Name         string
	PlatformRole string
	OrgRole      string
	ProjectRole  string
}

// StandardActors covers the platform roles and the project roles of the authorization model.
// The actor names are the row keys of ProjectMatrix and must not change without updating it.
//
// It is a var so that new scenarios can be appended; the helper never mutates it.
var StandardActors = []Actor{
	{Name: "anonymous", PlatformRole: "", OrgRole: "", ProjectRole: ""},
	{Name: "platform-admin", PlatformRole: "admin", OrgRole: "none", ProjectRole: ""},
	{Name: "org-owner", PlatformRole: "user", OrgRole: "owner", ProjectRole: ""},
	{Name: "org-admin", PlatformRole: "user", OrgRole: "admin", ProjectRole: ""},
	{Name: "org-member", PlatformRole: "user", OrgRole: "member", ProjectRole: ""},
	{Name: "org-guest", PlatformRole: "user", OrgRole: "guest", ProjectRole: ""},
	{Name: "lead", PlatformRole: "user", OrgRole: "member", ProjectRole: "lead"},
	{Name: "contributor", PlatformRole: "user", OrgRole: "member", ProjectRole: "contributor"},
	{Name: "viewer", PlatformRole: "user", OrgRole: "member", ProjectRole: "viewer"},
	{Name: "outsider", PlatformRole: "user", OrgRole: "none", ProjectRole: ""},
}

// RunMatrix runs one subtest per permission x actor, named "permission/actor". Each subtest
// asserts that check(actor, permission) equals the declarative table: the actor is allowed only
// if its name is listed under the permission in allowed; anything not listed is denied.
//
// The table is validated first and the test stops with t.Fatalf when it is invalid:
//   - an actor name in allowed that is not in actors,
//   - a permission in allowed that is not in permissions,
//   - a permission in permissions with no row in allowed. A row with an empty slice is valid and
//     means nobody is allowed, so every permission must be stated explicitly.
//   - duplicate actor or permission names.
func RunMatrix(t *testing.T, actors []Actor, permissions []string, allowed map[string][]string, check func(a Actor, permission string) bool) {
	t.Helper()
	if problems := validateMatrix(actors, permissions, allowed); len(problems) > 0 {
		t.Fatalf("invalid authorization matrix:\n  %s", strings.Join(problems, "\n  "))
	}
	for _, permission := range permissions {
		granted := make(map[string]bool, len(allowed[permission]))
		for _, name := range allowed[permission] {
			granted[name] = true
		}
		for _, actor := range actors {
			want := granted[actor.Name]
			t.Run(permission+"/"+actor.Name, func(t *testing.T) {
				if got := check(actor, permission); got != want {
					t.Errorf("%s for %s: got allowed=%v, want allowed=%v", permission, actor.Name, got, want)
				}
			})
		}
	}
}

// validateMatrix returns a description of every problem found in the table, in a stable order.
func validateMatrix(actors []Actor, permissions []string, allowed map[string][]string) []string {
	var problems []string

	actorNames := make(map[string]bool, len(actors))
	for _, actor := range actors {
		if actorNames[actor.Name] {
			problems = append(problems, fmt.Sprintf("duplicate actor %q", actor.Name))
		}
		actorNames[actor.Name] = true
	}

	permissionNames := make(map[string]bool, len(permissions))
	for _, permission := range permissions {
		if permissionNames[permission] {
			problems = append(problems, fmt.Sprintf("duplicate permission %q", permission))
		}
		permissionNames[permission] = true
		if _, ok := allowed[permission]; !ok {
			problems = append(problems, fmt.Sprintf("permission %q is not covered by the table", permission))
		}
	}

	keys := make([]string, 0, len(allowed))
	for permission := range allowed {
		keys = append(keys, permission)
	}
	sort.Strings(keys)
	for _, permission := range keys {
		if !permissionNames[permission] {
			problems = append(problems, fmt.Sprintf("table references unknown permission %q", permission))
		}
		for _, name := range allowed[permission] {
			if !actorNames[name] {
				problems = append(problems, fmt.Sprintf("table references unknown actor %q for permission %q", name, permission))
			}
		}
	}
	return problems
}

// Adding an actor is done in two steps, without changing RunMatrix: add it to StandardActors, then
// add its name to every row of ProjectMatrix where it is allowed. Actors not listed are denied, so a
// new actor silently denied everywhere is the most likely mistake: review every row.

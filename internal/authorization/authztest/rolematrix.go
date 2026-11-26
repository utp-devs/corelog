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
// PlatformRole is the global role: "" (anonymous), "admin", "agent" or "requester".
// ProjectRole is the project role: "lead", "contributor", "viewer", or "" when the actor has no
// membership in the project (outsider).
type Actor struct {
	Name         string
	PlatformRole string
	ProjectRole  string
}

// StandardActors covers the role model of the current code (docs/05-architecture.md section 3).
// The platform role of a project actor is "requester", the normal non-admin user. Agent and
// requester without a project role are denied everywhere; that is the point of those rows.
//
// It is a var so that later phases can extend it; the helper never mutates it.
var StandardActors = []Actor{
	{Name: "anonymous", PlatformRole: "", ProjectRole: ""},
	{Name: "admin", PlatformRole: "admin", ProjectRole: ""},
	{Name: "agent", PlatformRole: "agent", ProjectRole: ""},
	{Name: "requester", PlatformRole: "requester", ProjectRole: ""},
	{Name: "lead", PlatformRole: "requester", ProjectRole: "lead"},
	{Name: "contributor", PlatformRole: "requester", ProjectRole: "contributor"},
	{Name: "viewer", PlatformRole: "requester", ProjectRole: "viewer"},
	{Name: "outsider", PlatformRole: "requester", ProjectRole: ""},
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

// Adding a role (for example the organization roles of P2) is done in three steps, without
// changing RunMatrix:
//  1. Add the role as a field of Actor (for example OrgRole) and resolve it in the check func of
//     each test, the same way ProjectRole is resolved today.
//  2. Add one actor per new role to StandardActors (for example "org-owner", "org-member",
//     "org-guest"), so every permission row is checked against them.
//  3. Add the new actor's name to each permission row in the test tables where it is allowed.
//     Actors not listed are denied, so a new actor silently denied everywhere is the most likely
//     mistake: review every row when adding an actor.

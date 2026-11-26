package domain

import (
	"net"
	"strings"
)

// DefaultActorName is used in summaries when the payload has no actor_name.
const DefaultActorName = "Un usuario"

// SummaryFunc builds the Spanish summary of an entry from the resolved actor
// name and the event payload.
type SummaryFunc func(actor string, payload map[string]any) string

// Rule maps one event type to an audit entry shape.
type Rule struct {
	EntityType string
	Summary    SummaryFunc
}

// DefaultRules is the table of event types that produce audit entries.
// Event types missing from this table are ignored by the audit log.
var DefaultRules = map[string]Rule{
	"org.created": {EntityType: "organization", Summary: func(a string, p map[string]any) string {
		return a + " creó la organización" + named(p, "name")
	}},
	"org.updated": {EntityType: "organization", Summary: func(a string, p map[string]any) string {
		return a + " actualizó la organización" + named(p, "name")
	}},
	"org.deleted": {EntityType: "organization", Summary: func(a string, p map[string]any) string {
		return a + " eliminó la organización" + named(p, "name")
	}},
	"member.added": {EntityType: "member", Summary: func(a string, p map[string]any) string {
		return a + " añadió a " + target(p) + " a la organización"
	}},
	"member.removed": {EntityType: "member", Summary: func(a string, p map[string]any) string {
		return a + " quitó a " + target(p) + " de la organización"
	}},
	"member.role_changed": {EntityType: "member", Summary: func(a string, p map[string]any) string {
		if role := str(p, "role"); role != "" {
			return a + " cambió el rol de " + target(p) + " a " + role
		}
		return a + " cambió el rol de " + target(p)
	}},
	"invitation.created": {EntityType: "invitation", Summary: func(a string, p map[string]any) string {
		return a + " invitó a " + target(p) + " a la organización"
	}},
	"invitation.accepted": {EntityType: "invitation", Summary: func(a string, _ map[string]any) string {
		return a + " aceptó la invitación a la organización"
	}},
	"project.created": {EntityType: "project", Summary: func(a string, p map[string]any) string {
		return a + " creó el proyecto" + named(p, "name")
	}},
	"project.updated": {EntityType: "project", Summary: func(a string, p map[string]any) string {
		return a + " actualizó el proyecto" + named(p, "name")
	}},
	"project.archived": {EntityType: "project", Summary: func(a string, p map[string]any) string {
		return a + " archivó el proyecto" + named(p, "name")
	}},
	"project.deleted": {EntityType: "project", Summary: func(a string, p map[string]any) string {
		return a + " eliminó el proyecto" + named(p, "name")
	}},
	"project.member_added": {EntityType: "project", Summary: func(a string, p map[string]any) string {
		return a + " añadió a " + target(p) + " al proyecto" + named(p, "name")
	}},
	"project.role_changed": {EntityType: "project", Summary: func(a string, p map[string]any) string {
		if role := str(p, "role"); role != "" {
			return a + " cambió el rol de " + target(p) + " en el proyecto a " + role
		}
		return a + " cambió el rol de " + target(p) + " en el proyecto"
	}},
	"auth.login": {EntityType: "user", Summary: func(a string, _ map[string]any) string {
		return a + " inició sesión"
	}},
	"auth.login_failed": {EntityType: "user", Summary: func(a string, _ map[string]any) string {
		return a + " no pudo iniciar sesión"
	}},
	"auth.2fa_enabled": {EntityType: "user", Summary: func(a string, _ map[string]any) string {
		return a + " activó la autenticación en dos pasos"
	}},
	"auth.token_created": {EntityType: "api_token", Summary: func(a string, _ map[string]any) string {
		return a + " creó un token de acceso"
	}},
}

// Mapper turns source events into audit entries using a rule table.
type Mapper struct {
	rules map[string]Rule
}

// NewMapper returns a mapper for the given rules. The map is copied.
func NewMapper(rules map[string]Rule) *Mapper {
	copied := make(map[string]Rule, len(rules))
	for k, v := range rules {
		copied[k] = v
	}
	return &Mapper{rules: copied}
}

// Map builds the audit entry for src. It returns ok=false when the event type
// has no rule. The entry keeps the source event id for idempotent storage.
func (m *Mapper) Map(src SourceEvent) (Entry, bool) {
	rule, ok := m.rules[src.Type]
	if !ok {
		return Entry{}, false
	}
	actor := str(src.Payload, "actor_name")
	if actor == "" {
		actor = DefaultActorName
	}
	sourceID := src.ID
	return Entry{
		OrganizationID: src.OrgID,
		ProjectID:      src.ProjectID,
		ActorID:        src.ActorID,
		Action:         src.Type,
		EntityType:     rule.EntityType,
		EntityID:       src.EntityID,
		Summary:        rule.Summary(actor, src.Payload),
		Before:         object(src.Payload, "before"),
		After:          object(src.Payload, "after"),
		IP:             validIP(str(src.Payload, "ip")),
		UserAgent:      str(src.Payload, "user_agent"),
		SourceEventID:  &sourceID,
		CreatedAt:      src.CreatedAt,
	}, true
}

func str(p map[string]any, key string) string {
	s, _ := p[key].(string)
	return strings.TrimSpace(s)
}

// named returns ` "name"` when the payload has a name, otherwise an empty string.
func named(p map[string]any, key string) string {
	if s := str(p, key); s != "" {
		return ` "` + s + `"`
	}
	return ""
}

// target describes the person affected by a membership or role event.
func target(p map[string]any) string {
	for _, key := range []string{"member_name", "email"} {
		if s := str(p, key); s != "" {
			return s
		}
	}
	return "un miembro"
}

func object(p map[string]any, key string) map[string]any {
	m, _ := p[key].(map[string]any)
	if len(m) == 0 {
		return nil
	}
	return m
}

// validIP returns s when it is a valid IP address, otherwise an empty string.
func validIP(s string) string {
	if net.ParseIP(s) == nil {
		return ""
	}
	return s
}

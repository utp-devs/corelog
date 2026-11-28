package domain

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMapperRulesTable(t *testing.T) {
	t.Parallel()
	m := NewMapper(DefaultRules)
	when := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	org, project, actor, entity := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	tests := []struct {
		name        string
		eventType   string
		payload     map[string]any
		wantSummary string
		wantEntity  string
	}{
		{"org created with actor", "org.created", map[string]any{"actor_name": "Ana", "name": "Acme"}, "Ana creó la organización \"Acme\"", "organization"},
		{"org created without actor", "org.created", map[string]any{"name": "Acme"}, "Un usuario creó la organización \"Acme\"", "organization"},
		{"org updated", "org.updated", map[string]any{"actor_name": "Ana", "name": "Acme 2"}, "Ana actualizó la organización \"Acme 2\"", "organization"},
		{"org deleted", "org.deleted", map[string]any{"actor_name": "Ana"}, "Ana eliminó la organización", "organization"},
		{"member added", "member.added", map[string]any{"actor_name": "Ana", "email": "luis@example.com"}, "Ana añadió a luis@example.com a la organización", "member"},
		{"member removed", "member.removed", map[string]any{"actor_name": "Ana", "member_name": "Luis"}, "Ana quitó a Luis de la organización", "member"},
		{"member role changed", "member.role_changed", map[string]any{"actor_name": "Ana", "member_name": "Luis", "role": "admin"}, "Ana cambió el rol de Luis a admin", "member"},
		{"member role changed without role", "member.role_changed", map[string]any{"member_name": "Luis"}, "Un usuario cambió el rol de Luis", "member"},
		{"invitation created", "invitation.created", map[string]any{"actor_name": "Ana", "email": "x@example.com"}, "Ana invitó a x@example.com a la organización", "invitation"},
		{"invitation accepted", "invitation.accepted", map[string]any{}, "Un usuario aceptó la invitación a la organización", "invitation"},
		{"project created", "project.created", map[string]any{"actor_name": "Ana", "name": "Web"}, "Ana creó el proyecto \"Web\"", "project"},
		{"project updated", "project.updated", map[string]any{"actor_name": "Ana", "name": "Web"}, "Ana actualizó el proyecto \"Web\"", "project"},
		{"project archived", "project.archived", map[string]any{"actor_name": "Ana", "name": "Web"}, "Ana archivó el proyecto \"Web\"", "project"},
		{"project deleted", "project.deleted", map[string]any{"actor_name": "Ana", "name": "Web"}, "Ana eliminó el proyecto \"Web\"", "project"},
		{"project member added", "project.member_added", map[string]any{"actor_name": "Ana", "member_name": "Luis", "name": "Web"}, "Ana añadió a Luis al proyecto \"Web\"", "project"},
		{"project role changed", "project.role_changed", map[string]any{"actor_name": "Ana", "member_name": "Luis", "role": "lead"}, "Ana cambió el rol de Luis en el proyecto a lead", "project"},
		{"login", "auth.login", map[string]any{"actor_name": "Ana"}, "Ana inició sesión", "user"},
		{"login failed", "auth.login_failed", map[string]any{}, "Un usuario no pudo iniciar sesión", "user"},
		{"2fa enabled", "auth.2fa_enabled", map[string]any{"actor_name": "Ana"}, "Ana activó la autenticación en dos pasos", "user"},
		{"token created", "auth.token_created", map[string]any{"actor_name": "Ana"}, "Ana creó un token de acceso", "api_token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry, ok := m.Map(SourceEvent{
				ID: 7, OrgID: &org, ProjectID: &project, ActorID: &actor, EntityID: &entity,
				Type: tt.eventType, Payload: tt.payload, CreatedAt: when,
			})
			if !ok {
				t.Fatalf("Map(%s) ok = false, want true", tt.eventType)
			}
			if entry.Action != tt.eventType {
				t.Errorf("Action = %q, want %q", entry.Action, tt.eventType)
			}
			if entry.Summary != tt.wantSummary {
				t.Errorf("Summary = %q, want %q", entry.Summary, tt.wantSummary)
			}
			if entry.EntityType != tt.wantEntity {
				t.Errorf("EntityType = %q, want %q", entry.EntityType, tt.wantEntity)
			}
			if entry.OrganizationID == nil || *entry.OrganizationID != org {
				t.Errorf("OrganizationID not carried over")
			}
			if entry.ProjectID == nil || *entry.ProjectID != project || entry.ActorID == nil || *entry.ActorID != actor {
				t.Errorf("project/actor not carried over")
			}
			if entry.EntityID == nil || *entry.EntityID != entity {
				t.Errorf("EntityID not carried over")
			}
			if entry.SourceEventID == nil || *entry.SourceEventID != 7 {
				t.Errorf("SourceEventID = %v, want 7", entry.SourceEventID)
			}
			if !entry.CreatedAt.Equal(when) {
				t.Errorf("CreatedAt = %v, want %v", entry.CreatedAt, when)
			}
		})
	}
}

func TestMapperBeforeAfterAndRequestContext(t *testing.T) {
	t.Parallel()
	m := NewMapper(DefaultRules)
	before := map[string]any{"name": "Old"}
	after := map[string]any{"name": "New"}

	entry, ok := m.Map(SourceEvent{
		ID: 1, Type: "org.updated", EntityType: "organization",
		Payload: map[string]any{
			"name":       "New",
			"before":     before,
			"after":      after,
			"ip":         "10.1.2.3",
			"user_agent": "Mozilla/5.0",
		},
	})
	if !ok {
		t.Fatal("expected mapping")
	}
	if !reflect.DeepEqual(entry.Before, before) || !reflect.DeepEqual(entry.After, after) {
		t.Errorf("before/after = %v / %v", entry.Before, entry.After)
	}
	if entry.IP != "10.1.2.0" {
		t.Errorf("IP = %q, want the anonymized 10.1.2.0", entry.IP)
	}
	if entry.UserAgent != "Mozilla/5.0" {
		t.Errorf("UserAgent = %q", entry.UserAgent)
	}
}

func TestMapperDropsInvalidIPAndNonObjectDiffs(t *testing.T) {
	t.Parallel()
	m := NewMapper(DefaultRules)
	entry, ok := m.Map(SourceEvent{
		ID: 2, Type: "auth.login",
		Payload: map[string]any{"ip": "not-an-ip", "before": "text", "after": []any{1}},
	})
	if !ok {
		t.Fatal("expected mapping")
	}
	if entry.IP != "" {
		t.Errorf("IP = %q, want empty for invalid address", entry.IP)
	}
	if entry.Before != nil || entry.After != nil {
		t.Errorf("non-object diffs must be nil, got %v / %v", entry.Before, entry.After)
	}
}

func TestMapperIgnoresUnknownEventTypes(t *testing.T) {
	t.Parallel()
	m := NewMapper(DefaultRules)
	for _, typ := range []string{"sprint.unknown", "", "org.CREATED", "board.column_resized"} {
		if _, ok := m.Map(SourceEvent{ID: 3, Type: typ}); ok {
			t.Errorf("Map(%q) ok = true, want false", typ)
		}
	}
}

func TestNewMapperCopiesRules(t *testing.T) {
	t.Parallel()
	rules := map[string]Rule{"custom.thing": {EntityType: "thing", Summary: func(a string, _ map[string]any) string { return a }}}
	m := NewMapper(rules)
	delete(rules, "custom.thing")
	if _, ok := m.Map(SourceEvent{Type: "custom.thing"}); !ok {
		t.Fatal("mapper must keep its own copy of the rules")
	}
}

func TestAnonymizeIP(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"10.1.2.3":              "10.1.2.0",
		"2001:db8:abcd:1234::1": "2001:db8:abcd::",
		"::ffff:192.168.7.9":    "192.168.7.0",
		"nope":                  "",
		"":                      "",
	} {
		if got := AnonymizeIP(in); got != want {
			t.Errorf("AnonymizeIP(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMapperKeepsRequestIDAndMapsNewEvents(t *testing.T) {
	t.Parallel()
	m := NewMapper(DefaultRules)
	entry, ok := m.Map(SourceEvent{ID: 9, Type: "issue.created", Payload: map[string]any{"actor_name": "Ana", "key": "WEB-1", "request_id": "req-7"}})
	if !ok || entry.Summary != "Ana creó la incidencia WEB-1" || entry.RequestID != "req-7" || entry.EntityType != "issue" {
		t.Fatalf("entry = %+v ok=%v", entry, ok)
	}
	for _, typ := range []string{"auth.password_reset", "auth.password_reset_requested", "auth.email_verified", "auth.account_locked", "auth.login_failed", "user.two_factor_enabled", "legal.consent_recorded", "version.released", "comment.created"} {
		if _, ok := m.Map(SourceEvent{ID: 1, Type: typ}); !ok {
			t.Errorf("%s must be audited", typ)
		}
	}
}

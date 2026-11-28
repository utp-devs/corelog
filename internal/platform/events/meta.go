package events

import (
	"context"

	"github.com/google/uuid"
)

// RequestMeta describes the HTTP request that caused an event. The publisher copies it into the payload so
// consumers such as the audit log can record where a change came from.
type RequestMeta struct {
	IP        string
	UserAgent string
	RequestID string
}

type metaKey struct{}

// WithRequestMeta returns a context carrying the request metadata.
func WithRequestMeta(ctx context.Context, m RequestMeta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

// RequestMetaFrom returns the request metadata of ctx, if any.
func RequestMetaFrom(ctx context.Context) (RequestMeta, bool) {
	m, ok := ctx.Value(metaKey{}).(RequestMeta)
	return m, ok
}

// withMeta returns a copy of payload with the request metadata added; keys already present win.
func withMeta(ctx context.Context, payload map[string]any) map[string]any {
	m, hasMeta := RequestMetaFrom(ctx)
	o, hasOrigin := OriginFrom(ctx)
	if !hasMeta && !hasOrigin {
		return payload
	}
	out := make(map[string]any, len(payload)+6)
	for k, v := range payload {
		out[k] = v
	}
	set := func(k, v string) {
		if _, exists := out[k]; !exists && v != "" {
			out[k] = v
		}
	}
	set("ip", m.IP)
	set("user_agent", m.UserAgent)
	set("request_id", m.RequestID)
	if hasOrigin {
		out[PayloadAutomationRuleID] = o.RuleID.String()
		out[PayloadAutomationName] = o.Name
		out[PayloadAutomationDepth] = o.Depth
	}
	return out
}

// Origin marks events produced while an automation rule runs. Consumers use it to avoid loops: an event that
// carries the id of the rule that is evaluating it is ignored, and the depth bounds chains of rules.
type Origin struct {
	RuleID uuid.UUID
	Name   string
	Depth  int
}

// Payload keys written for events published under an Origin.
const (
	PayloadAutomationRuleID = "automation_rule_id"
	PayloadAutomationName   = "automation_rule_name"
	PayloadAutomationDepth  = "automation_depth"
)

type originKey struct{}

// WithOrigin returns a context whose published events carry the automation origin in their payload.
func WithOrigin(ctx context.Context, o Origin) context.Context {
	return context.WithValue(ctx, originKey{}, o)
}

// OriginFrom returns the automation origin of ctx, if any.
func OriginFrom(ctx context.Context) (Origin, bool) {
	o, ok := ctx.Value(originKey{}).(Origin)
	return o, ok
}

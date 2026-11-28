package events

import "context"

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
	m, ok := RequestMetaFrom(ctx)
	if !ok {
		return payload
	}
	out := make(map[string]any, len(payload)+3)
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
	return out
}

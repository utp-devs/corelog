package middleware

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the best-effort address of the caller: the first X-Forwarded-For entry when present,
// otherwise the host of RemoteAddr. It is meant for audit records and must not be used for access decisions.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

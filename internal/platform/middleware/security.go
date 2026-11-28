package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/jhonsferg/corelog/internal/platform/events"
)

// SecurityHeaders sets the defensive headers of an API: no sniffing, no framing, a CSP that allows nothing
// (the API serves data, not documents), a strict referrer policy and no caching of API responses. hsts adds
// Strict-Transport-Security and must only be on when the API is reached through TLS.
func SecurityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			h.Set("Cross-Origin-Resource-Policy", "same-site")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			if h.Get("Cache-Control") == "" && r.URL.Path != "/health" {
				// Handlers that serve cacheable files (avatars, logos) set their own value before writing,
				// but this runs first, so default to no-store and let them override.
				h.Set("Cache-Control", "no-store")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// BodyLimit rejects JSON request bodies larger than maxBytes with 413 (by Content-Length) and caps streams
// without a length. Multipart uploads are skipped: their handlers enforce the upload policy of each route.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if maxBytes <= 0 || r.Body == nil || r.Body == http.NoBody ||
				strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/") {
				next.ServeHTTP(w, r)
				return
			}
			if r.ContentLength > maxBytes {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_, _ = w.Write([]byte(`{"error":{"message":"La solicitud supera el tamaño máximo permitido","code":"request.too_large"}}`))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// RequestMeta stores the client address, user agent and request id in the context so the event publisher
// can attach them to the events of the request. It must run after chi's RequestID middleware.
func RequestMeta(trusted *TrustedProxies) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ua := r.UserAgent()
			if len(ua) > 300 {
				ua = ua[:300]
			}
			ctx := events.WithRequestMeta(r.Context(), events.RequestMeta{
				IP: trusted.ClientIP(r), UserAgent: ua, RequestID: middleware.GetReqID(r.Context()),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TrustedProxies decides whose X-Forwarded-For header is believed. The zero value (or nil) trusts nobody, so
// the client address is always the TCP peer.
type TrustedProxies struct {
	nets []*net.IPNet
}

// ParseTrustedProxies parses CIDRs and single addresses. Invalid entries are returned in bad.
func ParseTrustedProxies(entries []string) (tp *TrustedProxies, bad []string) {
	tp = &TrustedProxies{}
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if !strings.Contains(e, "/") {
			ip := net.ParseIP(e)
			if ip == nil {
				bad = append(bad, e)
				continue
			}
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			e = e + "/" + strconv.Itoa(bits)
		}
		_, n, err := net.ParseCIDR(e)
		if err != nil {
			bad = append(bad, e)
			continue
		}
		tp.nets = append(tp.nets, n)
	}
	return tp, bad
}

func (t *TrustedProxies) trusts(ip net.IP) bool {
	if t == nil {
		return false
	}
	for _, n := range t.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP returns the TCP peer address unless the peer is a trusted proxy, in which case it walks
// X-Forwarded-For from the right and returns the first address that is not a trusted proxy.
func (t *TrustedProxies) ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil || !t.trusts(peer) {
		return host
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(parts[i]))
		if ip == nil {
			continue
		}
		if !t.trusts(ip) {
			return ip.String()
		}
	}
	return host
}

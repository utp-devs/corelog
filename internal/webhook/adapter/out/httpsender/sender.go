// Package httpsender posts webhook deliveries with SSRF protection: every resolved address is checked against the
// blocklist, the connection goes to the checked address (so a second DNS answer cannot redirect it) and
// redirects are never followed.
package httpsender

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/jhonsferg/corelog/internal/webhook/application/port"
	"github.com/jhonsferg/corelog/internal/webhook/domain"
)

// ErrBlockedAddress is returned when a destination resolves to a forbidden address.
var ErrBlockedAddress = errors.New("destino bloqueado: direccion no permitida")

// Resolver looks up the addresses of a host. net.DefaultResolver satisfies it through LookupIP.
type Resolver interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

// Sender implements port.Sender.
type Sender struct {
	client       *http.Client
	allowPrivate bool
	resolver     Resolver
}

var _ port.Sender = (*Sender)(nil)

// Options configures a Sender. Zero values select the defaults.
type Options struct {
	// AllowPrivate disables the address blocklist. Development only.
	AllowPrivate bool
	Resolver     Resolver
	Timeout      time.Duration
	// Dial replaces the connection function, for tests.
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)
}

// New returns a sender.
func New(o Options) *Sender {
	s := &Sender{allowPrivate: o.AllowPrivate, resolver: o.Resolver}
	if s.resolver == nil {
		s.resolver = net.DefaultResolver
	}
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = domain.RequestTimeout
	}
	dial := o.Dial
	if dial == nil {
		d := &net.Dialer{Timeout: timeout}
		dial = d.DialContext
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DisableKeepAlives:     true,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ip, err := s.pick(ctx, host)
			if err != nil {
				return nil, err
			}
			return dial(ctx, network, net.JoinHostPort(ip.String(), port))
		},
	}
	s.client = &http.Client{
		Timeout:   timeout,
		Transport: transport,
		// Redirects are never followed: a 3xx is reported as the response and counts as a failed delivery.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return s
}

// pick resolves host and returns the first address, failing when any answer is blocked: a mixed answer is a
// rebinding signal.
func (s *Sender) pick(ctx context.Context, host string) (net.IP, error) {
	if ip, ok := domain.ParseIP(host); ok {
		if !s.allowPrivate && domain.IsBlockedIP(ip) {
			return nil, ErrBlockedAddress
		}
		return ip, nil
	}
	ips, err := s.resolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("no se pudo resolver el destino: %w", err)
	}
	if len(ips) == 0 {
		return nil, errors.New("el destino no tiene direcciones")
	}
	if !s.allowPrivate {
		for _, ip := range ips {
			if domain.IsBlockedIP(ip) {
				return nil, ErrBlockedAddress
			}
		}
	}
	return ips[0], nil
}

// Send posts the body and reads at most the excerpt size of the response.
func (s *Sender) Send(ctx context.Context, req port.SendRequest) port.SendResult {
	start := time.Now()
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return port.SendResult{Err: errors.New("URL no valida"), Duration: time.Since(start)}
	}
	for k, v := range req.Headers {
		r.Header.Set(k, v)
	}
	resp, err := s.client.Do(r)
	if err != nil {
		return port.SendResult{Err: unwrap(err), Duration: time.Since(start)}
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, domain.MaxExcerptBytes))
	return port.SendResult{Status: resp.StatusCode, Body: body, Duration: time.Since(start)}
}

// unwrap reports the root cause without the URL that net/http adds.
func unwrap(err error) error {
	if errors.Is(err, ErrBlockedAddress) {
		return ErrBlockedAddress
	}
	var ue interface{ Unwrap() error }
	if errors.As(err, &ue) && ue.Unwrap() != nil {
		return ue.Unwrap()
	}
	return err
}

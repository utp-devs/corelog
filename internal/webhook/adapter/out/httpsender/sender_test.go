package httpsender

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jhonsferg/corelog/internal/webhook/application/port"
	"github.com/jhonsferg/corelog/internal/webhook/domain"
)

type fakeResolver struct {
	answers map[string][]net.IP
	calls   atomic.Int32
}

func (r *fakeResolver) LookupIP(_ context.Context, _, host string) ([]net.IP, error) {
	r.calls.Add(1)
	if ips, ok := r.answers[host]; ok {
		return ips, nil
	}
	return nil, errors.New("no such host")
}

func ips(list ...string) []net.IP {
	out := make([]net.IP, len(list))
	for i, s := range list {
		out[i] = net.ParseIP(s)
	}
	return out
}

func TestSSRFBlocklistAtSendTime(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer target.Close()
	res := &fakeResolver{answers: map[string][]net.IP{
		"loopback.example":  ips("127.0.0.1"),
		"private.example":   ips("10.1.2.3"),
		"metadata.example":  ips("169.254.169.254"),
		"v6.example":        ips("::1"),
		"mapped.example":    ips("::ffff:127.0.0.1"),
		"mixed.example":     ips("93.184.216.34", "192.168.0.10"), // a rebinding style answer: one public, one private
		"public.example":    ips("93.184.216.34"),
		"public-v6.example": ips("2606:4700:4700::1111"),
	}}
	var dialed atomic.Value
	s := New(Options{Resolver: res, Timeout: time.Second, Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialed.Store(addr)
		return (&net.Dialer{}).DialContext(ctx, network, target.Listener.Addr().String())
	}})

	for _, host := range []string{"loopback.example", "private.example", "metadata.example", "v6.example", "mapped.example", "mixed.example"} {
		out := s.Send(context.Background(), port.SendRequest{URL: "http://" + host + "/hook", Body: []byte("{}")})
		if !errors.Is(out.Err, ErrBlockedAddress) {
			t.Errorf("%s: err = %v, status %d", host, out.Err, out.Status)
		}
	}
	for _, literal := range []string{"http://127.0.0.1:1/", "http://[::1]:1/", "http://169.254.169.254/latest", "http://10.0.0.1/", "http://[::ffff:10.0.0.1]/"} {
		if out := s.Send(context.Background(), port.SendRequest{URL: literal}); !errors.Is(out.Err, ErrBlockedAddress) {
			t.Errorf("%s: err = %v", literal, out.Err)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("the blocked destinations were contacted %d times", hits.Load())
	}

	// A public name is accepted and the connection goes to the address that was checked.
	out := s.Send(context.Background(), port.SendRequest{URL: "http://public.example:8080/hook", Body: []byte("{}")})
	if out.Err != nil || out.Status != http.StatusOK || hits.Load() != 1 {
		t.Fatalf("public: %+v hits %d", out, hits.Load())
	}
	if got := dialed.Load(); got != "93.184.216.34:8080" {
		t.Fatalf("dialed %v, want the resolved address", got)
	}
	if out := s.Send(context.Background(), port.SendRequest{URL: "http://public-v6.example/"}); out.Err != nil {
		t.Fatalf("public v6: %v", out.Err)
	}
	if dialed.Load() != "[2606:4700:4700::1111]:80" {
		t.Fatalf("dialed %v", dialed.Load())
	}
	if out := s.Send(context.Background(), port.SendRequest{URL: "http://unknown.example/"}); out.Err == nil || errors.Is(out.Err, ErrBlockedAddress) {
		t.Fatalf("unresolvable host: %v", out.Err)
	}
}

func TestRedirectsAreNeverFollowed(t *testing.T) {
	var internal atomic.Int32
	inner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { internal.Add(1) }))
	defer inner.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, inner.URL+"/secret", http.StatusFound)
	}))
	defer redirector.Close()

	// Private targets are allowed here so the first hop works; the redirect to the second server is still not followed.
	s := New(Options{AllowPrivate: true, Timeout: time.Second})
	out := s.Send(context.Background(), port.SendRequest{URL: redirector.URL, Body: []byte("{}")})
	if out.Err != nil || out.Status != http.StatusFound || internal.Load() != 0 {
		t.Fatalf("redirect: %+v internal hits %d", out, internal.Load())
	}
}

func TestSendResponseHandling(t *testing.T) {
	var gotHeader, gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader.Store(r.Header.Get("X-CoreLog-Event"))
		buf := make([]byte, 16)
		n, _ := r.Body.Read(buf)
		gotBody.Store(string(buf[:n]))
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("x", 5000)))
	}))
	defer srv.Close()
	s := New(Options{AllowPrivate: true, Timeout: time.Second})
	out := s.Send(context.Background(), port.SendRequest{URL: srv.URL, Body: []byte(`{"a":1}`), Headers: map[string]string{"X-CoreLog-Event": "ping"}})
	if out.Err != nil || out.Status != 500 || len(out.Body) != domain.MaxExcerptBytes {
		t.Fatalf("response: status %d body %d err %v", out.Status, len(out.Body), out.Err)
	}
	if gotHeader.Load() != "ping" || gotBody.Load() != `{"a":1}` {
		t.Fatalf("request: %v %v", gotHeader.Load(), gotBody.Load())
	}
	if out := s.Send(context.Background(), port.SendRequest{URL: "http://%zz"}); out.Err == nil {
		t.Fatal("invalid URL must fail")
	}
}

func TestSendTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	s := New(Options{AllowPrivate: true, Timeout: 50 * time.Millisecond})
	start := time.Now()
	if out := s.Send(context.Background(), port.SendRequest{URL: srv.URL}); out.Err == nil {
		t.Fatalf("timeout expected, got status %d", out.Status)
	}
	if time.Since(start) > time.Second {
		t.Fatal("the request did not time out")
	}
}

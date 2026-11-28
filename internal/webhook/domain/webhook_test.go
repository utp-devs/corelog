package domain

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestSignGolden(t *testing.T) {
	body := []byte(`{"event":"ping","id":"evt_1"}`)
	const want = "sha256=2c3f39b595c26ca0f7d9d0b1431618d7978f5b0975a64c7170a899db53ac89f3"
	if got := Sign("whsec_test_secret", 1700000000, body); got != want {
		t.Fatalf("Sign = %s, want %s", got, want)
	}
	if !Verify("whsec_test_secret", 1700000000, body, want) {
		t.Fatal("a valid signature must verify")
	}
	for name, ok := range map[string]bool{
		"other secret":    Verify("whsec_other", 1700000000, body, want),
		"other timestamp": Verify("whsec_test_secret", 1700000001, body, want),
		"other body":      Verify("whsec_test_secret", 1700000000, append(body, ' '), want),
		"no prefix":       Verify("whsec_test_secret", 1700000000, body, strings.TrimPrefix(want, "sha256=")),
	} {
		if ok {
			t.Errorf("%s verified", name)
		}
	}
}

func TestBlockedIPTable(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.255.255.254", "10.0.0.1", "10.255.255.255", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "169.254.0.1", "100.64.0.1", "100.100.100.200", "0.0.0.0", "0.1.2.3", "224.0.0.1", "239.255.255.255",
		"240.0.0.1", "255.255.255.255", "198.18.0.1", "192.0.0.1", "192.0.2.5", "198.51.100.7", "203.0.113.9",
		"::1", "::", "fe80::1", "fc00::1", "fd12:3456::1", "ff02::1", "2001:db8::1", "64:ff9b::7f00:1",
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:169.254.169.254",
	}
	for _, s := range blocked {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("bad fixture %q", s)
		}
		if !IsBlockedIP(ip) {
			t.Errorf("%s must be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "172.15.255.255", "172.32.0.1", "100.63.255.255", "2606:4700:4700::1111", "::ffff:8.8.8.8"}
	for _, s := range allowed {
		if IsBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s must be allowed", s)
		}
	}
	if !IsBlockedIP(nil) || !IsBlockedIP(net.IP{1, 2, 3}) {
		t.Error("invalid addresses must be blocked")
	}
}

func TestParseIP(t *testing.T) {
	for in, want := range map[string]bool{"127.0.0.1": true, "[::1]": true, "fe80::1%eth0": true, "example.com": false, "": false, "999.1.1.1": false} {
		if _, ok := ParseIP(in); ok != want {
			t.Errorf("ParseIP(%q) = %v", in, ok)
		}
	}
}

func TestValidateURL(t *testing.T) {
	good := []string{"https://example.com/hook", "http://example.com:8080/x?y=1", "https://hooks.example.org"}
	for _, u := range good {
		if _, err := ValidateURL(u, false); err != nil {
			t.Errorf("%s rejected: %v", u, err)
		}
	}
	bad := []string{
		"", "   ", "ftp://example.com", "file:///etc/passwd", "javascript:alert(1)", "//example.com", "example.com",
		"https://user:pass@example.com", "https://", "http://localhost/x", "http://api.localhost/x", "http://db.internal/x",
		"http://127.0.0.1:8080", "http://[::1]/", "http://10.0.0.5/", "http://169.254.169.254/latest/meta-data", "http://[::ffff:127.0.0.1]/",
		"https://example.com/" + strings.Repeat("a", 2100),
	}
	for _, u := range bad {
		if _, err := ValidateURL(u, false); err == nil {
			t.Errorf("%q accepted", u)
		}
	}
	if _, err := ValidateURL("http://127.0.0.1:8080/x", true); err != nil {
		t.Errorf("private target must be accepted when explicitly allowed: %v", err)
	}
	if _, err := ValidateURL("ftp://127.0.0.1", true); err == nil {
		t.Error("the scheme is checked even when private targets are allowed")
	}
}

func TestValidateNameEventsFilter(t *testing.T) {
	if n, err := ValidateName(" Hook "); err != nil || n != "Hook" {
		t.Fatalf("name %q %v", n, err)
	}
	if _, err := ValidateName(strings.Repeat("x", 121)); err == nil {
		t.Fatal("long name accepted")
	}
	evs, err := ValidateEvents([]string{"issue.created", "issue.created", "*"})
	if err != nil || len(evs) != 2 {
		t.Fatalf("events %v %v", evs, err)
	}
	for _, bad := range [][]string{nil, {}, {"nope"}, make([]string, 21)} {
		if _, err := ValidateEvents(bad); err == nil {
			t.Errorf("events %v accepted", bad)
		}
	}
	empty, long := "  ", strings.Repeat("x", 2001)
	if q, err := ValidateFilter(&empty); err != nil || q != nil {
		t.Fatalf("blank filter: %v %v", q, err)
	}
	if _, err := ValidateFilter(&long); err == nil {
		t.Fatal("long filter accepted")
	}
	if q, err := ValidateFilter(nil); err != nil || q != nil {
		t.Fatal("nil filter")
	}
	ok := " status = Open "
	if q, _ := ValidateFilter(&ok); q == nil || *q != "status = Open" {
		t.Fatal("filter must be trimmed")
	}
}

func TestSecrets(t *testing.T) {
	a, _ := NewSecret()
	b, _ := NewSecret()
	if !strings.HasPrefix(a, "whsec_") || len(a) != len("whsec_")+48 || a == b {
		t.Fatalf("secrets %q %q", a, b)
	}
}

func TestRetrySchedule(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	want := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour}
	for attempt := 1; attempt <= 5; attempt++ {
		status, next := NextAttempt(false, attempt, now)
		if status != StatusFailed || next == nil || !next.Equal(now.Add(want[attempt-1])) {
			t.Fatalf("attempt %d: %s %v", attempt, status, next)
		}
	}
	if status, next := NextAttempt(false, 6, now); status != StatusDead || next != nil {
		t.Fatalf("sixth failure: %s %v", status, next)
	}
	if status, next := NextAttempt(true, 3, now); status != StatusSuccess || next != nil {
		t.Fatalf("success: %s %v", status, next)
	}
}

func TestExcerpt(t *testing.T) {
	long := strings.Repeat("á", 2000) // 4000 bytes
	got := Excerpt([]byte(long))
	if len(got) > MaxExcerptBytes || strings.ContainsRune(got, '�') {
		t.Fatalf("excerpt %d bytes", len(got))
	}
	if got := Excerpt([]byte("ok\x00z")); got != "okz" {
		t.Fatalf("NUL kept: %q", got)
	}
}

func TestWebhookSubscribes(t *testing.T) {
	w := Webhook{Events: []string{"issue.created"}}
	if !w.Subscribes("issue.created") || w.Subscribes("issue.updated") {
		t.Fatal("subscription")
	}
	if !(Webhook{Events: []string{"*"}}).Subscribes("anything") {
		t.Fatal("wildcard")
	}
	if !IsSubscribable("*") || IsSubscribable("ping") {
		t.Fatal("catalog")
	}
}

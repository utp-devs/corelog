package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseUserAgent(t *testing.T) {
	tests := []struct {
		name, ua, browser, os, label string
	}{
		{"chrome windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Chrome", "Windows", "Chrome en Windows"},
		{"edge", "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 Chrome/120.0 Safari/537.36 Edg/120.0", "Edge", "Windows", "Edge en Windows"},
		{"opera", "Mozilla/5.0 (Windows NT 10.0) Chrome/120.0 Safari/537.36 OPR/105.0", "Opera", "Windows", "Opera en Windows"},
		{"firefox linux", "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0", "Firefox", "Linux", "Firefox en Linux"},
		{"safari mac", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15", "Safari", "macOS", "Safari en macOS"},
		{"safari iphone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 Version/17.1 Mobile/15E148 Safari/604.1", "Safari", "iOS", "Safari en iOS"},
		{"chrome android", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36", "Chrome", "Android", "Chrome en Android"},
		{"chrome ios", "Mozilla/5.0 (iPad; CPU OS 17_1 like Mac OS X) AppleWebKit/605.1.15 CriOS/120.0 Mobile/15E148 Safari/604.1", "Chrome", "iOS", "Chrome en iOS"},
		{"ie", "Mozilla/4.0 (compatible; MSIE 8.0; Windows NT 6.1)", "Internet Explorer", "Windows", "Internet Explorer en Windows"},
		{"curl", "curl/8.4.0", "curl", "", "curl"},
		{"os only", "Dalvik (Linux; Android 9)", "", "Android", "un dispositivo con Android"},
		{"chromeos", "Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) Chrome/120.0", "Chrome", "ChromeOS", "Chrome en ChromeOS"},
		{"unknown", "", "", "", "un dispositivo desconocido"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := ParseUserAgent(tt.ua)
			if d.Browser != tt.browser || d.OS != tt.os || d.Label() != tt.label {
				t.Errorf("got %+v %q", d, d.Label())
			}
		})
	}
}

func TestMaskIP(t *testing.T) {
	tests := map[string]string{
		"203.0.113.57":         "203.0.113.x",
		"10.0.0.1":             "10.0.0.x",
		" 192.168.1.255 ":      "192.168.1.x",
		"::ffff:198.51.100.9":  "198.51.100.x",
		"2001:db8:1:2:3:4:5:6": "2001:db8:1:2::/64",
		"fe80::1%eth0":         "fe80::/64",
		"":                     "",
		"not an ip":            "",
		"203.0.113.57:8080":    "",
	}
	for in, want := range tests {
		if got := MaskIP(in); got != want {
			t.Errorf("MaskIP(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewLoginAlert(t *testing.T) {
	user, session := uuid.New(), uuid.New()
	login := NewLogin{
		UserID: user, SessionID: session, IP: "203.0.113.57", At: time.Date(2026, 11, 27, 22, 41, 7, 0, time.UTC),
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36",
	}
	n, err := NewLoginAlert(login)
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != KindNewLogin || n.Link != SecurityLink || n.UserID != user {
		t.Fatalf("notification: %+v", n)
	}
	if want := "Se inició sesión desde Chrome en Windows (IP 203.0.113.x) el 27/11/2026 22:41 UTC"; n.Message != want {
		t.Fatalf("message = %q", n.Message)
	}
	if n.Data["title"] != "Nuevo inicio de sesión" || n.Data["session_id"] != session.String() || n.Data["ip"] != "203.0.113.x" {
		t.Fatalf("data: %v", n.Data)
	}
	actions := n.Data["actions"].([]map[string]string)
	if actions[0]["label"] != "No fui yo" || actions[0]["path"] != "/users/me/sessions/"+session.String() || actions[1]["label"] != "Ver mis sesiones" {
		t.Fatalf("actions: %v", actions)
	}

	login.IP = ""
	n, _ = NewLoginAlert(login)
	if !strings.Contains(n.Message, "(IP desconocida)") {
		t.Fatalf("message = %q", n.Message)
	}
	if _, err := NewLoginAlert(NewLogin{}); err == nil {
		t.Fatal("a login without user is invalid")
	}
}

func TestFingerprintDependsOnDeviceAndMaskedAddress(t *testing.T) {
	a := NewLogin{UserAgent: "ua", IP: "203.0.113.5"}
	b := NewLogin{UserAgent: "ua", IP: "203.0.113.200"}
	c := NewLogin{UserAgent: "ua", IP: "203.0.114.5"}
	d := NewLogin{UserAgent: "other", IP: "203.0.113.5"}
	if a.Fingerprint() != b.Fingerprint() {
		t.Error("same masked address and agent must match")
	}
	if a.Fingerprint() == c.Fingerprint() || a.Fingerprint() == d.Fingerprint() || len(a.Fingerprint()) != 64 {
		t.Error("different address or agent must differ")
	}
}

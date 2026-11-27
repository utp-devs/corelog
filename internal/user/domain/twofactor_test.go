package domain

import (
	"encoding/base32"
	"net/url"
	"strings"
	"testing"
	"time"
)

// rfc6238Secret is the ASCII secret "12345678901234567890" used by the RFC 6238 SHA-1 test vectors.
var rfc6238Secret = TOTPSecret("12345678901234567890")

// The RFC 6238 appendix B table lists 8-digit codes; the 6-digit codes are their last six digits.
func TestTOTPAtMatchesRFC6238Vectors(t *testing.T) {
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	for _, tc := range cases {
		step := StepOf(time.Unix(tc.unix, 0))
		if got := TOTPAt(rfc6238Secret, step); got != tc.want {
			t.Errorf("T=%d: got %s, want %s", tc.unix, got, tc.want)
		}
	}
}

func TestVerifyTOTPWindow(t *testing.T) {
	base := time.Unix(1111111111, 0)
	step := StepOf(base)
	code := TOTPAt(rfc6238Secret, step)

	cases := []struct {
		name string
		at   time.Time
		ok   bool
	}{
		{"same step", base, true},
		{"one step later", base.Add(TOTPPeriod), true},
		{"one step earlier", base.Add(-TOTPPeriod), true},
		{"two steps later", base.Add(2 * TOTPPeriod), false},
		{"two steps earlier", base.Add(-2 * TOTPPeriod), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matched, ok := VerifyTOTP(rfc6238Secret, code, tc.at, nil)
			if ok != tc.ok {
				t.Fatalf("ok=%v, want %v", ok, tc.ok)
			}
			if ok && matched != step {
				t.Fatalf("matched step %d, want %d", matched, step)
			}
		})
	}
}

func TestVerifyTOTPRejectsReplayedStep(t *testing.T) {
	now := time.Unix(1111111111, 0)
	step := StepOf(now)
	code := TOTPAt(rfc6238Secret, step)

	used := step
	if _, ok := VerifyTOTP(rfc6238Secret, code, now, &used); ok {
		t.Fatal("a code whose step was already used must be rejected")
	}
	earlier := step - 1
	if _, ok := VerifyTOTP(rfc6238Secret, code, now, &earlier); !ok {
		t.Fatal("a code from a later step than the last used one must be accepted")
	}
}

func TestVerifyTOTPRejectsMalformedInput(t *testing.T) {
	now := time.Unix(1111111111, 0)
	for _, code := range []string{"", "12345", "1234567", "abcdef", "12 34 5x"} {
		if _, ok := VerifyTOTP(rfc6238Secret, code, now, nil); ok {
			t.Errorf("code %q must be rejected", code)
		}
	}
	valid := TOTPAt(rfc6238Secret, StepOf(now))
	spaced := valid[:3] + " " + valid[3:]
	if _, ok := VerifyTOTP(rfc6238Secret, spaced, now, nil); !ok {
		t.Error("a code typed with a space must be accepted")
	}
}

func TestSecretBase32RoundTripAndLength(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatalf("NewTOTPSecret: %v", err)
	}
	if len(secret) != TOTPSecretSize {
		t.Fatalf("secret length %d, want %d", len(secret), TOTPSecretSize)
	}
	encoded := secret.Base32()
	if len(encoded) != 32 {
		t.Fatalf("base32 length %d, want 32", len(encoded))
	}
	back, err := ParseTOTPSecret(encoded)
	if err != nil {
		t.Fatalf("ParseTOTPSecret: %v", err)
	}
	if string(back) != string(secret) {
		t.Fatal("base32 round trip changed the secret")
	}
	if _, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(encoded); err != nil {
		t.Fatalf("encoded secret is not base32: %v", err)
	}
}

func TestOTPAuthURI(t *testing.T) {
	uri := OTPAuthURI("CoreLog", "ana@example.com", rfc6238Secret)
	if !strings.HasPrefix(uri, "otpauth://totp/CoreLog:ana@example.com?") {
		t.Fatalf("unexpected label: %s", uri)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("parse uri: %v", err)
	}
	q := parsed.Query()
	if q.Get("issuer") != "CoreLog" || q.Get("algorithm") != "SHA1" || q.Get("digits") != "6" || q.Get("period") != "30" {
		t.Fatalf("unexpected parameters: %v", q)
	}
	if q.Get("secret") != rfc6238Secret.Base32() {
		t.Fatal("secret parameter does not match the base32 secret")
	}
}

func TestGenerateRecoveryCodes(t *testing.T) {
	codes, err := GenerateRecoveryCodes(RecoveryCodeCount)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes: %v", err)
	}
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("got %d codes, want %d", len(codes), RecoveryCodeCount)
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if len(c) != RecoveryCodeLength {
			t.Fatalf("code %q has length %d", c, len(c))
		}
		for _, r := range c {
			if !strings.ContainsRune(RecoveryCodeAlphabet, r) {
				t.Fatalf("code %q contains %q outside the alphabet", c, r)
			}
		}
		if seen[c] {
			t.Fatalf("duplicate code %q", c)
		}
		seen[c] = true
	}
}

func TestHashRecoveryCodeNormalizesInput(t *testing.T) {
	want := HashRecoveryCode("ABCDE23456")
	for _, typed := range []string{"abcde23456", "ABCDE-23456", "abcde-23456", " ABCDE 23456 "} {
		if string(HashRecoveryCode(typed)) != string(want) {
			t.Errorf("input %q must hash like the canonical code", typed)
		}
	}
	if len(want) != 32 {
		t.Fatalf("digest length %d, want 32", len(want))
	}
}

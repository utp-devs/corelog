package domain

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestNewInvitationValidation(t *testing.T) {
	org := uuid.New()
	by := uuid.New()
	hash := HashToken("token")
	cases := []struct {
		name  string
		email string
		role  Role
		hash  string
		code  string
	}{
		{"valid member", "Ana@Example.com ", RoleMember, hash, ""},
		{"valid admin", "a@b.co", RoleAdmin, hash, ""},
		{"valid guest", "g@b.co", RoleGuest, hash, ""},
		{"owner never invited", "a@b.co", RoleOwner, hash, "org.role_invalid"},
		{"unknown role", "a@b.co", Role("boss"), hash, "org.role_invalid"},
		{"empty email", "   ", RoleMember, hash, "org.invitation_email_invalid"},
		{"no at sign", "anaexample.com", RoleMember, hash, "org.invitation_email_invalid"},
		{"display name form", "Ana <ana@example.com>", RoleMember, hash, "org.invitation_email_invalid"},
		{"too long", strings.Repeat("a", 252) + "@b.co", RoleMember, hash, "org.invitation_email_invalid"},
		{"empty hash", "a@b.co", RoleMember, "", ""},
	}
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.FixedZone("X", 3600))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv, err := NewInvitation(org, tc.email, tc.role, by, tc.hash, now)
			if tc.name == "empty hash" {
				if err == nil || inv != nil {
					t.Fatalf("empty hash must fail, got %+v, %v", inv, err)
				}
				return
			}
			if tc.code != "" {
				if apperror.CodeOf(err) != tc.code {
					t.Fatalf("code = %q, want %q (%v)", apperror.CodeOf(err), tc.code, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if inv.ID != uuid.Nil {
				t.Fatal("ID must stay nil until the repository assigns it")
			}
			if inv.Email != strings.ToLower(strings.TrimSpace(tc.email)) {
				t.Fatalf("email not normalized: %q", inv.Email)
			}
			if !inv.CreatedAt.Equal(now.UTC()) || !inv.ExpiresAt.Equal(now.UTC().Add(InvitationTTL)) {
				t.Fatalf("times = %v / %v", inv.CreatedAt, inv.ExpiresAt)
			}
		})
	}
}

func TestInvitationTTLIsSevenDays(t *testing.T) {
	if InvitationTTL != 7*24*time.Hour {
		t.Fatalf("InvitationTTL = %v", InvitationTTL)
	}
}

func TestInvitationStatusPrecedence(t *testing.T) {
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	at := now.Add(-time.Minute)
	cases := []struct {
		name     string
		accepted *time.Time
		revoked  *time.Time
		expires  time.Time
		want     InvitationStatus
	}{
		{"pending", nil, nil, now.Add(time.Hour), InvitationPending},
		{"expired at boundary", nil, nil, now, InvitationExpired},
		{"expired", nil, nil, now.Add(-time.Second), InvitationExpired},
		{"revoked", nil, &at, now.Add(time.Hour), InvitationRevoked},
		{"accepted", &at, nil, now.Add(time.Hour), InvitationAccepted},
		{"accepted beats expired", &at, nil, now.Add(-time.Hour), InvitationAccepted},
		{"revoked beats expired", nil, &at, now.Add(-time.Hour), InvitationRevoked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := &Invitation{AcceptedAt: tc.accepted, RevokedAt: tc.revoked, ExpiresAt: tc.expires}
			if got := inv.Status(now); got != tc.want {
				t.Fatalf("status = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestInvitationTransitions(t *testing.T) {
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	pending := func() *Invitation {
		return &Invitation{TokenHash: "h", ExpiresAt: now.Add(time.Hour)}
	}
	t.Run("accept pending", func(t *testing.T) {
		inv := pending()
		if err := inv.Accept(now); err != nil || inv.AcceptedAt == nil {
			t.Fatalf("accept: %v", err)
		}
		if err := inv.Accept(now); apperror.CodeOf(err) != "org.invitation_used" {
			t.Fatalf("second accept = %v", err)
		}
	})
	t.Run("revoke pending then reject", func(t *testing.T) {
		inv := pending()
		if err := inv.Revoke(now); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		if err := inv.Accept(now); apperror.CodeOf(err) != "org.invitation_revoked" {
			t.Fatalf("accept after revoke = %v", err)
		}
		if err := inv.Revoke(now); apperror.CodeOf(err) != "org.invitation_revoked" {
			t.Fatalf("second revoke = %v", err)
		}
	})
	t.Run("expired cannot be accepted, revoked or renewed", func(t *testing.T) {
		inv := pending()
		late := now.Add(2 * time.Hour)
		if err := inv.Accept(late); apperror.CodeOf(err) != "org.invitation_expired" {
			t.Fatalf("accept = %v", err)
		}
		if err := inv.Revoke(late); apperror.CodeOf(err) != "org.invitation_expired" {
			t.Fatalf("revoke = %v", err)
		}
		if err := inv.Renew("new", late); apperror.CodeOf(err) != "org.invitation_expired" {
			t.Fatalf("renew = %v", err)
		}
	})
	t.Run("renew replaces hash and restarts expiry", func(t *testing.T) {
		inv := pending()
		if err := inv.Renew("new-hash", now); err != nil {
			t.Fatalf("renew: %v", err)
		}
		if inv.TokenHash != "new-hash" || !inv.ExpiresAt.Equal(now.Add(InvitationTTL)) {
			t.Fatalf("renewed = %+v", inv)
		}
		if err := inv.Renew("", now); err == nil {
			t.Fatal("empty hash must be rejected")
		}
	})
	t.Run("retire only expired", func(t *testing.T) {
		inv := pending()
		if err := inv.Retire(now); apperror.CodeOf(err) != "org.invitation_exists" {
			t.Fatalf("retire pending = %v", err)
		}
		late := now.Add(2 * time.Hour)
		if err := inv.Retire(late); err != nil || inv.RevokedAt == nil {
			t.Fatalf("retire expired: %v", err)
		}
	})
}

func TestNewTokenAndHash(t *testing.T) {
	plain, hash, err := NewToken()
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(plain)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token must be 32 random bytes in base64url: len=%d err=%v", len(raw), err)
	}
	if hash != HashToken(plain) || len(hash) != 64 || hash == plain {
		t.Fatalf("hash = %q", hash)
	}
	other, _, _ := NewToken()
	if other == plain {
		t.Fatal("tokens must differ")
	}
	// Known vector: SHA-256 of "abc".
	const abc = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := HashToken("abc"); got != abc {
		t.Fatalf("HashToken(abc) = %s", got)
	}
}

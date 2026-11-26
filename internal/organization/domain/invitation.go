package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// InvitationTTL is how long an invitation stays usable after it is created or resent.
const InvitationTTL = 7 * 24 * time.Hour

// maxEmailLength matches organization_invitations.email (VARCHAR(255)).
const maxEmailLength = 255

// InvitationStatus is the state of an invitation at a given instant. It is derived, never stored.
type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
	InvitationRevoked  InvitationStatus = "revoked"
	InvitationExpired  InvitationStatus = "expired"
)

// Invitation is an offer to join an organization with a role. The plain token is never stored; only
// its SHA-256 hash (TokenHash) is. ID stays uuid.Nil until the repository assigns it.
type Invitation struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Email          string
	Role           Role
	TokenHash      string
	InvitedBy      uuid.UUID
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
}

// NormalizeEmail trims and lowercases an email address. It does not validate it.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail checks an already normalized address: it must parse as a bare address (no display
// name) and fit the column size.
func ValidateEmail(email string) error {
	if email == "" || len(email) > maxEmailLength {
		return ErrInvitationEmailInvalid
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return ErrInvitationEmailInvalid
	}
	return nil
}

// NewInvitation builds a pending invitation. The email is normalized and validated, the role must be
// admin, member or guest (owners are never invited), and the token hash must come from HashToken.
func NewInvitation(orgID uuid.UUID, email string, role Role, invitedBy uuid.UUID, tokenHash string, now time.Time) (*Invitation, error) {
	email = NormalizeEmail(email)
	if err := ValidateEmail(email); err != nil {
		return nil, err
	}
	if role == RoleOwner || !role.IsValid() {
		return nil, ErrRoleInvalid
	}
	if tokenHash == "" {
		return nil, fmt.Errorf("invitation: empty token hash")
	}
	at := now.UTC()
	return &Invitation{
		OrganizationID: orgID,
		Email:          email,
		Role:           role,
		TokenHash:      tokenHash,
		InvitedBy:      invitedBy,
		ExpiresAt:      at.Add(InvitationTTL),
		CreatedAt:      at,
	}, nil
}

// Status derives the state at instant now. Precedence: accepted, revoked, expired, pending.
func (i *Invitation) Status(now time.Time) InvitationStatus {
	switch {
	case i.AcceptedAt != nil:
		return InvitationAccepted
	case i.RevokedAt != nil:
		return InvitationRevoked
	case !now.UTC().Before(i.ExpiresAt):
		return InvitationExpired
	default:
		return InvitationPending
	}
}

// RequirePending returns nil while the invitation can still be accepted, resent or revoked. Otherwise
// it returns the error that explains why not.
func (i *Invitation) RequirePending(now time.Time) error {
	switch i.Status(now) {
	case InvitationPending:
		return nil
	case InvitationAccepted:
		return ErrInvitationUsed
	case InvitationRevoked:
		return ErrInvitationRevoked
	default:
		return ErrInvitationExpired
	}
}

// Renew replaces the token hash and restarts the expiry window. Only pending invitations may be renewed.
func (i *Invitation) Renew(tokenHash string, now time.Time) error {
	if err := i.RequirePending(now); err != nil {
		return err
	}
	if tokenHash == "" {
		return fmt.Errorf("invitation: empty token hash")
	}
	i.TokenHash = tokenHash
	i.ExpiresAt = now.UTC().Add(InvitationTTL)
	return nil
}

// Revoke marks a pending invitation as revoked. Only pending invitations may be revoked.
func (i *Invitation) Revoke(now time.Time) error {
	if err := i.RequirePending(now); err != nil {
		return err
	}
	at := now.UTC()
	i.RevokedAt = &at
	return nil
}

// Retire revokes an expired invitation so that a new one for the same email can replace it. Pending
// invitations are never retired, and accepted or revoked ones are already closed.
func (i *Invitation) Retire(now time.Time) error {
	switch i.Status(now) {
	case InvitationExpired:
		at := now.UTC()
		i.RevokedAt = &at
		return nil
	case InvitationPending:
		return ErrInvitationExists
	default:
		return nil
	}
}

// Accept marks a pending invitation as accepted.
func (i *Invitation) Accept(now time.Time) error {
	if err := i.RequirePending(now); err != nil {
		return err
	}
	at := now.UTC()
	i.AcceptedAt = &at
	return nil
}

// NewToken returns a new plain token (32 random bytes, base64url without padding) and its hash.
// The plain value goes to the invitee once; only the hash may be stored.
func NewToken() (plain, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("invitation: generate token: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, HashToken(plain), nil
}

// HashToken returns the hex SHA-256 of a plain token, the only form in which tokens are stored.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

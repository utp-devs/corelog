package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/phone"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// errPhoneVerificationUnavailable answers when no SMS delivery is wired.
var errPhoneVerificationUnavailable = apperror.WithCode(apperror.KindConflict, "user.phone_verification_unavailable",
	"la verificacion de telefono no esta disponible")

// orgRoleOwner and orgRoleAdmin are the organization_members.role values that may see member phones.
const (
	orgRoleOwner = "owner"
	orgRoleAdmin = "admin"
)

// randomCode returns a uniformly random numeric code with leading zeros preserved.
func randomCode(digits int) (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", digits, n), nil
}

// CanViewMemberPhones reports whether the actor owns or administers the organization.
func (s *UserService) CanViewMemberPhones(ctx context.Context, actorID, orgID uuid.UUID) (bool, error) {
	role, found, err := s.repo.FindOrganizationRole(ctx, orgID, actorID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, domain.ErrNotOrganizationMember
	}
	return role == orgRoleOwner || role == orgRoleAdmin, nil
}

// RequestPhoneVerification issues a 6-digit code for the current phone of the user and hands it to the
// SMS sender. At most three codes can be requested per hour; each one lives ten minutes.
func (s *UserService) RequestPhoneVerification(ctx context.Context, userID uuid.UUID) (port.PhoneVerificationReceipt, error) {
	if s.phones == nil || s.sms == nil {
		return port.PhoneVerificationReceipt{}, errPhoneVerificationUnavailable
	}
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return port.PhoneVerificationReceipt{}, err
	}
	if user.Phone == nil {
		return port.PhoneVerificationReceipt{}, domain.ErrPhoneNotSet
	}
	if user.PhoneVerified() {
		return port.PhoneVerificationReceipt{}, domain.ErrPhoneAlreadyVerified
	}

	code, err := s.codeSource(domain.PhoneCodeDigits)
	if err != nil {
		return port.PhoneVerificationReceipt{}, apperror.Wrap(apperror.KindInternal, "failed to generate code", err)
	}
	now := s.clock.Now()
	v := &domain.PhoneVerification{
		UserID:    userID,
		E164:      user.Phone.E164,
		CodeHash:  domain.HashPhoneCode(code),
		ExpiresAt: now.Add(domain.PhoneCodeTTL),
		CreatedAt: now,
	}
	if err := s.phones.CreateWithinLimit(ctx, v, now.Add(-domain.PhoneCodeSendWindow), domain.PhoneCodeMaxSends); err != nil {
		return port.PhoneVerificationReceipt{}, err
	}
	if err := s.sms.SendVerificationCode(ctx, user.Phone.E164, code); err != nil {
		return port.PhoneVerificationReceipt{}, apperror.Wrap(apperror.KindInternal, "failed to send verification code", err)
	}
	return port.PhoneVerificationReceipt{ExpiresAt: v.ExpiresAt, Destination: phone.Mask(user.Phone.E164)}, nil
}

// VerifyPhone checks the code against the latest open verification of the current number. Every try counts
// as an attempt; after five the code is dead. A number already verified by another account is refused.
func (s *UserService) VerifyPhone(ctx context.Context, userID uuid.UUID, code string) (*domain.User, error) {
	if s.phones == nil {
		return nil, errPhoneVerificationUnavailable
	}
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Phone == nil {
		return nil, domain.ErrPhoneNotSet
	}
	if user.PhoneVerified() {
		return nil, domain.ErrPhoneAlreadyVerified
	}

	v, err := s.phones.FindLatestOpen(ctx, userID, user.Phone.E164)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	if !v.Usable(now) {
		return nil, domain.ErrPhoneCodeExpired
	}
	attempts, err := s.phones.RegisterAttempt(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	if attempts > domain.PhoneCodeMaxAttempts {
		return nil, domain.ErrPhoneCodeAttempts
	}
	if !v.Matches(strings.TrimSpace(code)) {
		if attempts >= domain.PhoneCodeMaxAttempts {
			return nil, domain.ErrPhoneCodeAttempts
		}
		return nil, domain.ErrPhoneCodeInvalid
	}

	user.MarkPhoneVerified(now)
	run := func(ctx context.Context) error {
		if err := s.repo.Update(ctx, user); err != nil {
			return err
		}
		return s.phones.Consume(ctx, userID, v.ID, now)
	}
	if s.tx == nil {
		err = run(ctx)
	} else {
		err = s.tx.InTx(ctx, run)
	}
	if err != nil {
		if errors.Is(err, domain.ErrPhoneTaken) {
			return nil, domain.ErrPhoneTaken
		}
		return nil, err
	}
	return user, nil
}

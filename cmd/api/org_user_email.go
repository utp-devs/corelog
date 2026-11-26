package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// orgUserEmails resolves the account email for the organization hexagon, which needs it to check that
// an invitation is accepted by the invited address. The JWT carries no email.
type orgUserEmails struct {
	db *sqlx.DB
}

// EmailOf returns the email of the user. A missing account is reported as unauthorized, because the
// token refers to a user that no longer exists.
func (e orgUserEmails) EmailOf(ctx context.Context, userID uuid.UUID) (string, error) {
	var email string
	err := e.db.GetContext(ctx, &email, `SELECT email FROM users WHERE id = $1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", apperror.New(apperror.KindUnauthorized, "not authenticated")
	}
	if err != nil {
		return "", fmt.Errorf("organization user email: %w", err)
	}
	return email, nil
}

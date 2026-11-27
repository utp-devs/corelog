// Package sms holds the SMS delivery adapters. The real provider integration is plugged in by replacing
// the sender wired in cmd/api; NoopSender is the default.
package sms

import (
	"context"
	"log/slog"

	"github.com/jhonsferg/corelog/internal/platform/phone"
	"github.com/jhonsferg/corelog/internal/user/application/port"
)

// NoopSender does not deliver anything. It logs the masked destination and the code length; the code
// itself is logged only when development is true (APP_ENV=development).
type NoopSender struct {
	log         *slog.Logger
	development bool
}

// NewNoopSender builds the default sender. A nil logger uses slog.Default.
func NewNoopSender(log *slog.Logger, development bool) *NoopSender {
	if log == nil {
		log = slog.Default()
	}
	return &NoopSender{log: log, development: development}
}

var _ port.SMSSender = (*NoopSender)(nil)

func (s *NoopSender) SendVerificationCode(ctx context.Context, e164, code string) error {
	attrs := []any{"to", phone.Mask(e164), "code_length", len(code)}
	if s.development {
		attrs = append(attrs, "code", code)
	}
	s.log.InfoContext(ctx, "sms not delivered (no provider configured)", attrs...)
	return nil
}

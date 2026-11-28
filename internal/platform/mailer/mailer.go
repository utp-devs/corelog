// Package mailer is the outgoing email port with an SMTP adapter and a log adapter for development.
package mailer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Message is one outgoing email with a plain text body and an optional HTML alternative.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Mailer delivers email messages.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// ErrInvalidMessage is returned for messages with a bad recipient or header injection attempts.
var ErrInvalidMessage = errors.New("mailer: invalid message")

// SMTPConfig configures the SMTP adapter. Credentials are never logged.
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

// SMTP sends messages through net/smtp (STARTTLS is negotiated by the library when the server offers it).
type SMTP struct {
	cfg  SMTPConfig
	send func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
	now  func() time.Time
}

var _ Mailer = (*SMTP)(nil)

// NewSMTP returns an SMTP mailer.
func NewSMTP(cfg SMTPConfig) *SMTP {
	return &SMTP{cfg: cfg, send: smtp.SendMail, now: time.Now}
}

// Send delivers the message. The context is only checked before dialing: net/smtp has no cancellation.
func (s *SMTP) Send(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	from, err := mail.ParseAddress(s.cfg.From)
	if err != nil {
		return fmt.Errorf("mailer: invalid from address: %w", err)
	}
	to, err := mail.ParseAddress(msg.To)
	if err != nil || strings.ContainsAny(msg.Subject, "\r\n") {
		return ErrInvalidMessage
	}
	var auth smtp.Auth
	if s.cfg.User != "" {
		auth = smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
	}
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	if err := s.send(addr, auth, from.Address, []string{to.Address}, s.build(from, to, msg)); err != nil {
		return fmt.Errorf("mailer: smtp send: %w", err)
	}
	return nil
}

func (s *SMTP) build(from, to *mail.Address, msg Message) []byte {
	var b strings.Builder
	boundary := newBoundary()
	h := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	h("From", from.String())
	h("To", to.String())
	h("Subject", mimeHeader(msg.Subject))
	h("Date", s.now().UTC().Format(time.RFC1123Z))
	h("MIME-Version", "1.0")
	if msg.HTML == "" {
		h("Content-Type", `text/plain; charset="utf-8"`)
		h("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		b.WriteString(qp(msg.Text))
		return []byte(b.String())
	}
	h("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	b.WriteString("\r\n")
	for _, part := range []struct{ typ, body string }{{"text/plain", msg.Text}, {"text/html", msg.HTML}} {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: " + part.typ + "; charset=\"utf-8\"\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		b.WriteString(qp(part.body) + "\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

func qp(s string) string {
	var b strings.Builder
	w := quotedprintable.NewWriter(&b)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	return b.String()
}

func mimeHeader(s string) string {
	for _, r := range s {
		if r > 127 {
			return mimeQ(s)
		}
	}
	return s
}

func mimeQ(s string) string {
	return "=?UTF-8?Q?" + strings.ReplaceAll(strings.TrimRight(qpHeader(s), "\r\n"), " ", "_") + "?="
}

func qpHeader(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == ' ' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "=%02X", c)
	}
	return b.String()
}

func newBoundary() string {
	raw := make([]byte, 12)
	_, _ = rand.Read(raw)
	return "corelog-" + hex.EncodeToString(raw)
}

// Log is the adapter used when no SMTP relay is configured. It never delivers anything. The body (which holds
// the action link) is logged only when ShowBody is true, which the wiring sets for APP_ENV=development.
type Log struct {
	Logger   *slog.Logger
	ShowBody bool
}

var _ Mailer = Log{}

// Send logs the delivery without the recipient address; in development it also logs the text body.
func (l Log) Send(_ context.Context, msg Message) error {
	logger := l.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if l.ShowBody {
		logger.Info("email not delivered (no SMTP configured)", slog.String("subject", msg.Subject), slog.String("body", msg.Text))
		return nil
	}
	logger.Info("email not delivered (no SMTP configured)", slog.String("subject", msg.Subject))
	return nil
}

// Recorder keeps sent messages in memory (tests).
type Recorder struct {
	Messages []Message
	Err      error
}

var _ Mailer = (*Recorder)(nil)

// Send records the message or returns Err.
func (r *Recorder) Send(_ context.Context, msg Message) error {
	if r.Err != nil {
		return r.Err
	}
	r.Messages = append(r.Messages, msg)
	return nil
}

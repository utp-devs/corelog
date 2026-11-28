package mailer

import (
	"context"
	"errors"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestSMTPSendBuildsMessage(t *testing.T) {
	t.Parallel()
	var gotFrom string
	var gotTo []string
	var gotMsg string
	s := NewSMTP(SMTPConfig{Host: "smtp.test", Port: 2525, User: "u", Password: "p", From: "CoreLog <no-reply@corelog.test>"})
	s.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	s.send = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		if addr != "smtp.test:2525" || a == nil {
			t.Fatalf("addr=%q auth=%v", addr, a)
		}
		gotFrom, gotTo, gotMsg = from, to, string(msg)
		return nil
	}
	err := s.Send(context.Background(), Message{To: "ana@example.com", Subject: "Recuperación", Text: "hola", HTML: "<p>hola</p>"})
	if err != nil {
		t.Fatal(err)
	}
	if gotFrom != "no-reply@corelog.test" || len(gotTo) != 1 || gotTo[0] != "ana@example.com" {
		t.Fatalf("envelope %q %v", gotFrom, gotTo)
	}
	for _, want := range []string{"multipart/alternative", "text/html", "Subject: =?UTF-8?Q?Recuperaci=C3=B3n?="} {
		if !strings.Contains(gotMsg, want) {
			t.Fatalf("message misses %q:\n%s", want, gotMsg)
		}
	}
}

func TestSMTPRejectsHeaderInjection(t *testing.T) {
	t.Parallel()
	s := NewSMTP(SMTPConfig{Host: "h", Port: 25, From: "a@b.test"})
	s.send = func(string, smtp.Auth, string, []string, []byte) error { t.Fatal("must not send"); return nil }
	if err := s.Send(context.Background(), Message{To: "a@b.test", Subject: "x\r\nBcc: evil@x.test"}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("err = %v", err)
	}
	if err := s.Send(context.Background(), Message{To: "not an address", Subject: "x"}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("err = %v", err)
	}
}

func TestLogMailerShowsBodyOnlyWhenAllowed(t *testing.T) {
	t.Parallel()
	if err := (Log{}).Send(context.Background(), Message{Subject: "s", Text: "secret-link"}); err != nil {
		t.Fatal(err)
	}
}

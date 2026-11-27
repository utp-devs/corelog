package sms

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestNoopSenderLogging(t *testing.T) {
	for _, dev := range []bool{false, true} {
		var buf bytes.Buffer
		s := NewNoopSender(slog.New(slog.NewTextHandler(&buf, nil)), dev)
		if err := s.SendVerificationCode(context.Background(), "+51912345678", "424242"); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if strings.Contains(out, "912345678") || !strings.Contains(out, "+51******678") || !strings.Contains(out, "code_length=6") {
			t.Errorf("dev=%v: destination not masked or length missing: %s", dev, out)
		}
		if has := strings.Contains(out, "424242"); has != dev {
			t.Errorf("dev=%v: code logged = %v", dev, has)
		}
	}
}

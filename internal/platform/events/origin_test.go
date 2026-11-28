package events

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestOriginIsWrittenToThePayload(t *testing.T) {
	rule := uuid.New()
	ctx := WithOrigin(context.Background(), Origin{RuleID: rule, Name: "Regla", Depth: 2})
	if o, ok := OriginFrom(ctx); !ok || o.RuleID != rule {
		t.Fatalf("origin = %+v %v", o, ok)
	}
	got := withMeta(ctx, map[string]any{"key": "A-1"})
	if got[PayloadAutomationRuleID] != rule.String() || got[PayloadAutomationName] != "Regla" || got[PayloadAutomationDepth] != 2 || got["key"] != "A-1" {
		t.Fatalf("payload = %v", got)
	}
	if _, ok := OriginFrom(context.Background()); ok {
		t.Fatal("no origin expected")
	}
	plain := map[string]any{"a": 1}
	if out := withMeta(context.Background(), plain); len(out) != 1 {
		t.Fatalf("payload without context must be untouched: %v", out)
	}
	// Request metadata and origin combine.
	ctx = WithRequestMeta(ctx, RequestMeta{IP: "10.0.0.1"})
	if out := withMeta(ctx, nil); out["ip"] != "10.0.0.1" || out[PayloadAutomationDepth] != 2 {
		t.Fatalf("combined payload = %v", out)
	}
}

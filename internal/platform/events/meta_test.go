package events

import (
	"context"
	"testing"
)

func TestWithMetaCopiesRequestMetadataWithoutOverridingPayload(t *testing.T) {
	t.Parallel()
	ctx := WithRequestMeta(context.Background(), RequestMeta{IP: "10.0.0.1", UserAgent: "ua", RequestID: "r1"})
	in := map[string]any{"ip": "keep", "name": "x"}
	out := withMeta(ctx, in)
	if out["ip"] != "keep" || out["user_agent"] != "ua" || out["request_id"] != "r1" || out["name"] != "x" {
		t.Fatalf("payload = %v", out)
	}
	if _, polluted := in["user_agent"]; polluted {
		t.Fatal("the caller payload must not be mutated")
	}
	if got := withMeta(context.Background(), in); len(got) != 2 {
		t.Fatalf("without metadata the payload is untouched, got %v", got)
	}
}

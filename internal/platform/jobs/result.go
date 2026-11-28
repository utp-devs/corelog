package jobs

import (
	"context"
	"sync"
)

type resultKey struct{}

// resultSink collects the summary a running job reports about itself.
type resultSink struct {
	mu   sync.Mutex
	text string
}

func (r *resultSink) set(text string) {
	r.mu.Lock()
	r.text = text
	r.mu.Unlock()
}

func (r *resultSink) get() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.text
}

func withResultSink(ctx context.Context, sink *resultSink) context.Context {
	return context.WithValue(ctx, resultKey{}, sink)
}

// ReportResult stores a short summary of the current job run (for example "labels=2 sprints=1"). It is shown next
// to the run in the jobs console. Outside a scheduled run it does nothing.
func ReportResult(ctx context.Context, summary string) {
	if sink, ok := ctx.Value(resultKey{}).(*resultSink); ok {
		sink.set(summary)
	}
}

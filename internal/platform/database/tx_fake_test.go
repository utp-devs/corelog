package database

import (
	"context"
	"errors"
	"testing"
)

func TestFakeTxManager(t *testing.T) {
	boom := errors.New("boom")
	fnErr := errors.New("fn failed")

	tests := []struct {
		name      string
		mgr       *FakeTxManager
		fn        func(ctx context.Context) error
		wantErr   error
		wantRun   bool
		wantCalls int
	}{
		{"runs fn and returns nil", &FakeTxManager{}, func(context.Context) error { return nil }, nil, true, 1},
		{"returns fn error", &FakeTxManager{}, func(context.Context) error { return fnErr }, fnErr, true, 1},
		{"returns Err after successful fn", &FakeTxManager{Err: boom}, func(context.Context) error { return nil }, boom, true, 1},
		{"FailBegin returns Err without running fn", &FakeTxManager{Err: boom, FailBegin: true}, func(context.Context) error { return nil }, boom, false, 1},
		{"FailBegin without Err returns generic error", &FakeTxManager{FailBegin: true}, func(context.Context) error { return nil }, errFakeBegin, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ran := false
			err := tt.mgr.InTx(context.Background(), func(context.Context) error {
				ran = true
				return tt.fn(context.Background())
			})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if ran != tt.wantRun {
				t.Errorf("fn ran = %v, want %v", ran, tt.wantRun)
			}
			if tt.mgr.Calls != tt.wantCalls {
				t.Errorf("Calls = %d, want %d", tt.mgr.Calls, tt.wantCalls)
			}
		})
	}

	t.Run("Calls accumulates across invocations", func(t *testing.T) {
		m := &FakeTxManager{}
		for i := 0; i < 3; i++ {
			_ = m.InTx(context.Background(), func(context.Context) error { return nil })
		}
		if m.Calls != 3 {
			t.Errorf("Calls = %d, want 3", m.Calls)
		}
	})
}

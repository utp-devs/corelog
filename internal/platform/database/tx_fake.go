package database

import (
	"context"
	"errors"
)

// errFakeBegin is returned by FakeTxManager when FailBegin is set without Err.
var errFakeBegin = errors.New("fake tx: begin failed")

// FakeTxManager is a TxManager for use-case tests. It runs fn directly with
// the given context and does not touch any database.
//
// Calls counts every InTx invocation. When FailBegin is true, InTx returns Err
// (or a generic error when Err is nil) without running fn. Otherwise fn runs
// first and, if fn returned nil, Err is returned to simulate a commit failure.
type FakeTxManager struct {
	Calls     int
	Err       error
	FailBegin bool
}

// InTx implements TxManager.
func (f *FakeTxManager) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	f.Calls++
	if f.FailBegin {
		if f.Err != nil {
			return f.Err
		}
		return errFakeBegin
	}
	if err := fn(ctx); err != nil {
		return err
	}
	return f.Err
}

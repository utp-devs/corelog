import { HttpErrorResponse } from '@angular/common/http';
import { Subject, of, throwError } from 'rxjs';

import { AsyncState } from './async-state';

describe('AsyncState', () => {
  it('stores the value and clears loading when the source emits', () => {
    const state = new AsyncState<string>();
    state.load(of('listo'));
    expect(state.value()).toBe('listo');
    expect(state.loading()).toBe(false);
    expect(state.hasValue()).toBe(true);
  });

  it('exposes a normalized ApiError and keeps no value on failure', () => {
    const state = new AsyncState<string>();
    state.load(throwError(() => new HttpErrorResponse({ status: 404, error: null })));
    expect(state.error()?.status).toBe(404);
    expect(state.error()?.code).toBe('http.404');
    expect(state.value()).toBeUndefined();
    expect(state.loading()).toBe(false);
  });

  it('cancel stops the request in flight and keeps the previous value', () => {
    const state = new AsyncState<number>();
    state.set(1);
    const pending = new Subject<number>();
    state.load(pending);
    expect(state.loading()).toBe(true);
    state.cancel();
    expect(state.loading()).toBe(false);
    expect(state.value()).toBe(1);
    pending.next(2);
    expect(state.value()).toBe(1);
  });

  it('reset clears value and error', () => {
    const state = new AsyncState<number>();
    state.load(throwError(() => new Error('boom')));
    state.set(5);
    state.reset();
    expect(state.value()).toBeUndefined();
    expect(state.error()).toBeNull();
    expect(state.hasValue()).toBe(false);
  });
});

import { computed, signal } from '@angular/core';
import { Observable, Subscription, catchError, defer, finalize, tap, throwError } from 'rxjs';

import { ApiError } from '../models';
import { asApiError } from '../utils/api-error';

/**
 * Signals for one asynchronous read: the last value, the loading flag and the last error.
 * `load` cancels the previous request, so only the latest response is kept.
 */
export class AsyncState<T> {
  private readonly valueState = signal<T | undefined>(undefined);
  private readonly loadingState = signal(false);
  private readonly errorState = signal<ApiError | null>(null);
  private subscription: Subscription | null = null;

  readonly value = this.valueState.asReadonly();
  readonly loading = this.loadingState.asReadonly();
  readonly error = this.errorState.asReadonly();
  readonly hasValue = computed(() => this.valueState() !== undefined);

  /** Wraps a source so the signals follow its lifecycle. Errors are re-thrown normalized. */
  track(source: Observable<T>): Observable<T> {
    return defer(() => {
      this.loadingState.set(true);
      this.errorState.set(null);
      return source.pipe(
        tap((value) => this.valueState.set(value)),
        catchError((error: unknown) => {
          const apiError = asApiError(error);
          this.errorState.set(apiError);
          return throwError(() => apiError);
        }),
        finalize(() => this.loadingState.set(false)),
      );
    });
  }

  /** Subscribes to the source, cancelling any request still in flight. Errors go to `error`. */
  load(source: Observable<T>): void {
    this.cancel();
    this.subscription = this.track(source).subscribe({ error: () => undefined });
  }

  set(value: T | undefined): void {
    this.valueState.set(value);
  }

  update(updater: (current: T | undefined) => T | undefined): void {
    this.valueState.update(updater);
  }

  /** Stops the request in flight and clears the loading flag. The value is kept. */
  cancel(): void {
    this.subscription?.unsubscribe();
    this.subscription = null;
    this.loadingState.set(false);
  }

  /** Cancels the request and clears value and error. */
  reset(): void {
    this.cancel();
    this.valueState.set(undefined);
    this.errorState.set(null);
  }
}

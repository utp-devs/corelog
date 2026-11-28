import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, firstValueFrom } from 'rxjs';

import type {
  TwoFactorDisablePayload,
  TwoFactorEnrollment,
  TwoFactorStatus,
} from '../models/two-factor.model';
import { TwoFactorApiService } from '../services/api/two-factor-api.service';
import { asApiError } from '../utils/api-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/**
 * Two-step verification of the signed-in account: status, the pending enrollment and the recovery codes.
 * Recovery codes live only in memory, until the dialog that shows them clears them: they are never
 * written to storage. Mutations reject with the normalized ApiError so the dialog can render it.
 */
@Injectable({ providedIn: 'root' })
export class TwoFactorStore {
  private readonly api = inject(TwoFactorApiService);
  private readonly enrollmentState = signal<TwoFactorEnrollment | null>(null);
  private readonly codesState = signal<readonly string[] | null>(null);
  private readonly busyState = signal(false);

  readonly status = new AsyncState<TwoFactorStatus>();
  readonly enrollment = this.enrollmentState.asReadonly();
  /** Recovery codes waiting to be shown (once). Null when there is nothing to show. */
  readonly recoveryCodes = this.codesState.asReadonly();
  readonly busy = this.busyState.asReadonly();
  readonly enabled = computed(() => this.status.value()?.enabled ?? false);

  constructor() {
    resetOnLogout(() => this.reset());
  }

  load(): void {
    this.status.load(this.api.status());
  }

  /** Starts the enrollment: the server creates a secret that stays pending until a code confirms it. */
  async beginEnrollment(): Promise<TwoFactorEnrollment> {
    const enrollment = await this.run(this.api.enroll());
    this.enrollmentState.set(enrollment);
    return enrollment;
  }

  /** Confirms the enrollment with a code from the app. Leaves the new recovery codes ready to show. */
  async confirmEnrollment(code: string): Promise<void> {
    const codes = await this.run(this.api.confirm(code));
    this.enrollmentState.set(null);
    this.codesState.set(codes);
    this.status.load(this.api.status());
  }

  /** Replaces the recovery codes with a new set. */
  async regenerateRecoveryCodes(code: string): Promise<void> {
    this.codesState.set(await this.run(this.api.regenerateRecoveryCodes(code)));
    this.status.load(this.api.status());
  }

  async disable(payload: TwoFactorDisablePayload): Promise<void> {
    await this.run(this.api.disable(payload));
    this.codesState.set(null);
    this.status.load(this.api.status());
  }

  /** Forgets the pending enrollment (the user closed the dialog before confirming). */
  cancelEnrollment(): void {
    this.enrollmentState.set(null);
  }

  /** Drops the recovery codes from memory. */
  clearRecoveryCodes(): void {
    this.codesState.set(null);
  }

  reset(): void {
    this.status.reset();
    this.enrollmentState.set(null);
    this.codesState.set(null);
    this.busyState.set(false);
  }

  private async run<T>(source: Observable<T>): Promise<T> {
    this.busyState.set(true);
    try {
      return await firstValueFrom(source);
    } catch (error) {
      throw asApiError(error);
    } finally {
      this.busyState.set(false);
    }
  }
}

import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import type {
  ConsentRef,
  ConsentSource,
  ConsentStatus,
  PendingConsent,
} from '../models/legal.model';
import { AuthApiService } from '../services/api/auth-api.service';
import { LegalApiService } from '../services/api/legal-api.service';
import { SessionService } from '../services/session.service';
import { asApiError } from '../utils/api-error';
import { AuthStore } from './auth.store';
import { resetOnLogout } from './session-scoped';

/**
 * Consents of the signed-in account: the documents still pending (taken from the session user), the
 * accepted ones, and the actions that change them. Mutations reject with the normalized ApiError.
 */
@Injectable({ providedIn: 'root' })
export class ConsentStore {
  private readonly api = inject(LegalApiService);
  private readonly auth = inject(AuthStore);
  private readonly authApi = inject(AuthApiService);
  private readonly session = inject(SessionService);

  readonly status = signal<ConsentStatus | null>(null);
  readonly loading = signal(false);
  readonly error = signal<string | null>(null);

  /** Required documents the account must accept before it continues. */
  readonly pending = computed<readonly PendingConsent[]>(
    () => this.auth.user()?.pendingConsents ?? [],
  );
  readonly hasPending = computed(() => this.pending().length > 0);

  constructor() {
    resetOnLogout(() => {
      this.status.set(null);
      this.error.set(null);
    });
  }

  async load(): Promise<void> {
    this.loading.set(true);
    this.error.set(null);
    try {
      this.status.set(await firstValueFrom(this.api.myConsents()));
    } catch (error) {
      this.error.set(asApiError(error).message);
    } finally {
      this.loading.set(false);
    }
  }

  /** Records the acceptance of the given versions and refreshes the session user. */
  async accept(refs: readonly ConsentRef[], source: ConsentSource): Promise<void> {
    this.loading.set(true);
    this.error.set(null);
    try {
      this.status.set(await firstValueFrom(this.api.recordConsents(refs, source)));
      await this.refreshUser();
    } catch (error) {
      this.error.set(asApiError(error).message);
      throw asApiError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async revoke(slug: string): Promise<void> {
    this.loading.set(true);
    this.error.set(null);
    try {
      await firstValueFrom(this.api.revokeConsent(slug));
      await this.load();
    } catch (error) {
      this.error.set(asApiError(error).message);
      throw asApiError(error);
    } finally {
      this.loading.set(false);
    }
  }

  private async refreshUser(): Promise<void> {
    const token = this.session.token();
    if (!token) return;
    this.auth.setUser(await firstValueFrom(this.authApi.me(token)));
  }
}

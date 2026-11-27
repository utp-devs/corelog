import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import type { AuthSession, LoginPayload, RegisterPayload } from '../models/auth.model';
import type { ChangePasswordPayload, UpdateProfilePayload, User } from '../models/user.model';
import { AccountSecurityApiService } from '../services/api/account-security-api.service';
import { AuthApiService } from '../services/api/auth-api.service';
import { SessionService } from '../services/session.service';
import { ApiError } from '../models/api-error.model';
import { asApiError } from '../utils/api-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

export type AuthStatus = 'idle' | 'loading' | 'authenticated' | 'error';

/**
 * Signed-in account and the session lifecycle. The token itself lives in SessionService; this store
 * keeps the account, the status of the last sign-in attempt and whether the stored session was checked.
 * Mutations reject with the normalized ApiError so the caller can render field or form errors.
 */
@Injectable({ providedIn: 'root' })
export class AuthStore {
  private readonly api = inject(AuthApiService);
  private readonly accountSecurity = inject(AccountSecurityApiService);
  private readonly session = inject(SessionService);

  private readonly account = new AsyncState<User>();

  readonly status = signal<AuthStatus>('idle');
  readonly error = signal<string | null>(null);
  /** False until the stored session has been checked (the guards wait for it). */
  readonly sessionChecked = signal(false);

  readonly user = computed<User | null>(() => this.account.value() ?? null);
  readonly isAuthenticated = computed(() => this.user() !== null);
  readonly isPlatformAdmin = computed(() => this.user()?.role === 'admin');
  readonly displayName = computed<string | null>(() => {
    const user = this.user();
    return user ? user.displayName || user.name : null;
  });

  constructor() {
    resetOnLogout(() => this.reset());
  }

  async login(payload: LoginPayload): Promise<void> {
    this.status.set('loading');
    this.error.set(null);
    try {
      this.open(await firstValueFrom(this.api.login(payload)));
    } catch (error) {
      throw this.fail(error);
    }
  }

  /** Creates the account and signs in with it (the API does both steps). */
  async register(payload: RegisterPayload): Promise<void> {
    this.status.set('loading');
    this.error.set(null);
    try {
      this.open(await firstValueFrom(this.api.register(payload)));
    } catch (error) {
      throw this.fail(error);
    }
  }

  /** Checks the stored token against GET /users/me. An invalid token is discarded silently. */
  async restoreSession(): Promise<void> {
    const token = this.session.token();
    if (!token) {
      this.status.set('idle');
      this.sessionChecked.set(true);
      return;
    }
    this.status.set('loading');
    try {
      await firstValueFrom(this.account.track(this.api.me(token)));
      this.status.set('authenticated');
    } catch {
      this.session.clear();
      this.account.reset();
      this.status.set('idle');
    }
    this.sessionChecked.set(true);
  }

  /** Replaces the signed-in account in place, so the shell reflects profile or avatar edits without a reload. */
  setUser(user: User): void {
    this.account.set(user);
  }

  async updateProfile(payload: UpdateProfilePayload): Promise<void> {
    this.account.set(await firstValueFrom(this.api.updateProfile(payload)));
  }

  async changePassword(payload: ChangePasswordPayload): Promise<void> {
    await firstValueFrom(this.api.changePassword(payload));
  }

  /**
   * Ends the session. The server closes it on a best-effort basis, even when the browser has already
   * lost the token. Stores that depend on the session reset through resetOnLogout.
   */
  logout(): void {
    const token = this.session.token();
    if (token) {
      this.accountSecurity.logout(token).subscribe({ error: () => undefined });
    }
    this.session.clear();
    this.reset();
    this.sessionChecked.set(true);
  }

  /** Clears the account and the status. Runs on logout and whenever the session ends. */
  reset(): void {
    this.account.reset();
    this.status.set('idle');
    this.error.set(null);
  }

  private open(session: AuthSession): void {
    this.session.startSession(session.token);
    this.account.set(session.user);
    this.status.set('authenticated');
    this.sessionChecked.set(true);
  }

  private fail(error: unknown): ApiError {
    const apiError = asApiError(error);
    this.status.set('error');
    this.error.set(apiError.message);
    this.sessionChecked.set(true);
    return apiError;
  }
}

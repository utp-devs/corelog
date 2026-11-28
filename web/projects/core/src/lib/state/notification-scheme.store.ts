import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import type { ApiError } from '../models/api-error.model';
import type {
  NotificationRuleInput,
  NotificationScheme,
  NotificationSchemeScope,
} from '../models/notification-scheme.model';
import { NotificationSchemeApiService } from '../services/api/notification-scheme-api.service';
import { asApiError } from '../utils/api-error';

export type NotificationSchemeStatus = 'idle' | 'loading' | 'ready' | 'error';

/**
 * Notification scheme of one scope (a project or the organization defaults). Provide it in the
 * component that edits the scheme, so each editor owns its own copy.
 */
@Injectable()
export class NotificationSchemeStore {
  private readonly api = inject(NotificationSchemeApiService);
  private scope: NotificationSchemeScope | null = null;
  private generation = 0;

  private readonly schemeState = signal<NotificationScheme | null>(null);
  private readonly statusState = signal<NotificationSchemeStatus>('idle');
  private readonly errorState = signal<ApiError | null>(null);
  private readonly savingState = signal(false);

  readonly scheme = this.schemeState.asReadonly();
  readonly status = this.statusState.asReadonly();
  /** Last failure: a load error while `status` is `error`, or the failure of a save or reset. */
  readonly error = this.errorState.asReadonly();
  readonly saving = this.savingState.asReadonly();
  readonly forbidden = computed(() => this.errorState()?.status === 403);

  /** Reads the scheme of a scope. A response of an older scope is ignored. */
  async load(scope: NotificationSchemeScope): Promise<void> {
    this.scope = scope;
    const generation = ++this.generation;
    this.statusState.set('loading');
    this.errorState.set(null);
    try {
      const scheme = await firstValueFrom(this.api.get(scope));
      if (generation !== this.generation) return;
      this.schemeState.set(scheme);
      this.statusState.set('ready');
    } catch (failure) {
      if (generation !== this.generation) return;
      this.errorState.set(asApiError(failure));
      this.statusState.set('error');
    }
  }

  /** Saves the own rules. Resolves true on success; on failure `error` carries the problem. */
  async save(rules: readonly NotificationRuleInput[]): Promise<boolean> {
    return this.mutate((scope) => this.api.save(scope, rules));
  }

  /** Deletes the own rules so the scope inherits again. */
  async reset(): Promise<boolean> {
    return this.mutate((scope) => this.api.reset(scope));
  }

  clearError(): void {
    this.errorState.set(null);
  }

  private async mutate(
    call: (scope: NotificationSchemeScope) => ReturnType<NotificationSchemeApiService['get']>,
  ): Promise<boolean> {
    const scope = this.scope;
    if (!scope) return false;
    this.savingState.set(true);
    this.errorState.set(null);
    try {
      this.schemeState.set(await firstValueFrom(call(scope)));
      return true;
    } catch (failure) {
      this.errorState.set(asApiError(failure));
      return false;
    } finally {
      this.savingState.set(false);
    }
  }
}

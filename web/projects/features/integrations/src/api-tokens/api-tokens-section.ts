import { DatePipe } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  signal,
} from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import {
  ApiTokenApiService,
  NotificationService,
  asApiError,
  type ApiToken,
  type ApiTokenScope,
} from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { Dialog } from '@corelog/ui/dialog';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { SecretReveal } from '@corelog/ui/secret-reveal';
import { Section, SectionActions } from '@corelog/ui/section';
import { ToneTag, type Tone } from '@corelog/ui/tone-tag';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { firstValueFrom } from 'rxjs';

import {
  SCOPE_LABELS,
  TOKEN_STATE_LABELS,
  expiryFromDate,
  tokenState,
  tomorrow,
  type TokenState,
} from './token-state';

type LoadStatus = 'loading' | 'ready' | 'error';

const STATE_TONES: Readonly<Record<TokenState, Tone>> = {
  active: 'green',
  expired: 'orange',
  revoked: 'neutral',
};

/**
 * Personal access tokens of the account: list with state, creation with a one-time reveal of the plain value,
 * and revocation. The plain token lives only in the `revealed` signal while the reveal panel is open; closing
 * the panel clears it.
 */
@Component({
  selector: 'app-api-tokens-section',
  imports: [
    Button,
    ConfirmModal,
    DatePipe,
    Dialog,
    EmptyState,
    ErrorState,
    InputText,
    LoadingSkeleton,
    ReactiveFormsModule,
    SecretReveal,
    Section,
    SectionActions,
    ToneTag,
  ],
  templateUrl: './api-tokens-section.html',
  styleUrl: './api-tokens-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ApiTokensSection {
  private readonly api = inject(ApiTokenApiService);
  private readonly notifications = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly status = signal<LoadStatus>('loading');
  protected readonly tokens = signal<readonly ApiToken[]>([]);
  protected readonly activeCount = computed(
    () => this.tokens().filter((t) => tokenState(t) === 'active').length,
  );

  protected readonly formOpen = signal(false);
  protected readonly saving = signal(false);
  protected readonly formError = signal<string | null>(null);
  /** Plain value of the token just created; cleared when the reveal panel is dismissed. */
  protected readonly revealed = signal<string | null>(null);
  protected readonly revoking = signal<ApiToken | null>(null);
  protected readonly revokeBusy = signal(false);
  protected readonly minDate = tomorrow();

  protected readonly form = new FormGroup({
    name: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.maxLength(80)],
    }),
    scope: new FormControl<ApiTokenScope>('read', { nonNullable: true }),
    expires: new FormControl('', { nonNullable: true }),
  });

  protected readonly scopeLabels = SCOPE_LABELS;
  protected readonly stateLabels = TOKEN_STATE_LABELS;

  constructor() {
    this.load();
    // Never keep the plain token beyond the life of the page.
    this.destroyRef.onDestroy(() => this.revealed.set(null));
  }

  protected stateOf(token: ApiToken): TokenState {
    return tokenState(token);
  }

  protected toneOf(token: ApiToken): Tone {
    return STATE_TONES[tokenState(token)];
  }

  protected load(): void {
    this.status.set('loading');
    this.api.list().subscribe({
      next: (tokens) => {
        this.tokens.set(tokens);
        this.status.set('ready');
      },
      error: () => this.status.set('error'),
    });
  }

  protected openForm(): void {
    this.form.reset({ name: '', scope: 'read', expires: '' });
    this.formError.set(null);
    this.formOpen.set(true);
  }

  protected closeForm(): void {
    // While the secret is on screen the dialog only closes through the confirmation.
    if (this.revealed()) return;
    this.formOpen.set(false);
  }

  protected async create(): Promise<void> {
    if (this.form.invalid || this.saving()) return;
    const value = this.form.getRawValue();
    this.saving.set(true);
    this.formError.set(null);
    try {
      const created = await firstValueFrom(
        this.api.create({
          name: value.name.trim(),
          scopes: value.scope === 'write' ? ['read', 'write'] : ['read'],
          expiresAt: expiryFromDate(value.expires),
        }),
      );
      this.tokens.update((list) => [created.token, ...list]);
      this.revealed.set(created.secret);
    } catch (error) {
      this.formError.set(asApiError(error).message);
    } finally {
      this.saving.set(false);
    }
  }

  protected onRevealDismissed(): void {
    this.revealed.set(null);
    this.formOpen.set(false);
  }

  protected async revoke(): Promise<void> {
    const token = this.revoking();
    if (!token || this.revokeBusy()) return;
    this.revokeBusy.set(true);
    try {
      await firstValueFrom(this.api.revoke(token.id));
      this.tokens.update((list) =>
        list.map((t) => (t.id === token.id ? { ...t, revokedAt: new Date().toISOString() } : t)),
      );
      this.notifications.success(`El token «${token.name}» quedó revocado.`);
      this.revoking.set(null);
    } catch (error) {
      this.notifications.error(error);
    } finally {
      this.revokeBusy.set(false);
    }
  }

  protected scopeText(token: ApiToken): string {
    return token.scopes.includes('admin')
      ? SCOPE_LABELS.admin
      : token.scopes.includes('write')
        ? SCOPE_LABELS.write
        : SCOPE_LABELS.read;
  }
}

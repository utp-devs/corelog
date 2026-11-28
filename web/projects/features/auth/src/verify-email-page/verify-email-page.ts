import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { ActivatedRoute, RouterLink } from '@angular/router';
import {
  AuthStore,
  PasswordResetApiService,
  asApiError,
  isRateLimited,
  rateLimitMessage,
} from '@corelog/core';
import { Button } from 'primeng/button';
import { ProgressSpinner } from 'primeng/progressspinner';
import { firstValueFrom } from 'rxjs';

import { AuthAlert } from '../shared/auth-alert';
import { AuthLayout } from '../shared/auth-layout';

export type VerifyState = 'loading' | 'success' | 'error';

/** Confirms the email address from the emailed link as soon as the page opens. */
@Component({
  selector: 'app-verify-email-page',
  imports: [RouterLink, AuthLayout, AuthAlert, Button, ProgressSpinner],
  templateUrl: './verify-email-page.html',
  styleUrl: './verify-email-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class VerifyEmailPage {
  private readonly api = inject(PasswordResetApiService);
  private readonly token = inject(ActivatedRoute).snapshot.queryParamMap.get('token') ?? '';

  protected readonly auth = inject(AuthStore);
  protected readonly state = signal<VerifyState>(this.token ? 'loading' : 'error');
  protected readonly resendMessage = signal<string | null>(null);
  protected readonly resending = signal(false);

  constructor() {
    if (this.token) void this.verify();
  }

  private async verify(): Promise<void> {
    try {
      await firstValueFrom(this.api.verify(this.token));
      this.state.set('success');
    } catch {
      this.state.set('error');
    }
  }

  /** A signed-in person can ask for a new email from here. */
  protected async resend(): Promise<void> {
    if (this.resending()) return;
    this.resending.set(true);
    this.resendMessage.set(null);
    try {
      await firstValueFrom(this.api.requestVerification());
      this.resendMessage.set('Te enviamos un correo nuevo. Revisa tu bandeja de entrada.');
    } catch (error) {
      const api = asApiError(error);
      if (api.status === 409) {
        this.resendMessage.set('Tu correo ya está verificado.');
      } else if (isRateLimited(api)) {
        this.resendMessage.set(rateLimitMessage(api));
      } else {
        this.resendMessage.set('No se pudo enviar el correo. Inténtalo de nuevo en unos minutos.');
      }
    } finally {
      this.resending.set(false);
    }
  }
}

import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { AuthStore, PasswordResetApiService, asApiError } from '@corelog/core';
import { firstValueFrom } from 'rxjs';

const DISMISSED_KEY = 'corelog.verify-email-banner.dismissed';

type Feedback = { readonly kind: 'success' | 'error'; readonly text: string };

function readDismissed(): boolean {
  try {
    return sessionStorage.getItem(DISMISSED_KEY) === '1';
  } catch {
    return false;
  }
}

/**
 * Reminder shown to a signed-in account whose email address is not verified yet. It offers to send the
 * verification email again and can be dismissed for the rest of the browser session.
 */
@Component({
  selector: 'app-verify-email-banner',
  templateUrl: './verify-email-banner.html',
  styleUrl: './verify-email-banner.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class VerifyEmailBanner {
  private readonly auth = inject(AuthStore);
  private readonly api = inject(PasswordResetApiService);

  /** True once the banner was dismissed in this browser session. */
  private readonly dismissed = signal(readDismissed());
  /** Set once the API says the address is verified, before the profile is read again. */
  private readonly verifiedNow = signal(false);

  protected readonly sending = signal(false);
  protected readonly feedback = signal<Feedback | null>(null);

  protected readonly visible = computed(() => {
    const user = this.auth.user();
    return user !== null && !user.emailVerifiedAt && !this.verifiedNow() && !this.dismissed();
  });
  protected readonly email = computed(() => this.auth.user()?.email ?? '');

  protected async resend(): Promise<void> {
    if (this.sending()) return;
    this.sending.set(true);
    this.feedback.set(null);
    try {
      await firstValueFrom(this.api.requestVerification());
      this.feedback.set({
        kind: 'success',
        text: 'Te enviamos un correo con el enlace. Revisa tu bandeja de entrada.',
      });
    } catch (failure) {
      const error = asApiError(failure);
      if (error.status === 429) {
        this.feedback.set({
          kind: 'error',
          text: 'Pediste demasiados correos seguidos. Espera un momento antes de volver a intentarlo.',
        });
      } else if (error.code === 'auth.email_already_verified') {
        this.verifiedNow.set(true);
      } else if (error.code === 'auth.verification_unavailable') {
        this.feedback.set({
          kind: 'error',
          text: 'El envío de correos no está disponible por ahora. Inténtalo más tarde.',
        });
      } else {
        this.feedback.set({
          kind: 'error',
          text: 'No pudimos enviar el correo. Inténtalo de nuevo.',
        });
      }
    } finally {
      this.sending.set(false);
    }
  }

  protected dismiss(): void {
    this.dismissed.set(true);
    try {
      sessionStorage.setItem(DISMISSED_KEY, '1');
    } catch {
      // The banner stays hidden until the page reloads.
    }
  }
}

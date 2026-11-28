import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { PhoneApiService, asApiError, type User } from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { Button } from 'primeng/button';
import { firstValueFrom } from 'rxjs';

/** Seconds before another code can be asked for. */
const RESEND_SECONDS = 30;
const CODE_LENGTH = 6;

type Step = 'sending' | 'code' | 'done' | 'failed';

const ERROR_COPY: Readonly<Record<string, string>> = {
  'user.phone_send_limit': 'Pediste demasiados códigos. Espera un rato antes de pedir otro.',
  'user.phone_code_attempts': 'Agotaste los intentos de este código. Pide uno nuevo.',
  'user.phone_code_invalid': 'El código no es correcto. Revísalo e inténtalo de nuevo.',
  'user.phone_code_expired': 'El código venció. Pide uno nuevo.',
  'user.phone_taken': 'Este teléfono ya está verificado en otra cuenta.',
  'user.phone_already_verified': 'Tu teléfono ya está verificado.',
  'user.phone_not_set': 'Guarda primero un teléfono en tu perfil.',
};

/**
 * Dialog that verifies the phone of the account: it asks for a code when it opens, shows the masked
 * destination, takes the six digit code and offers to resend it after a short wait. The copy is generic
 * about the channel because the delivery depends on how the installation is configured.
 */
@Component({
  selector: 'app-phone-verify-dialog',
  imports: [Button, Dialog],
  templateUrl: './phone-verify-dialog.html',
  styleUrl: './phone-verify-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PhoneVerifyDialog {
  private readonly api = inject(PhoneApiService);
  private timer: ReturnType<typeof setInterval> | null = null;

  readonly open = input(false);
  readonly closed = output<void>();
  /** Emits the account once its phone is verified. */
  readonly verified = output<User>();

  protected readonly step = signal<Step>('sending');
  protected readonly destination = signal('');
  protected readonly code = signal('');
  protected readonly working = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly wait = signal(0);

  protected readonly canVerify = computed(
    () => this.code().length === CODE_LENGTH && !this.working(),
  );
  protected readonly canResend = computed(() => this.wait() === 0 && !this.working());
  protected readonly heading = computed(() =>
    this.step() === 'done' ? 'Teléfono verificado' : 'Verificar teléfono',
  );

  constructor() {
    effect(() => {
      const isOpen = this.open();
      untracked(() => (isOpen ? void this.request() : this.reset()));
    });
    inject(DestroyRef).onDestroy(() => this.stopTimer());
  }

  protected onCode(event: Event): void {
    this.code.set(
      (event.target as HTMLInputElement).value.replace(/\D/g, '').slice(0, CODE_LENGTH),
    );
  }

  protected async request(): Promise<void> {
    this.working.set(true);
    this.error.set(null);
    this.code.set('');
    try {
      const sent = await firstValueFrom(this.api.requestVerification());
      this.destination.set(sent.destination);
      this.step.set('code');
      this.startWait();
    } catch (error) {
      const message = this.copyOf(error);
      this.error.set(message);
      this.step.set(this.step() === 'code' ? 'code' : 'failed');
    } finally {
      this.working.set(false);
    }
  }

  protected async verify(): Promise<void> {
    if (!this.canVerify()) return;
    this.working.set(true);
    this.error.set(null);
    try {
      const user = await firstValueFrom(this.api.verify(this.code()));
      this.step.set('done');
      this.stopTimer();
      this.verified.emit(user);
    } catch (error) {
      this.error.set(this.copyOf(error));
    } finally {
      this.working.set(false);
    }
  }

  private copyOf(error: unknown): string {
    const api = asApiError(error);
    return ERROR_COPY[api.code] ?? 'No se pudo completar la operación. Inténtalo de nuevo.';
  }

  private startWait(): void {
    this.stopTimer();
    this.wait.set(RESEND_SECONDS);
    this.timer = setInterval(() => {
      this.wait.update((seconds) => Math.max(0, seconds - 1));
      if (this.wait() === 0) this.stopTimer();
    }, 1000);
  }

  private stopTimer(): void {
    if (this.timer !== null) clearInterval(this.timer);
    this.timer = null;
  }

  private reset(): void {
    this.stopTimer();
    this.step.set('sending');
    this.code.set('');
    this.error.set(null);
    this.wait.set(0);
    this.destination.set('');
  }
}

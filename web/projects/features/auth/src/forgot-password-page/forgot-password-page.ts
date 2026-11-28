import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  signal,
} from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink } from '@angular/router';
import {
  PasswordResetApiService,
  asApiError,
  isRateLimited,
  rateLimitMessage,
} from '@corelog/core';
import { Button } from 'primeng/button';

import { focusFirstInvalid } from '../auth-navigation';
import { AuthAlert } from '../shared/auth-alert';
import { AuthControl, AuthField } from '../shared/auth-field';
import { AuthLayout } from '../shared/auth-layout';

const EMAIL_MESSAGE = 'Introduce un correo electrónico válido';
/** Seconds before the same screen allows another request. */
export const RESEND_SECONDS = 60;

/**
 * First step of a password reset. The answer is the same whether or not the address has an account, so the
 * screen never reveals which addresses are registered. A countdown limits repeated requests.
 */
@Component({
  selector: 'app-forgot-password-page',
  imports: [ReactiveFormsModule, RouterLink, AuthLayout, AuthAlert, AuthField, AuthControl, Button],
  templateUrl: './forgot-password-page.html',
  styleUrl: './forgot-password-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ForgotPasswordPage {
  private readonly api = inject(PasswordResetApiService);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  protected readonly emailMessages = { required: EMAIL_MESSAGE, email: EMAIL_MESSAGE };
  protected readonly form = new FormGroup({
    email: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.email],
    }),
  });

  protected readonly submitted = signal(false);
  protected readonly submitting = signal(false);
  protected readonly sent = signal(false);
  protected readonly formError = signal<string | null>(null);
  protected readonly remaining = signal(0);
  protected readonly canResend = computed(() => this.remaining() === 0 && !this.submitting());

  private timer: ReturnType<typeof setInterval> | null = null;

  constructor() {
    inject(DestroyRef).onDestroy(() => this.stopTimer());
  }

  protected async onSubmit(): Promise<void> {
    if (this.submitting() || this.remaining() > 0) return;
    this.submitted.set(true);
    this.formError.set(null);
    if (this.form.invalid) {
      afterNextRender(() => focusFirstInvalid(this.host.nativeElement), {
        injector: this.injector,
      });
      return;
    }
    this.submitting.set(true);
    try {
      await new Promise<void>((resolve, reject) =>
        this.api.forgot(this.form.controls.email.value.trim()).subscribe({
          next: () => resolve(),
          error: (error: unknown) => reject(error),
        }),
      );
      this.sent.set(true);
      this.startCountdown(RESEND_SECONDS);
    } catch (error) {
      const api = asApiError(error);
      if (isRateLimited(api)) {
        this.formError.set(rateLimitMessage(api));
        this.startCountdown(api.retryAfterSeconds ?? RESEND_SECONDS);
      } else if (api.status === 0) {
        this.formError.set(api.message);
      } else {
        // Any other answer reads the same as success: the screen must not tell addresses apart.
        this.sent.set(true);
        this.startCountdown(RESEND_SECONDS);
      }
    } finally {
      this.submitting.set(false);
    }
  }

  /** Back to the form to try another address. */
  protected useAnotherEmail(): void {
    this.sent.set(false);
    this.submitted.set(false);
    this.form.reset();
  }

  private startCountdown(seconds: number): void {
    this.stopTimer();
    this.remaining.set(Math.max(1, Math.ceil(seconds)));
    this.timer = setInterval(() => {
      this.remaining.update((value) => Math.max(0, value - 1));
      if (this.remaining() === 0) this.stopTimer();
    }, 1000);
  }

  private stopTimer(): void {
    if (this.timer !== null) clearInterval(this.timer);
    this.timer = null;
  }
}

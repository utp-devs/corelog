import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  inject,
  signal,
} from '@angular/core';
import {
  AbstractControl,
  FormControl,
  FormGroup,
  ReactiveFormsModule,
  Validators,
} from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import {
  AuthStore,
  HOME_URL,
  LOCATION_ASSIGN,
  applyServerErrors,
  asApiError,
  safeRedirect,
} from '@corelog/core';
import { PasswordInput } from '@corelog/ui/password-input';
import { Button } from 'primeng/button';

import { authErrorMessage } from '../auth-messages';
import { focusFirstInvalid, goAfterAuth } from '../auth-navigation';
import { AuthAlert } from '../shared/auth-alert';
import { AuthControl, AuthField } from '../shared/auth-field';
import { AuthLayout } from '../shared/auth-layout';

const EMAIL_MESSAGE = 'Introduce un correo electrónico válido';
const PASSWORD_MESSAGE = 'La contraseña es obligatoria';

/** Sign-in screen. Server field details are shown under their fields; other failures in a banner. */
@Component({
  selector: 'app-login-page',
  imports: [
    ReactiveFormsModule,
    AuthLayout,
    AuthAlert,
    AuthField,
    AuthControl,
    PasswordInput,
    Button,
    RouterLink,
  ],
  templateUrl: './login-page.html',
  styleUrl: './login-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LoginPage {
  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly home = inject(HOME_URL);
  private readonly assign = inject(LOCATION_ASSIGN);

  protected readonly returnUrl = inject(ActivatedRoute).snapshot.queryParamMap.get('returnUrl');
  /** Shown when the session was closed from another device. */
  protected readonly sessionClosedNotice =
    inject(ActivatedRoute).snapshot.queryParamMap.get('reason') === 'session_revoked'
      ? 'Tu sesión fue cerrada desde otro dispositivo'
      : null;
  protected readonly registerQuery: Readonly<Record<string, string>> = this.returnUrl
    ? { returnUrl: this.returnUrl }
    : {};

  protected readonly emailMessages = { required: EMAIL_MESSAGE, email: EMAIL_MESSAGE };
  protected readonly passwordMessages = { required: PASSWORD_MESSAGE };

  protected readonly form = new FormGroup({
    email: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.email],
    }),
    password: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
  });

  protected readonly challenge = this.auth.twoFactorChallenge;
  /** The second step asks for a recovery code instead of the six digits of the app. */
  protected readonly useRecovery = signal(false);
  protected readonly codeMessages = { required: 'Introduce el código' };
  protected readonly codeForm = new FormGroup({
    code: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
  });

  protected readonly submitted = signal(false);
  protected readonly submitting = signal(false);
  protected readonly formError = signal<string | null>(null);

  /** Detail the server attached to the field, if any. */
  protected serverError(control: AbstractControl): string | undefined {
    return control.getError('server') ?? undefined;
  }

  protected async onSubmit(): Promise<void> {
    if (this.submitting()) return;
    this.submitted.set(true);
    this.formError.set(null);
    // Drops server errors from the previous attempt so they do not block the next submit.
    Object.values(this.form.controls).forEach((control) => control.updateValueAndValidity());
    if (this.form.invalid) {
      afterNextRender(() => focusFirstInvalid(this.host.nativeElement), {
        injector: this.injector,
      });
      return;
    }

    this.submitting.set(true);
    try {
      await this.auth.login(this.form.getRawValue());
      if (this.challenge()) {
        // The account asks for a second step: show it with a fresh form.
        this.submitted.set(false);
        this.codeForm.reset();
        this.focusCode();
        return;
      }
      goAfterAuth(safeRedirect(this.returnUrl, this.home), this.router, this.assign);
    } catch (error) {
      if (!applyServerErrors(this.form, asApiError(error))) {
        this.formError.set(authErrorMessage(error, 'login'));
      }
    } finally {
      this.submitting.set(false);
    }
  }

  /** Second step: sends the code (or the recovery code) with the challenge of the first step. */
  protected async onVerify(): Promise<void> {
    if (this.submitting()) return;
    this.submitted.set(true);
    this.formError.set(null);
    const value = this.codeForm.controls.code.value.trim();
    if (value === '') {
      this.focusCode();
      return;
    }
    this.submitting.set(true);
    try {
      await this.auth.completeTwoFactor(
        this.useRecovery() ? { recoveryCode: value } : { code: value },
      );
      goAfterAuth(safeRedirect(this.returnUrl, this.home), this.router, this.assign);
    } catch (error) {
      this.formError.set(authErrorMessage(error, 'login'));
      if (!this.challenge()) {
        // The challenge expired: back to the credentials with the reason shown.
        this.submitted.set(false);
        this.codeForm.reset();
      } else {
        this.codeForm.reset();
        this.focusCode();
      }
    } finally {
      this.submitting.set(false);
    }
  }

  protected toggleRecovery(): void {
    this.useRecovery.update((value) => !value);
    this.codeForm.reset();
    this.formError.set(null);
    this.focusCode();
  }

  protected backToCredentials(): void {
    this.auth.cancelTwoFactor();
    this.codeForm.reset();
    this.useRecovery.set(false);
    this.formError.set(null);
    this.submitted.set(false);
  }

  private focusCode(): void {
    afterNextRender(
      () =>
        this.host.nativeElement
          .querySelector<HTMLInputElement>('[data-testid="two-factor-code"]')
          ?.focus(),
      { injector: this.injector },
    );
  }
}

import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  inject,
  signal,
} from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
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

import { goAfterAuth, focusFirstInvalid } from '../auth-navigation';
import { AuthField } from '../shared/auth-field';
import { AuthLayout } from '../shared/auth-layout';

const GENERIC_ERROR = 'No se pudo iniciar sesión. Revisa tus datos e inténtalo de nuevo.';
const EMAIL_MESSAGE = 'Introduce un correo electrónico válido';
const PASSWORD_MESSAGE = 'La contraseña es obligatoria';

/** Sign-in screen. Server field details are shown under their fields; other failures in an alert. */
@Component({
  selector: 'app-login-page',
  imports: [ReactiveFormsModule, RouterLink, AuthLayout, AuthField, PasswordInput],
  templateUrl: './login-page.html',
  styleUrl: '../shared/_auth-form.scss',
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
  protected readonly registerQuery: Readonly<Record<string, string>> = this.returnUrl
    ? { returnUrl: this.returnUrl }
    : {};

  protected readonly form = new FormGroup({
    email: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.email],
    }),
    password: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
  });

  protected readonly submitted = signal(false);
  protected readonly submitting = signal(false);
  protected readonly formError = signal<string | null>(null);

  protected emailError(): string | null {
    const control = this.form.controls.email;
    if (control.hasError('server')) return control.getError('server');
    return this.submitted() && (control.hasError('required') || control.hasError('email'))
      ? EMAIL_MESSAGE
      : null;
  }

  protected passwordError(): string | null {
    const control = this.form.controls.password;
    if (control.hasError('server')) return control.getError('server');
    return this.submitted() && control.hasError('required') ? PASSWORD_MESSAGE : null;
  }

  protected async onSubmit(): Promise<void> {
    this.submitted.set(true);
    this.formError.set(null);
    // Drops server errors from the previous attempt so they do not block the next submit.
    this.form.updateValueAndValidity();
    if (this.form.invalid) {
      afterNextRender(() => focusFirstInvalid(this.host.nativeElement), {
        injector: this.injector,
      });
      return;
    }

    this.submitting.set(true);
    try {
      await this.auth.login(this.form.getRawValue());
      goAfterAuth(safeRedirect(this.returnUrl, this.home), this.router, this.assign);
    } catch (error) {
      const apiError = asApiError(error);
      if (!applyServerErrors(this.form, apiError)) {
        this.formError.set(apiError.message || GENERIC_ERROR);
      }
    } finally {
      this.submitting.set(false);
    }
  }
}

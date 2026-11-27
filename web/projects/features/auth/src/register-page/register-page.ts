import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  inject,
  signal,
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
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
import { PasswordStrength } from '@corelog/ui/password-strength';

import { goAfterAuth, focusFirstInvalid } from '../auth-navigation';
import { AuthField } from '../shared/auth-field';
import { AuthLayout } from '../shared/auth-layout';

const GENERIC_ERROR = 'No se pudo crear la cuenta. Inténtalo de nuevo en unos minutos.';
const EMAIL_MESSAGE = 'Introduce un correo electrónico válido';
const NAME_MESSAGE = 'El nombre debe tener al menos 2 caracteres';
const PASSWORD_MESSAGE = 'La contraseña debe tener al menos 8 caracteres';
const PASSWORD_HELPER = 'Mínimo 8 caracteres.';

/** Account creation screen. Signs the new account in on success. */
@Component({
  selector: 'app-register-page',
  imports: [
    ReactiveFormsModule,
    RouterLink,
    AuthLayout,
    AuthField,
    PasswordInput,
    PasswordStrength,
  ],
  templateUrl: './register-page.html',
  styleUrl: '../shared/_auth-form.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RegisterPage {
  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly home = inject(HOME_URL);
  private readonly assign = inject(LOCATION_ASSIGN);

  protected readonly returnUrl = inject(ActivatedRoute).snapshot.queryParamMap.get('returnUrl');
  protected readonly loginQuery: Readonly<Record<string, string>> = this.returnUrl
    ? { returnUrl: this.returnUrl }
    : {};

  protected readonly form = new FormGroup({
    name: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.minLength(2)],
    }),
    email: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.email],
    }),
    password: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.minLength(8)],
    }),
  });

  /** Live password value for the strength meter. */
  protected readonly password = toSignal(
    this.form.controls.password.valueChanges.pipe(takeUntilDestroyed()),
    { initialValue: '' },
  );

  protected readonly submitted = signal(false);
  protected readonly submitting = signal(false);
  protected readonly formError = signal<string | null>(null);

  protected nameError(): string | null {
    const control = this.form.controls.name;
    if (control.hasError('server')) return control.getError('server');
    return this.submitted() && (control.hasError('required') || control.hasError('minlength'))
      ? NAME_MESSAGE
      : null;
  }

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
    return this.submitted() && (control.hasError('required') || control.hasError('minlength'))
      ? PASSWORD_MESSAGE
      : null;
  }

  /** Helper text under the password, replaced by the error while there is one. */
  protected passwordHelper(): string | null {
    return this.passwordError() ? null : PASSWORD_HELPER;
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
      await this.auth.register(this.form.getRawValue());
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

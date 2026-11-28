import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
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
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import {
  AuthStore,
  FIRST_NAME_MAX,
  HOME_URL,
  LAST_NAME_MAX,
  LOCATION_ASSIGN,
  asApiError,
  identityFieldErrors,
  missingConsentSlugs,
  normalizeName,
  passwordMatchValidator,
  personNameValidator,
  revalidateOnPasswordChange,
  safeRedirect,
  type ConsentRef,
  type IdentityField,
  type PhoneValue,
} from '@corelog/core';
import { LegalConsentGroup } from '@corelog/ui/legal-consent-checkbox';
import { PasswordInput } from '@corelog/ui/password-input';
import { PasswordStrength } from '@corelog/ui/password-strength';
import { PhoneInput } from '@corelog/ui/phone-input';
import { Button } from 'primeng/button';

import { authErrorMessage } from '../auth-messages';
import { focusFirstInvalid, goAfterAuth } from '../auth-navigation';
import { AuthAlert } from '../shared/auth-alert';
import { AuthControl, AuthField } from '../shared/auth-field';
import { AuthLayout } from '../shared/auth-layout';

const NAME_PATTERN_MESSAGE = 'Usa solo letras, espacios, guiones y apóstrofes';
const PASSWORD_HELPER = 'Mínimo 8 caracteres.';
const EMAIL_MESSAGE = 'Introduce un correo electrónico válido';
const PASSWORD_MESSAGE = 'La contraseña debe tener al menos 8 caracteres';
const CONSENTS_FAILED = 'Acepta los documentos obligatorios para crear la cuenta.';

/** Account creation screen. Signs the new account in on success. */
@Component({
  selector: 'app-register-page',
  imports: [
    ReactiveFormsModule,
    AuthLayout,
    AuthAlert,
    AuthField,
    AuthControl,
    LegalConsentGroup,
    PasswordInput,
    PasswordStrength,
    PhoneInput,
    Button,
  ],
  templateUrl: './register-page.html',
  styleUrl: './register-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RegisterPage {
  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly home = inject(HOME_URL);
  private readonly assign = inject(LOCATION_ASSIGN);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly returnUrl = inject(ActivatedRoute).snapshot.queryParamMap.get('returnUrl');
  protected readonly loginQuery: Readonly<Record<string, string>> = this.returnUrl
    ? { returnUrl: this.returnUrl }
    : {};

  protected readonly firstNameMessages = {
    required: 'Escribe tus nombres',
    maxlength: `Máximo ${FIRST_NAME_MAX} caracteres`,
    pattern: NAME_PATTERN_MESSAGE,
  };
  protected readonly lastNameMessages = {
    required: 'Escribe tus apellidos',
    maxlength: `Máximo ${LAST_NAME_MAX} caracteres`,
    pattern: NAME_PATTERN_MESSAGE,
  };
  protected readonly phoneMessages = {
    required: 'Introduce tu teléfono',
    phone: 'El teléfono no es válido para el país elegido',
  };
  protected readonly emailMessages = { required: EMAIL_MESSAGE, email: EMAIL_MESSAGE };
  protected readonly passwordMessages = { required: PASSWORD_MESSAGE, minlength: PASSWORD_MESSAGE };
  protected readonly confirmationMessages = {
    required: 'Confirma tu contraseña',
    mismatch: 'Las contraseñas no coinciden',
  };
  protected readonly passwordHelper = PASSWORD_HELPER;

  protected readonly form = new FormGroup({
    firstName: new FormControl('', {
      nonNullable: true,
      validators: [personNameValidator(FIRST_NAME_MAX)],
    }),
    lastName: new FormControl('', {
      nonNullable: true,
      validators: [personNameValidator(LAST_NAME_MAX)],
    }),
    phone: new FormControl<PhoneValue | null>(null, { validators: [Validators.required] }),
    email: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.email],
    }),
    password: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.minLength(8)],
    }),
    passwordConfirmation: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, passwordMatchValidator('password')],
    }),
    consents: new FormControl<ConsentRef[]>([], { nonNullable: true }),
  });

  /** Live password value for the strength meter. */
  protected readonly password = toSignal(
    this.form.controls.password.valueChanges.pipe(takeUntilDestroyed()),
    { initialValue: '' },
  );

  protected readonly submitted = signal(false);
  protected readonly submitting = signal(false);
  protected readonly formError = signal<string | null>(null);
  protected readonly missingConsents = signal<readonly string[]>([]);

  constructor() {
    revalidateOnPasswordChange(this.form, 'password', 'passwordConfirmation', this.destroyRef);
  }

  /** Detail the server attached to the field, if any. */
  protected serverError(control: AbstractControl): string | undefined {
    return control.getError('server') ?? undefined;
  }

  protected async onSubmit(): Promise<void> {
    if (this.submitting()) return;
    this.submitted.set(true);
    this.formError.set(null);
    this.missingConsents.set([]);
    // Drops server errors from the previous attempt so they do not block the next submit.
    Object.values(this.form.controls).forEach((control) => control.updateValueAndValidity());
    if (this.form.invalid) {
      afterNextRender(() => focusFirstInvalid(this.host.nativeElement), {
        injector: this.injector,
      });
      return;
    }

    this.submitting.set(true);
    const value = this.form.getRawValue();
    try {
      await this.auth.register({
        firstName: normalizeName(value.firstName),
        lastName: normalizeName(value.lastName),
        phone: value.phone as PhoneValue,
        email: value.email.trim(),
        password: value.password,
        passwordConfirmation: value.passwordConfirmation,
        locale: 'es',
        consents: value.consents,
      });
      goAfterAuth(safeRedirect(this.returnUrl, this.home), this.router, this.assign);
    } catch (error) {
      this.showFailure(error);
    } finally {
      this.submitting.set(false);
    }
  }

  private showFailure(error: unknown): void {
    const missing = missingConsentSlugs(error);
    if (missing.length > 0) {
      this.missingConsents.set(missing);
      this.formError.set(CONSENTS_FAILED);
      return;
    }
    const fields = identityFieldErrors(error);
    const names = Object.keys(fields) as IdentityField[];
    for (const field of names) {
      const control = this.form.get(field);
      control?.setErrors({ ...control.errors, server: fields[field] });
      control?.markAsTouched();
    }
    if (names.length === 0) {
      this.formError.set(authErrorMessage(asApiError(error), 'register'));
    }
  }
}

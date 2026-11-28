import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  afterNextRender,
  inject,
  computed,
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
import { AuthStepper, type AuthStep } from '../shared/auth-stepper';

const NAME_PATTERN_MESSAGE = 'Usa solo letras, espacios, guiones y apóstrofes';
const PASSWORD_HELPER = 'Mínimo 8 caracteres.';
const EMAIL_MESSAGE = 'Introduce un correo electrónico válido';
const PASSWORD_MESSAGE = 'La contraseña debe tener al menos 8 caracteres';
const CONSENTS_FAILED = 'Acepta los documentos obligatorios para crear la cuenta.';

const STEPS: readonly AuthStep[] = [
  { label: 'Tus datos' },
  { label: 'Tu cuenta' },
  { label: 'Privacidad' },
];
const SUBTITLES: readonly string[] = [
  'Cuéntanos quién eres.',
  'Define cómo vas a acceder.',
  'Revisa y acepta para terminar.',
];
const LAST_STEP = STEPS.length - 1;

/** Step that holds each field of the form, to send server errors to the step where the person can fix them. */
const STEP_OF_FIELD: Readonly<Record<IdentityField, number>> = {
  firstName: 0,
  lastName: 0,
  phone: 0,
  email: 1,
  password: 1,
  passwordConfirmation: 1,
};

/** Account creation in three steps (data, account, privacy). Signs the new account in on success. */
@Component({
  selector: 'app-register-page',
  imports: [
    ReactiveFormsModule,
    AuthLayout,
    AuthAlert,
    AuthField,
    AuthControl,
    AuthStepper,
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

  protected readonly steps = STEPS;
  protected readonly lastStep = LAST_STEP;
  /** Zero-based index of the step being filled. */
  protected readonly step = signal(0);
  protected readonly subtitle = computed(() => SUBTITLES[this.step()]);
  /** Text read out by the live region each time the step changes. */
  protected readonly announcement = signal('');
  /** Slide direction of the step transition. */
  protected readonly direction = signal<'forward' | 'back'>('forward');
  /** Steps whose "Siguiente" was pressed: their invalid fields show their messages. */
  protected readonly attempted = signal<readonly boolean[]>([false, false, false]);
  protected readonly submitted = computed(() => this.attempted()[LAST_STEP]);
  protected readonly submitting = signal(false);
  protected readonly formError = signal<string | null>(null);
  protected readonly missingConsents = signal<readonly string[]>([]);

  constructor() {
    revalidateOnPasswordChange(this.form, 'password', 'passwordConfirmation', this.destroyRef);
  }

  private readonly stepControls: readonly (readonly AbstractControl[])[] = [
    [this.form.controls.firstName, this.form.controls.lastName, this.form.controls.phone],
    [
      this.form.controls.email,
      this.form.controls.password,
      this.form.controls.passwordConfirmation,
    ],
    [this.form.controls.consents],
  ];

  /** Detail the server attached to the field, if any. */
  protected serverError(control: AbstractControl): string | undefined {
    return control.getError('server') ?? undefined;
  }

  /** True once the fields of the step are valid; otherwise shows their messages and focuses the first one. */
  private validateStep(index: number): boolean {
    this.attempted.update((flags) => flags.map((flag, i) => flag || i === index));
    this.formError.set(null);
    this.missingConsents.set([]);
    const controls = this.stepControls[index];
    // Drops server errors from the previous attempt so they do not block the next one.
    controls.forEach((control) => {
      control.markAsTouched();
      control.updateValueAndValidity();
    });
    if (controls.every((control) => control.valid)) return true;
    this.focusIn(index, true);
    return false;
  }

  /** Moves to a step, announces it and puts focus on its first field. */
  protected goTo(index: number): void {
    if (index === this.step() || index < 0 || index > LAST_STEP) return;
    this.direction.set(index > this.step() ? 'forward' : 'back');
    this.step.set(index);
    this.announcement.set(`Paso ${index + 1} de ${STEPS.length}: ${STEPS[index].label}`);
    this.focusIn(index, false);
  }

  protected next(): void {
    if (this.validateStep(this.step())) this.goTo(this.step() + 1);
  }

  protected back(): void {
    this.formError.set(null);
    this.goTo(this.step() - 1);
  }

  /** Focus after the step is visible: the first invalid control, or the first control of the step. */
  private focusIn(index: number, invalidOnly: boolean): void {
    afterNextRender(
      () => {
        const root = this.host.nativeElement.querySelector<HTMLElement>(
          `[data-testid="register-step-${index + 1}"]`,
        );
        if (!root) return;
        const invalid = root.querySelector<HTMLElement>('[aria-invalid="true"]');
        if (invalid) {
          invalid.focus();
          return;
        }
        if (!invalidOnly) {
          root.querySelector<HTMLElement>('input:not([disabled]), button:not([disabled])')?.focus();
        }
      },
      { injector: this.injector },
    );
  }

  protected async onSubmit(): Promise<void> {
    if (this.submitting()) return;
    // Enter on the first steps advances; only the last one creates the account.
    if (this.step() < LAST_STEP) {
      this.next();
      return;
    }
    this.formError.set(null);
    this.missingConsents.set([]);
    this.attempted.set([true, true, true]);
    Object.values(this.form.controls).forEach((control) => control.updateValueAndValidity());
    if (this.form.invalid) {
      const first = this.stepControls.findIndex((controls) => controls.some((c) => c.invalid));
      if (first >= 0 && first !== this.step()) this.goTo(first);
      this.focusIn(Math.max(first, 0), true);
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
      this.goTo(LAST_STEP);
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
      return;
    }
    // The first offending field decides the step the person lands on.
    const target = Math.min(...names.map((field) => STEP_OF_FIELD[field]));
    if (target !== this.step()) this.goTo(target);
    this.focusIn(target, true);
  }
}

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
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import {
  AuthStore,
  HOME_URL,
  LOCATION_ASSIGN,
  applyServerErrors,
  asApiError,
  safeRedirect,
} from '@corelog/core';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { PasswordInput } from '@corelog/ui/password-input';
import { PasswordStrength } from '@corelog/ui/password-strength';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';

import { authErrorMessage } from '../auth-messages';
import { focusFirstInvalid, goAfterAuth } from '../auth-navigation';
import { AuthAlert } from '../shared/auth-alert';
import { AuthLayout } from '../shared/auth-layout';

const NAME_MESSAGE = 'El nombre debe tener al menos 2 caracteres';
const PASSWORD_HELPER = 'Mínimo 8 caracteres.';
const EMAIL_MESSAGE = 'Introduce un correo electrónico válido';
const PASSWORD_MESSAGE = 'La contraseña debe tener al menos 8 caracteres';

/** Account creation screen. Signs the new account in on success. */
@Component({
  selector: 'app-register-page',
  imports: [
    ReactiveFormsModule,
    AuthLayout,
    AuthAlert,
    FieldRow,
    FieldControl,
    PasswordInput,
    PasswordStrength,
    Button,
    InputText,
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

  protected readonly returnUrl = inject(ActivatedRoute).snapshot.queryParamMap.get('returnUrl');
  protected readonly loginQuery: Readonly<Record<string, string>> = this.returnUrl
    ? { returnUrl: this.returnUrl }
    : {};

  protected readonly emailMessages = { required: EMAIL_MESSAGE, email: EMAIL_MESSAGE };
  protected readonly nameMessages = { required: NAME_MESSAGE, minlength: NAME_MESSAGE };
  protected readonly passwordMessages = { required: PASSWORD_MESSAGE, minlength: PASSWORD_MESSAGE };
  protected readonly passwordHelper = PASSWORD_HELPER;

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
      await this.auth.register(this.form.getRawValue());
      goAfterAuth(safeRedirect(this.returnUrl, this.home), this.router, this.assign);
    } catch (error) {
      if (!applyServerErrors(this.form, asApiError(error))) {
        this.formError.set(authErrorMessage(error, 'register'));
      }
    } finally {
      this.submitting.set(false);
    }
  }
}

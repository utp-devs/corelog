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
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';

import { authErrorMessage } from '../auth-messages';
import { focusFirstInvalid, goAfterAuth } from '../auth-navigation';
import { AuthAlert } from '../shared/auth-alert';
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
    FieldRow,
    FieldControl,
    PasswordInput,
    Button,
    InputText,
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
      goAfterAuth(safeRedirect(this.returnUrl, this.home), this.router, this.assign);
    } catch (error) {
      if (!applyServerErrors(this.form, asApiError(error))) {
        this.formError.set(authErrorMessage(error, 'login'));
      }
    } finally {
      this.submitting.set(false);
    }
  }
}

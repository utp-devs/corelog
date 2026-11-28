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
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import {
  AbstractControl,
  FormControl,
  FormGroup,
  ReactiveFormsModule,
  Validators,
} from '@angular/forms';
import { ActivatedRoute, RouterLink } from '@angular/router';
import {
  PasswordResetApiService,
  asApiError,
  isRateLimited,
  passwordMatchValidator,
  rateLimitMessage,
  revalidateOnPasswordChange,
} from '@corelog/core';
import { PasswordInput } from '@corelog/ui/password-input';
import { PasswordStrength } from '@corelog/ui/password-strength';
import { Button } from 'primeng/button';
import { firstValueFrom } from 'rxjs';

import { focusFirstInvalid } from '../auth-navigation';
import { AuthAlert } from '../shared/auth-alert';
import { AuthField } from '../shared/auth-field';
import { AuthLayout } from '../shared/auth-layout';

export type ResetState = 'form' | 'done' | 'invalid';

const WEAK_PASSWORD =
  'La contraseña es demasiado débil. Usa al menos 10 caracteres combinando mayúsculas, minúsculas, números o símbolos.';
const MISMATCH = 'Las contraseñas no coinciden';

/** Second step of a password reset: the person arrives from the emailed link and chooses a new password. */
@Component({
  selector: 'app-reset-password-page',
  imports: [
    ReactiveFormsModule,
    RouterLink,
    AuthLayout,
    AuthAlert,
    AuthField,
    PasswordInput,
    PasswordStrength,
    Button,
  ],
  templateUrl: './reset-password-page.html',
  styleUrl: './reset-password-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ResetPasswordPage {
  private readonly api = inject(PasswordResetApiService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly token = inject(ActivatedRoute).snapshot.queryParamMap.get('token') ?? '';

  protected readonly state = signal<ResetState>(this.token ? 'form' : 'invalid');
  protected readonly submitted = signal(false);
  protected readonly submitting = signal(false);
  protected readonly formError = signal<string | null>(null);

  protected readonly passwordMessages = {
    required: 'Escribe la contraseña nueva',
    minlength: 'La contraseña debe tener al menos 10 caracteres',
  };
  protected readonly confirmationMessages = {
    required: 'Confirma tu contraseña',
    mismatch: MISMATCH,
  };

  protected readonly form = new FormGroup({
    password: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.minLength(10)],
    }),
    passwordConfirmation: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, passwordMatchValidator('password')],
    }),
  });

  /** Live password value for the strength meter. */
  protected readonly password = toSignal(
    this.form.controls.password.valueChanges.pipe(takeUntilDestroyed()),
    { initialValue: '' },
  );

  constructor() {
    revalidateOnPasswordChange(this.form, 'password', 'passwordConfirmation', this.destroyRef);
  }

  protected serverError(control: AbstractControl): string | undefined {
    return control.getError('server') ?? undefined;
  }

  protected async onSubmit(): Promise<void> {
    if (this.submitting()) return;
    this.submitted.set(true);
    this.formError.set(null);
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
      await firstValueFrom(
        this.api.reset({
          token: this.token,
          password: value.password,
          passwordConfirmation: value.passwordConfirmation,
        }),
      );
      this.state.set('done');
    } catch (error) {
      this.showFailure(error);
    } finally {
      this.submitting.set(false);
    }
  }

  private showFailure(error: unknown): void {
    const api = asApiError(error);
    if (api.code === 'auth.reset_token_invalid') {
      this.state.set('invalid');
    } else if (api.code === 'user.password_weak') {
      this.fieldError('password', WEAK_PASSWORD);
    } else if (api.code === 'user.password_mismatch') {
      this.fieldError('passwordConfirmation', MISMATCH);
    } else if (isRateLimited(api)) {
      this.formError.set(rateLimitMessage(api));
    } else if (api.status === 0) {
      this.formError.set(api.message);
    } else {
      this.formError.set('No se pudo cambiar la contraseña. Inténtalo de nuevo en unos minutos.');
    }
  }

  private fieldError(name: 'password' | 'passwordConfirmation', message: string): void {
    const control = this.form.controls[name];
    control.setErrors({ ...control.errors, server: message });
    control.markAsTouched();
    afterNextRender(() => focusFirstInvalid(this.host.nativeElement), { injector: this.injector });
  }
}

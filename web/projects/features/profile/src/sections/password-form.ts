import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  inject,
  input,
  signal,
} from '@angular/core';
import {
  AbstractControl,
  Validators,
  FormControl,
  FormGroup,
  ReactiveFormsModule,
  ValidationErrors,
} from '@angular/forms';
import {
  ChangePasswordPayload,
  passwordMatchValidator,
  revalidateOnPasswordChange,
} from '@corelog/core';
import { FieldRow } from '@corelog/ui/field-row';
import { PasswordInput } from '@corelog/ui/password-input';
import { PasswordStrength } from '@corelog/ui/password-strength';
import { Button } from 'primeng/button';

const MIN_NEW_PASSWORD_LENGTH = 8;

function requiredCurrent(control: AbstractControl<string>): ValidationErrors | null {
  return control.value ? null : { invalid: 'La contraseña actual es obligatoria' };
}

function minimumNew(control: AbstractControl<string>): ValidationErrors | null {
  return control.value.length >= MIN_NEW_PASSWORD_LENGTH
    ? null
    : { invalid: 'La nueva contraseña debe tener al menos 8 caracteres' };
}

/** Password change form. The form is cleared after a successful change and keeps its values after a failure. */
@Component({
  selector: 'app-profile-password-form',
  imports: [Button, FieldRow, PasswordInput, PasswordStrength, ReactiveFormsModule],
  templateUrl: './password-form.html',
  styleUrl: './password-form.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfilePasswordForm {
  readonly isSubmitting = input(false);
  /** Rejects when the change fails; the caller reports the failure. */
  readonly save = input.required<(payload: ChangePasswordPayload) => Promise<void>>();

  protected readonly form = new FormGroup({
    currentPassword: new FormControl('', { nonNullable: true, validators: [requiredCurrent] }),
    newPassword: new FormControl('', { nonNullable: true, validators: [minimumNew] }),
    newPasswordConfirmation: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, passwordMatchValidator('newPassword')],
    }),
  });

  protected readonly confirmationMessages = {
    required: 'Confirma la nueva contraseña',
    mismatch: 'Las contraseñas no coinciden',
  };

  constructor() {
    revalidateOnPasswordChange(
      this.form,
      'newPassword',
      'newPasswordConfirmation',
      inject(DestroyRef),
    );
  }

  /** True after the first submit: errors of untouched fields become visible. */
  protected readonly submitted = signal(false);

  protected async submit(): Promise<void> {
    this.submitted.set(true);
    this.form.updateValueAndValidity();
    if (this.form.invalid) return;

    try {
      await this.save()(this.form.getRawValue());
      this.form.reset({ currentPassword: '', newPassword: '', newPasswordConfirmation: '' });
      this.submitted.set(false);
    } catch {
      // The caller has reported the failure; the values stay for another try.
    }
  }
}

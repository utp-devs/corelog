import { ChangeDetectionStrategy, Component, input, signal } from '@angular/core';
import {
  AbstractControl,
  FormControl,
  FormGroup,
  ReactiveFormsModule,
  ValidationErrors,
} from '@angular/forms';
import { ChangePasswordPayload } from '@corelog/core';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { PasswordInput } from '@corelog/ui/password-input';
import { Button } from 'primeng/button';

const MIN_NEW_PASSWORD_LENGTH = 8;

type FieldName = 'currentPassword' | 'newPassword';

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
  imports: [Button, FieldRow, PasswordInput, ReactiveFormsModule],
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
  });

  protected readonly submitted = signal(false);

  /** Error text for a field; shown after the first submit and updated as the user types. */
  protected errorOf(name: FieldName): string | undefined {
    const errors: ValidationErrors | null = this.form.controls[name].errors;
    if (!errors || !this.submitted()) return undefined;
    return typeof errors['invalid'] === 'string' ? errors['invalid'] : undefined;
  }

  protected async submit(): Promise<void> {
    this.submitted.set(true);
    this.form.updateValueAndValidity();
    if (this.form.invalid) return;

    try {
      await this.save()(this.form.getRawValue());
      this.form.reset({ currentPassword: '', newPassword: '' });
      this.submitted.set(false);
    } catch {
      // The caller has reported the failure; the values stay for another try.
    }
  }
}

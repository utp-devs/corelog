import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  effect,
  inject,
  input,
  model,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  AbstractControl,
  FormControl,
  FormGroup,
  ReactiveFormsModule,
  ValidationErrors,
} from '@angular/forms';
import { ProfileUpdate, profileFieldErrors, User } from '@corelog/core';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Select } from 'primeng/select';

const MAX_TEXT_LENGTH = 120;
const NAME_MIN_LENGTH = 2;

type FieldName = 'name' | 'displayName' | 'jobTitle' | 'department' | 'timezone';

interface ProfileValues {
  readonly name: string;
  readonly displayName: string;
  readonly jobTitle: string;
  readonly department: string;
  readonly timezone: string;
}

/** Client-side messages; the server messages arrive as `server` errors after a failed save. */
function nameLength(control: AbstractControl<string>): ValidationErrors | null {
  return control.value.trim().length >= NAME_MIN_LENGTH
    ? null
    : { invalid: 'El nombre debe tener al menos 2 caracteres' };
}

function timezoneRequired(control: AbstractControl<string>): ValidationErrors | null {
  return control.value ? null : { invalid: 'Elige una zona horaria' };
}

function valuesOf(user: User): ProfileValues {
  return {
    name: user.name,
    displayName: user.displayName,
    jobTitle: user.jobTitle,
    department: user.department,
    timezone: user.timezone || 'UTC',
  };
}

/** Personal data form. Saves through the `save` callback and maps server field errors to the fields. */
@Component({
  selector: 'app-profile-form',
  imports: [Button, FieldControl, FieldRow, InputText, ReactiveFormsModule, Select],
  templateUrl: './profile-form.html',
  styleUrl: './profile-form.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfileForm {
  private readonly changeDetector = inject(ChangeDetectorRef);

  readonly user = input.required<User>();
  readonly timezones = input.required<string[]>();
  readonly isSubmitting = input(false);
  /** Rejects on failure; errors that do not belong to a field are reported by the caller. */
  readonly save = input.required<(payload: ProfileUpdate) => Promise<void>>();
  /** True while the form has edits that are not saved. */
  readonly dirty = model(false);

  protected readonly form = new FormGroup({
    name: new FormControl('', { nonNullable: true, validators: [nameLength] }),
    displayName: new FormControl('', { nonNullable: true }),
    jobTitle: new FormControl('', { nonNullable: true }),
    department: new FormControl('', { nonNullable: true }),
    timezone: new FormControl('', { nonNullable: true, validators: [timezoneRequired] }),
  });

  protected readonly submitted = signal(false);
  private appliedValues = '';

  constructor() {
    // The form follows the account. A new account object with the same values keeps unsaved edits.
    effect(() => {
      const values = valuesOf(this.user());
      const serialized = JSON.stringify(values);
      if (serialized === this.appliedValues) return;
      this.appliedValues = serialized;
      this.form.reset(values);
      this.submitted.set(false);
      this.dirty.set(false);
    });

    this.form.valueChanges.pipe(takeUntilDestroyed()).subscribe(() => {
      this.dirty.set(this.form.dirty);
    });
  }

  /** Error text for a field: server errors at once, client errors after the first submit. */
  protected errorOf(name: FieldName): string | undefined {
    const errors: ValidationErrors | null = this.form.controls[name].errors;
    if (!errors) return undefined;
    if (typeof errors['server'] === 'string') return errors['server'];
    if (!this.submitted()) return undefined;
    if (typeof errors['invalid'] === 'string') return errors['invalid'];
    if (errors['maxlength']) return `Máximo ${MAX_TEXT_LENGTH} caracteres`;
    return undefined;
  }

  protected async submit(): Promise<void> {
    this.submitted.set(true);
    this.form.updateValueAndValidity();
    if (this.form.invalid) return;

    const values = this.form.getRawValue();
    try {
      await this.save()({ ...values, name: values.name.trim() });
    } catch (error) {
      for (const [field, message] of Object.entries(profileFieldErrors(error))) {
        this.form.get(field)?.setErrors({ server: message });
      }
      this.changeDetector.markForCheck();
    }
  }
}

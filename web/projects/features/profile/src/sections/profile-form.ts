import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  computed,
  effect,
  inject,
  input,
  model,
  output,
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
import {
  FIRST_NAME_MAX,
  LAST_NAME_MAX,
  ProfileUpdate,
  identityFieldErrors,
  normalizeName,
  personNameValidator,
  profileFieldErrors,
  samePhone,
  type PhoneValue,
  User,
} from '@corelog/core';
import { PhoneInput } from '@corelog/ui/phone-input';
import { ToneTag } from '@corelog/ui/tone-tag';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';

import { PhoneVerifyDialog } from './phone-verify-dialog';
import { InputText } from 'primeng/inputtext';

const MAX_TEXT_LENGTH = 120;

type FieldName =
  'firstName' | 'lastName' | 'phone' | 'displayName' | 'jobTitle' | 'department' | 'timezone';

interface ProfileValues {
  readonly firstName: string;
  readonly lastName: string;
  readonly phone: PhoneValue | null;
  readonly displayName: string;
  readonly jobTitle: string;
  readonly department: string;
  readonly timezone: string;
}

const NAME_MESSAGES = {
  required: 'Escribe tu nombre',
  maxlength: 'Es demasiado largo',
  pattern: 'Usa solo letras, espacios, guiones y apóstrofes',
} as const;

/** Given names and surnames as the form edits them; an older account only has the full name. */
function namesOf(user: User): { firstName: string; lastName: string } {
  if (user.firstName || user.lastName) {
    return { firstName: user.firstName ?? '', lastName: user.lastName ?? '' };
  }
  const [first = '', ...rest] = user.name.trim().split(/s+/);
  return { firstName: first, lastName: rest.join(' ') };
}

function timezoneRequired(control: AbstractControl<string>): ValidationErrors | null {
  return control.value ? null : { invalid: 'Elige una zona horaria' };
}

function valuesOf(user: User): ProfileValues {
  return {
    ...namesOf(user),
    phone: user.phone ? { country: user.phone.country, number: user.phone.number } : null,
    displayName: user.displayName,
    jobTitle: user.jobTitle,
    department: user.department,
    timezone: user.timezone || 'UTC',
  };
}

/** Personal data form. Saves through the `save` callback and maps server field errors to the fields. */
@Component({
  selector: 'app-profile-form',
  imports: [
    Button,
    FieldControl,
    FieldRow,
    InputText,
    PhoneInput,
    PhoneVerifyDialog,
    ReactiveFormsModule,
    SelectField,
    ToneTag,
  ],
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
  /** Emits the account after the phone was verified. */
  readonly verified = output<User>();

  protected readonly verifyOpen = signal(false);
  /** The saved phone is shown with its verification state only while the field still holds it. */
  protected readonly savedPhone = computed(() => this.user().phone ?? null);

  protected readonly form = new FormGroup({
    firstName: new FormControl('', {
      nonNullable: true,
      validators: [personNameValidator(FIRST_NAME_MAX)],
    }),
    lastName: new FormControl('', {
      nonNullable: true,
      validators: [personNameValidator(LAST_NAME_MAX)],
    }),
    phone: new FormControl<PhoneValue | null>(null),
    displayName: new FormControl('', { nonNullable: true }),
    jobTitle: new FormControl('', { nonNullable: true }),
    department: new FormControl('', { nonNullable: true }),
    timezone: new FormControl('', { nonNullable: true, validators: [timezoneRequired] }),
  });

  protected readonly timezoneOptions = computed<SelectFieldOption[]>(() =>
    this.timezones().map((zone) => ({ value: zone, label: zone })),
  );
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
    if (errors['maxlength']) {
      return name === 'firstName' || name === 'lastName'
        ? NAME_MESSAGES.maxlength
        : `Máximo ${MAX_TEXT_LENGTH} caracteres`;
    }
    if (name === 'firstName' || name === 'lastName') {
      if (errors['required']) return NAME_MESSAGES.required;
      if (errors['pattern']) return NAME_MESSAGES.pattern;
    }
    if (name === 'phone' && errors['phone']) return 'El teléfono no es válido para el país elegido';
    return undefined;
  }

  protected async submit(): Promise<void> {
    this.submitted.set(true);
    this.form.updateValueAndValidity();
    if (this.form.invalid) return;

    const { phone, firstName, lastName, ...rest } = this.form.getRawValue();
    const payload: ProfileUpdate = {
      ...rest,
      firstName: normalizeName(firstName),
      lastName: normalizeName(lastName),
      // The phone is sent only when it changed: sending it again would drop its verification.
      ...(samePhone(phone, this.user().phone) ? {} : { phone }),
    };
    try {
      await this.save()(payload);
    } catch (error) {
      for (const [field, message] of Object.entries({
        ...profileFieldErrors(error),
        ...identityFieldErrors(error),
      })) {
        this.form.get(field)?.setErrors({ server: message });
      }
      this.changeDetector.markForCheck();
    }
  }
}

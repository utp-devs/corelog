import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  inject,
  output,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  FormControl,
  FormGroup,
  ReactiveFormsModule,
  ValidatorFn,
  Validators,
} from '@angular/forms';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { InputText } from 'primeng/inputtext';
import {
  OrganizationStore,
  applyServerErrors,
  asApiError,
  isValidOrgSlug,
  orgCreateErrorMessage,
  slugFromName,
} from '@corelog/core';
import { FieldRow } from '@corelog/ui/field-row';

import { fieldMessage, ruleOf } from '../shared/field-message';

const SLUG_RULE =
  'Usa 3 a 40 caracteres: minúsculas, números y guiones, sin guion al principio ni al final.';

const slugValidator: ValidatorFn = ruleOf(
  (value) => typeof value === 'string' && isValidOrgSlug(value),
  SLUG_RULE,
);

/** Wizard step 2: name and slug of the new organization, with live slug validation and server errors. */
@Component({
  selector: 'app-org-form-step',
  imports: [Button, InputText, Message, FieldRow, ReactiveFormsModule],
  templateUrl: './org-form-step.html',
  styleUrl: './org-form-step.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgFormStep {
  private readonly orgs = inject(OrganizationStore);
  private readonly destroyRef = inject(DestroyRef);

  /** The organization was created and is now the active one. */
  readonly created = output<void>();
  /** Back to the choice step. */
  readonly back = output<void>();

  protected readonly slugRule = SLUG_RULE;
  protected readonly submitting = signal(false);
  protected readonly formError = signal<string | null>(null);
  protected readonly submitted = signal(false);
  private slugEdited = false;

  protected readonly form = new FormGroup({
    name: new FormControl('', {
      nonNullable: true,
      validators: [
        ruleOf(
          (value) => typeof value === 'string' && value.trim().length > 0,
          'El nombre es obligatorio',
        ),
        ruleOf((value) => String(value ?? '').length <= 120, 'Usa como máximo 120 caracteres'),
      ],
    }),
    slug: new FormControl('', {
      nonNullable: true,
      validators: [slugValidator, Validators.required],
    }),
  });

  constructor() {
    // Derived slug: follows the name until the user edits the slug by hand.
    this.form.controls.name.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((name) => {
        if (!this.slugEdited) {
          this.form.controls.slug.setValue(slugFromName(name), { emitEvent: false });
        }
      });

    this.form.controls.slug.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((value) => {
        this.slugEdited = true;
        const lower = value.toLowerCase();
        if (lower !== value) this.form.controls.slug.setValue(lower, { emitEvent: false });
      });
  }

  /** Message under a field: the server or rule message once the field is dirty or the form was submitted. */
  protected errorOf(control: FormControl<string>): string | undefined {
    if (!control.dirty && !control.touched && !this.submitted()) return undefined;
    return fieldMessage(control) ?? undefined;
  }

  protected submit(): void {
    this.submitted.set(true);
    this.form.markAllAsTouched();
    if (this.form.invalid) return;

    this.formError.set(null);
    this.submitting.set(true);
    const values = this.form.getRawValue();
    this.orgs
      .createOrg({ name: values.name.trim(), slug: values.slug })
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.submitting.set(false);
          this.created.emit();
        },
        error: (error: unknown) => {
          this.submitting.set(false);
          const apiError = asApiError(error);
          if (applyServerErrors(this.form, apiError)) return;
          if (apiError.code === 'org.slug_taken') {
            this.form.controls.slug.setErrors({
              server: 'Ese identificador ya está en uso. Elige otro.',
            });
            return;
          }
          if (apiError.code === 'org.slug_reserved') {
            this.form.controls.slug.setErrors({
              server: 'Ese identificador está reservado. Elige otro.',
            });
            return;
          }
          this.formError.set(orgCreateErrorMessage(error));
        },
      });
  }
}

import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { CatalogItem, CatalogKind } from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';
import { Textarea } from 'primeng/textarea';
import { startWith } from 'rxjs';

import { CATALOG_COPY } from './catalog-copy';
import { PalettePicker } from './palette-picker';

/** What the dialog hands back when it is saved. Icon and color are empty for resolutions. */
export interface CatalogFormValue {
  readonly name: string;
  readonly description: string;
  readonly icon: string | null;
  readonly color: string | null;
  readonly isDefault: boolean;
}

const DEFAULT_ICON: Readonly<Record<CatalogKind, string | null>> = {
  'issue-types': 'task',
  priorities: 'minus',
  resolutions: null,
};
const DEFAULT_COLOR: Readonly<Record<CatalogKind, string | null>> = {
  'issue-types': 'blue',
  priorities: 'gray',
  resolutions: null,
};

/** Create or edit dialog of a catalog item: name, description, icon and color (when the catalog has them), default. */
@Component({
  selector: 'app-catalog-form-dialog',
  imports: [
    Button,
    Dialog,
    FieldControl,
    FieldRow,
    InputText,
    Message,
    PalettePicker,
    ReactiveFormsModule,
    Textarea,
  ],
  templateUrl: './catalog-form-dialog.html',
  styleUrl: './catalog-form-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CatalogFormDialog {
  readonly open = input(false);
  readonly kind = input.required<CatalogKind>();
  /** Item being edited; null creates a new one. */
  readonly item = input<CatalogItem | null>(null);
  readonly saving = input(false);
  /** Failure of the last save, shown inside the dialog. */
  readonly error = input<string | null>(null);

  readonly saved = output<CatalogFormValue>();
  readonly closeRequested = output<void>();

  protected readonly copy = computed(() => CATALOG_COPY[this.kind()]);
  protected readonly heading = computed(() =>
    this.item() ? this.copy().editTitle : this.copy().newTitle,
  );

  protected readonly form = new FormGroup({
    name: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
    description: new FormControl('', {
      nonNullable: true,
      validators: [Validators.maxLength(500)],
    }),
    icon: new FormControl<string | null>(null),
    color: new FormControl<string | null>(null),
    isDefault: new FormControl(false, { nonNullable: true }),
  });
  protected readonly color = toSignal(
    this.form.controls.color.valueChanges.pipe(startWith(this.form.controls.color.value)),
    { initialValue: null },
  );
  protected readonly icon = toSignal(
    this.form.controls.icon.valueChanges.pipe(startWith(this.form.controls.icon.value)),
    { initialValue: null },
  );
  protected readonly submitted = signal(false);

  protected readonly nameMessages = {
    required: 'Escribe un nombre.',
    maxlength: 'El nombre es demasiado largo.',
  };

  constructor() {
    effect(() => {
      if (!this.open()) return;
      const item = this.item();
      const kind = this.kind();
      untracked(() => {
        this.submitted.set(false);
        this.form.controls.name.setValidators([
          Validators.required,
          Validators.maxLength(CATALOG_COPY[kind].nameMax),
        ]);
        this.form.reset({
          name: item?.name ?? '',
          description: item?.description ?? '',
          icon: item ? item.icon : DEFAULT_ICON[kind],
          color: item ? item.color : DEFAULT_COLOR[kind],
          isDefault: item?.isDefault ?? false,
        });
      });
    });
  }

  protected submit(): void {
    this.submitted.set(true);
    this.form.markAllAsTouched();
    if (this.form.invalid || this.saving()) return;
    const value = this.form.getRawValue();
    this.saved.emit({
      name: value.name.trim(),
      description: value.description.trim(),
      icon: this.copy().hasIcon ? value.icon : null,
      color: this.copy().hasIcon ? value.color : null,
      isDefault: value.isDefault,
    });
  }
}

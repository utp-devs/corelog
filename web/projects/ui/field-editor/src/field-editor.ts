import {
  ChangeDetectionStrategy,
  Component,
  computed,
  input,
  linkedSignal,
  output,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Checkbox } from 'primeng/checkbox';
import { MultiSelect } from 'primeng/multiselect';

import { DateField } from '@corelog/ui/date-field';
import { SelectField } from '@corelog/ui/select-field';
import { PickableUser, UserPicker } from '@corelog/ui/user-picker';

export interface FieldEditorOption {
  readonly id: string;
  readonly label: string;
  readonly archived?: boolean;
}

/**
 * Editor of one custom field value, chosen by the field type (text, paragraph, number, date, datetime,
 * select, multiselect, checkbox, user, url, labels). The edited value is local until it is committed:
 * on blur or Enter for text-like fields, on change for the pick-based ones. `valueChange` emits the
 * committed value (null when cleared; arrays for multiselect and labels; a boolean for checkbox).
 */
@Component({
  selector: 'app-field-editor',
  imports: [Checkbox, DateField, FormsModule, MultiSelect, SelectField, UserPicker],
  templateUrl: './field-editor.html',
  styleUrl: './field-editor.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FieldEditor {
  readonly type = input.required<string>();
  /** Accessible name of the control. */
  readonly label = input.required<string>();
  readonly value = input<unknown>(null);
  readonly options = input<readonly FieldEditorOption[]>([]);
  readonly users = input<readonly PickableUser[]>([]);
  readonly min = input<number | null>(null);
  readonly max = input<number | null>(null);
  readonly disabled = input(false);
  readonly invalid = input(false);
  readonly inputId = input<string>();

  readonly valueChange = output<unknown>();
  readonly userSearch = output<string>();

  /** Value being edited; reset whenever the committed value changes. */
  protected readonly draft = linkedSignal<unknown>(() => this.value());

  protected readonly text = computed(() => {
    const draft = this.draft();
    return draft === null || draft === undefined ? '' : String(draft);
  });
  protected readonly labelsText = computed(() => {
    const draft = this.draft();
    return Array.isArray(draft) ? draft.join(', ') : '';
  });
  protected readonly selectOptions = computed(() =>
    this.options()
      .filter((option) => !option.archived || option.id === this.draft())
      .map((option) => ({ value: option.id, label: option.label })),
  );
  protected readonly multiOptions = computed(() => {
    const current = this.draft();
    const selected = Array.isArray(current) ? current : [];
    return this.options()
      .filter((option) => !option.archived || selected.includes(option.id))
      .map((option) => ({ value: option.id, label: option.label }));
  });
  protected readonly pickable = computed<PickableUser[]>(() => [...this.users()]);
  protected readonly userValue = computed(() => {
    const draft = this.draft();
    return typeof draft === 'string' && draft !== '' ? draft : undefined;
  });
  protected readonly dateTimeText = computed(() => {
    const draft = this.draft();
    return typeof draft === 'string' ? draft.slice(0, 16) : '';
  });

  protected commitText(event: Event): void {
    const raw = (event.target as HTMLInputElement | HTMLTextAreaElement).value.trim();
    let next: unknown = raw === '' ? null : raw;
    if (this.type() === 'number' && next !== null) {
      const parsed = Number(raw.replace(',', '.'));
      if (Number.isNaN(parsed)) return;
      next = parsed;
    }
    this.draft.set(next);
    if (next !== this.value()) this.valueChange.emit(next);
  }

  protected commitDateTime(event: Event): void {
    const raw = (event.target as HTMLInputElement).value;
    const next = raw === '' ? null : new Date(raw).toISOString();
    this.draft.set(next);
    if (next !== this.value()) this.valueChange.emit(next);
  }

  protected commitLabels(event: Event): void {
    const raw = (event.target as HTMLInputElement).value;
    const next = raw
      .split(',')
      .map((part) => part.trim())
      .filter((part) => part !== '');
    this.draft.set(next);
    this.valueChange.emit(next);
  }

  protected commitValue(next: unknown): void {
    const normalized = next === undefined || next === '' ? null : next;
    this.draft.set(normalized);
    this.valueChange.emit(normalized);
  }

  protected blurOnEnter(event: Event): void {
    (event.target as HTMLElement).blur();
  }
}

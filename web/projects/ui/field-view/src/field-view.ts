import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { FieldEditorOption } from '@corelog/ui/field-editor';

/** Text shown for a field with no value. */
export const FIELD_VIEW_EMPTY = 'Sin valor';

/** Formats an ISO date or datetime as dd/MM/yyyy (and HH:mm for datetimes). */
export function formatFieldDate(value: string, withTime: boolean): string {
  const date = new Date(value.length === 10 ? `${value}T00:00:00` : value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (n: number): string => String(n).padStart(2, '0');
  const day = `${pad(date.getDate())}/${pad(date.getMonth() + 1)}/${date.getFullYear()}`;
  return withTime ? `${day} ${pad(date.getHours())}:${pad(date.getMinutes())}` : day;
}

/**
 * Read-only rendering of a custom field value by type: plain text, formatted numbers and dates,
 * option labels, a yes/no for checkboxes, a link for urls, chips for multiselect and labels.
 */
@Component({
  selector: 'app-field-view',
  templateUrl: './field-view.html',
  styleUrl: './field-view.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FieldView {
  readonly type = input.required<string>();
  readonly value = input<unknown>(null);
  readonly options = input<readonly FieldEditorOption[]>([]);
  /** Resolves a user id to a name for `user` fields. */
  readonly userName = input<(id: string) => string>((id) => id);

  protected readonly empty = computed(() => {
    const value = this.value();
    if (this.type() === 'checkbox') return false;
    return (
      value === null ||
      value === undefined ||
      value === '' ||
      (Array.isArray(value) && value.length === 0)
    );
  });

  /** Items shown as chips (multiselect, labels). */
  protected readonly chips = computed<string[]>(() => {
    const value = this.value();
    if (!Array.isArray(value)) return [];
    if (this.type() === 'multiselect') {
      return value.map((id) => this.labelOf(String(id)));
    }
    return value.map(String);
  });

  protected readonly text = computed<string>(() => {
    const value = this.value();
    switch (this.type()) {
      case 'checkbox':
        return value === true ? 'Sí' : 'No';
      case 'date':
        return typeof value === 'string' ? formatFieldDate(value, false) : '';
      case 'datetime':
        return typeof value === 'string' ? formatFieldDate(value, true) : '';
      case 'select':
        return typeof value === 'string' ? this.labelOf(value) : '';
      case 'user':
        return typeof value === 'string' ? this.userName()(value) : '';
      case 'number':
        return typeof value === 'number' ? value.toLocaleString('es') : String(value ?? '');
      default:
        return value === null || value === undefined ? '' : String(value);
    }
  });

  protected readonly href = computed(() => {
    const value = this.value();
    return this.type() === 'url' && typeof value === 'string' && /^https?:\/\//i.test(value)
      ? value
      : null;
  });

  protected readonly emptyText = FIELD_VIEW_EMPTY;

  private labelOf(id: string): string {
    return this.options().find((option) => option.id === id)?.label ?? id;
  }
}

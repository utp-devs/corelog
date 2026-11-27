import {
  ChangeDetectionStrategy,
  Component,
  computed,
  forwardRef,
  input,
  signal,
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';
import { CatalogIcon } from '@corelog/ui/catalog-icon';
import { Select } from 'primeng/select';

export interface SelectFieldOption {
  readonly value: string;
  readonly label: string;
  readonly disabled?: boolean;
  /** Catalog icon key drawn before the label (issue types and priorities). */
  readonly icon?: string | null;
  /** Catalog color key of the icon. */
  readonly color?: string | null;
}

/** Lists with more options than this get a filter box in the panel. */
export const SELECT_FILTER_THRESHOLD = 8;

/**
 * The shared single-choice select. Same height as every control (36 px), a panel as wide as the field
 * with a bounded height (about seven rows), a filter box when the list is long and a clear button.
 *
 * `size="compact"` is the variant for table cells and dense toolbars: 32 px high with small text.
 * The value is the option `value` (string) or null. Works with `[(ngModel)]` and reactive forms.
 */
@Component({
  selector: 'app-select-field',
  imports: [CatalogIcon, FormsModule, Select],
  templateUrl: './select-field.html',
  styleUrl: './select-field.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => SelectField), multi: true },
  ],
})
export class SelectField implements ControlValueAccessor {
  readonly options = input<readonly SelectFieldOption[]>([]);
  readonly placeholder = input('Seleccionar');
  readonly inputId = input<string>();
  readonly ariaLabel = input<string>();
  readonly ariaLabelledBy = input<string>();
  /** `default` is 36 px; `compact` is 32 px with small text, for tables. */
  readonly size = input<'default' | 'compact'>('default');
  /** Filter box: `auto` shows it when the list has more than 8 options. */
  readonly filter = input<'auto' | 'on' | 'off'>('auto');
  readonly showClear = input(false);
  readonly loading = input(false);
  readonly invalid = input(false);
  readonly emptyMessage = input('No hay opciones');
  /** Disabled state from the template; a form control can disable it too. */
  readonly disabled = input(false);
  /** Why the control is disabled. Shown as its tooltip so a disabled select explains itself. */
  readonly disabledReason = input<string>();

  protected readonly value = signal<string | null>(null);
  protected readonly formDisabled = signal(false);
  protected readonly isDisabled = computed(() => this.disabled() || this.formDisabled());
  protected readonly items = computed(() => [...this.options()]);
  protected readonly hasFilter = computed(() => {
    const mode = this.filter();
    return mode === 'on' || (mode === 'auto' && this.options().length > SELECT_FILTER_THRESHOLD);
  });
  protected readonly reason = computed(() => (this.isDisabled() ? this.disabledReason() : null));

  private onChangeFn: (value: string | null) => void = () => {};
  private onTouchedFn: () => void = () => {};

  protected onValueChange(next: string | null): void {
    this.value.set(next ?? null);
    this.onChangeFn(next ?? null);
  }

  protected onBlur(): void {
    this.onTouchedFn();
  }

  writeValue(value: string | null): void {
    this.value.set(value ?? null);
  }

  registerOnChange(fn: (value: string | null) => void): void {
    this.onChangeFn = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouchedFn = fn;
  }

  setDisabledState(isDisabled: boolean): void {
    this.formDisabled.set(isDisabled);
  }
}

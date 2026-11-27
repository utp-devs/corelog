import {
  ChangeDetectionStrategy,
  Component,
  computed,
  forwardRef,
  input,
  output,
  signal,
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';
import { DatePicker } from 'primeng/datepicker';

import { parseIsoDate, toIsoDate } from './date-value';

/**
 * The single date field of the product. Shows `dd/MM/yyyy` in the Spanish calendar, accepts typed dates,
 * has a clear button, a calendar button and keyboard support (arrow keys move over days, Escape closes).
 *
 * The value is an ISO `yyyy-MM-dd` string (or null), so forms and APIs never deal with time zones.
 * `min` and `max` are ISO strings too. Works with `[(value)]`, reactive forms and `ngModel`.
 */
@Component({
  selector: 'app-date-field',
  imports: [FormsModule, DatePicker],
  templateUrl: './date-field.html',
  styleUrl: './date-field.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => DateField), multi: true },
  ],
})
export class DateField implements ControlValueAccessor {
  /** Id of the native input, so a label can point at it (use the `controlId` of `app-field-row`). */
  readonly inputId = input<string>();
  /** Accessible name when there is no visible label. */
  readonly ariaLabel = input<string>();
  /** Id of the visible label element. */
  readonly ariaLabelledBy = input<string>();
  readonly placeholder = input('dd/mm/aaaa');
  /** Earliest selectable day, ISO `yyyy-MM-dd`. */
  readonly min = input<string | null>(null);
  /** Latest selectable day, ISO `yyyy-MM-dd`. */
  readonly max = input<string | null>(null);
  /** Shows the clear button when there is a value. */
  readonly showClear = input(true);
  /** Moves focus to the field when it is created (inline editors). */
  readonly autofocus = input(false);
  readonly invalid = input(false);
  /** Disabled state set from the template; a form control can disable it too. */
  readonly disabled = input(false);

  /** Emits when the calendar panel closes, with or without a pick. */
  readonly closed = output<void>();

  protected readonly date = signal<Date | null>(null);
  protected readonly formDisabled = signal(false);
  protected readonly isDisabled = computed(() => this.disabled() || this.formDisabled());
  protected readonly minDate = computed(() => parseIsoDate(this.min()));
  protected readonly maxDate = computed(() => parseIsoDate(this.max()));

  private onChangeFn: (value: string | null) => void = () => {};
  private onTouchedFn: () => void = () => {};

  protected onDateChange(next: Date | null): void {
    this.date.set(next);
    this.onChangeFn(next ? toIsoDate(next) : null);
  }

  protected onBlur(): void {
    this.onTouchedFn();
  }

  writeValue(value: string | null): void {
    this.date.set(parseIsoDate(value));
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

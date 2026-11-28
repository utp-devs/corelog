import {
  ChangeDetectionStrategy,
  Component,
  computed,
  forwardRef,
  input,
  signal,
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';

import {
  DEFAULT_WORKING_TIME,
  MAX_DURATION_SECONDS,
  WorkingTime,
  formatDuration,
  parseDuration,
} from '@corelog/core';

let nextId = 0;

/**
 * Text field for durations written as `1w 2d 3h 30m`. It shows what the text means in plain words
 * (or why it is not valid) and exposes the text as its form value. Use `parseDuration` to read it.
 */
@Component({
  selector: 'app-duration-input',
  templateUrl: './duration-input.html',
  styleUrl: './duration-input.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => DurationInput), multi: true },
  ],
})
export class DurationInput implements ControlValueAccessor {
  readonly inputId = input<string>(`duration-input-${nextId++}`);
  readonly ariaLabel = input<string>();
  readonly placeholder = input('Ej.: 2h 30m');
  readonly working = input<WorkingTime>(DEFAULT_WORKING_TIME);
  /** Longest accepted duration in seconds; 0 means no limit. */
  readonly maxSeconds = input(MAX_DURATION_SECONDS);
  /** Zero ("0m") is a valid value (estimates) or not (time spent). */
  readonly allowZero = input(false);
  readonly invalid = input(false);
  readonly disabled = input(false);

  protected readonly text = signal('');
  protected readonly formDisabled = signal(false);
  protected readonly isDisabled = computed(() => this.disabled() || this.formDisabled());
  protected readonly touched = signal(false);

  protected readonly seconds = computed(() => parseDuration(this.text(), this.working()));
  protected readonly error = computed<string | null>(() => {
    const text = this.text().trim();
    if (text === '') return null;
    const seconds = this.seconds();
    if (seconds === null) return 'Usa semanas, días, horas y minutos, por ejemplo 1d 2h 30m.';
    if (seconds === 0 && !this.allowZero()) return 'La duración debe ser mayor que cero.';
    const max = this.maxSeconds();
    if (max > 0 && seconds > max) return 'La duración es demasiado larga.';
    return null;
  });
  protected readonly hint = computed(() => {
    const seconds = this.seconds();
    return seconds !== null && this.error() === null
      ? `= ${formatDuration(seconds, this.working())}`
      : null;
  });
  protected readonly describedBy = computed(() => `${this.inputId()}-hint`);

  private onChangeFn: (value: string) => void = () => {};
  private onTouchedFn: () => void = () => {};

  protected onInput(event: Event): void {
    const value = (event.target as HTMLInputElement).value;
    this.text.set(value);
    this.onChangeFn(value);
  }

  protected onBlur(): void {
    this.touched.set(true);
    this.onTouchedFn();
  }

  writeValue(value: string | null): void {
    this.text.set(value ?? '');
  }
  registerOnChange(fn: (value: string) => void): void {
    this.onChangeFn = fn;
  }
  registerOnTouched(fn: () => void): void {
    this.onTouchedFn = fn;
  }
  setDisabledState(disabled: boolean): void {
    this.formDisabled.set(disabled);
  }
}

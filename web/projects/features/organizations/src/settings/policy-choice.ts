import { ChangeDetectionStrategy, Component, forwardRef, input, signal } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';

export interface PolicyChoiceOption {
  readonly value: string;
  readonly label: string;
  readonly hint: string;
}

let nextGroupId = 0;

/**
 * A group of radio cards for a short list of mutually exclusive policies. Each card shows the option
 * name and a line explaining what it means. Works with reactive forms and honors the disabled state.
 */
@Component({
  selector: 'app-policy-choice',
  templateUrl: './policy-choice.html',
  styleUrl: './policy-choice.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => PolicyChoice), multi: true },
  ],
})
export class PolicyChoice implements ControlValueAccessor {
  readonly options = input.required<readonly PolicyChoiceOption[]>();
  /** Accessible name of the group (the visible question). */
  readonly label = input.required<string>();

  protected readonly name = `policy-choice-${nextGroupId++}`;
  protected readonly value = signal<string | null>(null);
  protected readonly disabled = signal(false);

  private onChange: (value: string) => void = () => undefined;
  private onTouched: () => void = () => undefined;

  writeValue(value: string | null): void {
    this.value.set(value);
  }

  registerOnChange(fn: (value: string) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  setDisabledState(isDisabled: boolean): void {
    this.disabled.set(isDisabled);
  }

  protected select(value: string): void {
    this.value.set(value);
    this.onChange(value);
    this.onTouched();
  }
}

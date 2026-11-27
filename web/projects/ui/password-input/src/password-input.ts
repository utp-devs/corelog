import { ChangeDetectionStrategy, Component, forwardRef, input, signal } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';

/**
 * Password field with a keyboard-reachable visibility toggle. The toggle reflects the state in
 * aria-pressed and controls the input. Implements ControlValueAccessor so it works with reactive forms.
 */
@Component({
  selector: 'app-password-input',
  templateUrl: './password-input.html',
  styleUrl: './password-input.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => PasswordInput),
      multi: true,
    },
  ],
})
export class PasswordInput implements ControlValueAccessor {
  /** Id of the native input; the toggle controls it through aria-controls. */
  readonly inputId = input.required<string>();
  readonly autocomplete = input('current-password');
  readonly invalid = input(false);
  readonly required = input(false);
  /** Control height: 36 px (default) or 44 px for auth screens. */
  readonly size = input<'md' | 'lg'>('md');
  readonly placeholder = input<string | null>(null);
  /** Id of the element that describes the field (helper or error text). */
  readonly describedBy = input<string | null>(null);

  protected readonly visible = signal(false);
  protected readonly value = signal('');
  protected readonly disabled = signal(false);

  private onChange: (value: string) => void = () => undefined;
  private onTouched: () => void = () => undefined;

  writeValue(value: string | null): void {
    this.value.set(value ?? '');
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

  protected onInput(event: Event): void {
    const target = event.target as HTMLInputElement;
    this.value.set(target.value);
    this.onChange(target.value);
  }

  protected toggleVisibility(): void {
    this.visible.update((current) => !current);
  }

  protected blur(): void {
    this.onTouched();
  }
}

import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  effect,
  forwardRef,
  inject,
  input,
  model,
  signal,
  untracked,
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { InputText } from 'primeng/inputtext';

/**
 * Search field with icon, clear button and debounce. Notifies the final value, not every key.
 * Works with `[(value)]` and with reactive forms (ControlValueAccessor).
 */
@Component({
  selector: 'app-search-input',
  imports: [InputText],
  templateUrl: './search-input.html',
  styleUrl: './search-input.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => SearchInput),
      multi: true,
    },
  ],
})
export class SearchInput implements ControlValueAccessor {
  /** Committed value. Changes after the debounce delay or when cleared. */
  readonly value = model<string>('');
  readonly placeholder = input('Buscar');
  /** Time to wait after the last key before notifying the change. */
  readonly debounceMs = input(300);
  readonly ariaLabel = input('Buscar');

  protected readonly draft = signal(this.value());
  protected readonly disabled = signal(false);

  private lastValue = this.value();
  private timer: number | undefined;
  private onChangeFn: (value: string) => void = () => {};
  private onTouchedFn: () => void = () => {};

  constructor() {
    // Follows external changes of the value (for example, a parent clearing all filters).
    effect(() => {
      const next = this.value();
      untracked(() => {
        if (next === this.lastValue) {
          return;
        }
        this.lastValue = next;
        this.cancelTimer();
        this.draft.set(next);
      });
    });
    inject(DestroyRef).onDestroy(() => this.cancelTimer());
  }

  protected onInput(event: Event): void {
    const next = (event.target as HTMLInputElement).value;
    this.draft.set(next);
    this.cancelTimer();
    if (next === this.value()) {
      return;
    }
    this.timer = window.setTimeout(() => {
      this.timer = undefined;
      this.commit(next);
    }, this.debounceMs());
  }

  protected clear(): void {
    this.draft.set('');
    this.cancelTimer();
    this.commit('');
  }

  protected onBlur(): void {
    this.onTouchedFn();
  }

  writeValue(value: string | null): void {
    this.value.set(value ?? '');
  }

  registerOnChange(fn: (value: string) => void): void {
    this.onChangeFn = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouchedFn = fn;
  }

  setDisabledState(isDisabled: boolean): void {
    this.disabled.set(isDisabled);
  }

  private commit(next: string): void {
    this.lastValue = next;
    this.value.set(next);
    this.onChangeFn(next);
  }

  private cancelTimer(): void {
    if (this.timer !== undefined) {
      window.clearTimeout(this.timer);
      this.timer = undefined;
    }
  }
}

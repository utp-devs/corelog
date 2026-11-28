import {
  ChangeDetectionStrategy,
  Component,
  computed,
  forwardRef,
  input,
  output,
  signal,
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';

let nextConsentId = 0;

/**
 * One consent checkbox whose label holds a link to a legal document: `before`, the link text and
 * `after`. The link is a real button styled as a link that emits `read`; it never toggles the box.
 * Works with reactive forms as a boolean control.
 */
@Component({
  selector: 'app-legal-consent-checkbox',
  templateUrl: './legal-consent-checkbox.html',
  styleUrl: './legal-consent-checkbox.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => LegalConsentCheckbox),
      multi: true,
    },
  ],
})
export class LegalConsentCheckbox implements ControlValueAccessor {
  /** Text before the link, for example "He leído y acepto la". */
  readonly before = input('');
  readonly linkLabel = input('');
  readonly after = input('');
  readonly required = input(false);
  /** Message shown under the box; makes the box invalid. */
  readonly error = input<string | null>(null);
  /** Accessible name of the link when it needs more than its text. */
  readonly linkDescription = input<string | null>(null);

  /** Emits when the person asks to read the document; the button that was pressed is passed along. */
  readonly read = output<void>();

  protected readonly id = `cl-consent-${nextConsentId++}`;
  /** The box is named by its whole sentence: the text around the link and the link itself. */
  protected readonly labelledBy = computed(
    () => `${this.id}-before ${this.id}-link${this.after() ? ` ${this.id}-after` : ''}`,
  );
  protected readonly checked = signal(false);
  protected readonly disabled = signal(false);

  private onChange: (value: boolean) => void = () => undefined;
  private onTouched: () => void = () => undefined;

  writeValue(value: boolean | null): void {
    this.checked.set(Boolean(value));
  }

  registerOnChange(fn: (value: boolean) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  setDisabledState(isDisabled: boolean): void {
    this.disabled.set(isDisabled);
  }

  protected onToggle(event: Event): void {
    const value = (event.target as HTMLInputElement).checked;
    this.checked.set(value);
    this.onChange(value);
    this.onTouched();
  }

  protected openDocument(event: Event): void {
    event.preventDefault();
    event.stopPropagation();
    this.read.emit();
  }
}

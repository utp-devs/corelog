import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  forwardRef,
  input,
  signal,
  viewChild,
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';

import { EMAIL_PATTERN } from './invite-form';

/** Characters that end an address while typing or pasting. */
const SEPARATORS = /[\s,;]+/;

interface Chip {
  readonly email: string;
  readonly valid: boolean;
}

/**
 * Input for several email addresses. Each completed address (after a comma, semicolon, space, Enter or
 * when the field loses focus) becomes a chip with a remove button; addresses that are not valid are
 * marked. The form value is one text with every address (chips and the one being typed) separated by
 * commas, so the form rules and the API items keep working on plain text.
 */
@Component({
  selector: 'app-invite-emails-input',
  templateUrl: './invite-emails-input.html',
  styleUrl: './invite-emails-input.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => InviteEmailsInput),
      multi: true,
    },
  ],
})
export class InviteEmailsInput implements ControlValueAccessor {
  private readonly field = viewChild.required<ElementRef<HTMLInputElement>>('field');

  /** Id of the native input, so the label of the field row points at it. */
  readonly inputId = input.required<string>();
  readonly placeholder = input('ana@empresa.com');
  readonly invalid = input(false);
  /** Id of the text that describes the field (counter or error). */
  readonly describedBy = input<string | null>(null);

  private readonly addresses = signal<string[]>([]);
  protected readonly draft = signal('');
  protected readonly disabled = signal(false);
  protected readonly chips = computed<Chip[]>(() =>
    this.addresses().map((email) => ({ email, valid: EMAIL_PATTERN.test(email) })),
  );

  private onChange: (value: string) => void = () => undefined;
  private onTouched: () => void = () => undefined;

  writeValue(value: string | null): void {
    this.addresses.set(splitAddresses(value ?? ''));
    this.draft.set('');
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

  protected focusField(): void {
    this.field().nativeElement.focus();
  }

  protected onInput(event: Event): void {
    const target = event.target as HTMLInputElement;
    const parts = target.value.split(SEPARATORS);
    // Everything before the last separator is complete; the rest is still being typed.
    const rest = parts.pop() ?? '';
    const completed = parts.filter((part) => part !== '');
    if (completed.length > 0) this.addresses.update((list) => [...list, ...completed]);
    this.draft.set(rest);
    target.value = rest;
    this.emit();
  }

  protected onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Enter') {
      // Enter completes the address instead of submitting the form.
      event.preventDefault();
      this.commitDraft();
    } else if (event.key === 'Backspace' && this.draft() === '' && this.addresses().length > 0) {
      this.addresses.update((list) => list.slice(0, -1));
      this.emit();
    }
  }

  protected onBlur(): void {
    this.commitDraft();
    this.onTouched();
  }

  protected remove(index: number): void {
    this.addresses.update((list) => list.filter((_, position) => position !== index));
    this.emit();
    this.focusField();
  }

  private commitDraft(): void {
    const pending = this.draft().trim();
    if (pending === '') return;
    this.addresses.update((list) => [...list, pending]);
    this.draft.set('');
    this.field().nativeElement.value = '';
    this.emit();
  }

  private emit(): void {
    const pending = this.draft().trim();
    this.onChange([...this.addresses(), ...(pending ? [pending] : [])].join(', '));
  }
}

function splitAddresses(text: string): string[] {
  return text.split(SEPARATORS).filter((part) => part !== '');
}

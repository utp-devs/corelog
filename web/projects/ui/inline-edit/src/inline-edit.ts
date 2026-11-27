import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  forwardRef,
  inject,
  input,
  model,
  signal,
  viewChild,
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';
import { InputText } from 'primeng/inputtext';
import { Select } from 'primeng/select';

import { DateField, parseIsoDate } from '@corelog/ui/date-field';

export type InlineEditType = 'text' | 'select' | 'date' | 'user';
/**
 * How the closed field looks. `text` reads as plain text (pencil and border appear on hover); `field` looks like a
 * select (border and chevron always visible); `lozenge` is a tinted pill with a chevron, for status and priority.
 */
export type InlineEditAppearance = 'text' | 'field' | 'lozenge';
/** Color of the lozenge appearance, from the category tokens. */
export type InlineEditTone = 'neutral' | 'progress' | 'done' | 'danger' | 'warning';
export interface InlineEditOption {
  readonly value: string;
  readonly label: string;
}

export const INLINE_EDIT_SAVE_ERROR =
  'No se pudo guardar. Revisa tu conexión e inténtalo de nuevo.';
const EMPTY_TEXT = 'Sin valor';

/** `dd/MM/yyyy` text of an ISO date; the raw text when it is not a valid ISO date. */
function formatDisplayDate(iso: string): string {
  const date = parseIsoDate(iso);
  if (!date) {
    return iso;
  }
  const day = String(date.getDate()).padStart(2, '0');
  const month = String(date.getMonth() + 1).padStart(2, '0');
  return `${day}/${month}/${date.getFullYear()}`;
}

/** Message of the error thrown by the save, or the generic message when it carries none. */
function saveErrorMessage(error: unknown): string {
  const message = (error as { readonly message?: unknown } | null)?.message;
  return typeof message === 'string' && message.trim() !== '' ? message : INLINE_EDIT_SAVE_ERROR;
}

/**
 * Inline editable field. Enter saves, Escape cancels, leaving the field saves.
 * While saving it shows a pending indicator; if saving fails it keeps the previous value and shows a message.
 * Works with `[(value)]` and with reactive forms (ControlValueAccessor).
 */
@Component({
  selector: 'app-inline-edit',
  imports: [FormsModule, InputText, Select, DateField],
  templateUrl: './inline-edit.html',
  styleUrl: './inline-edit.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => InlineEdit),
      multi: true,
    },
  ],
})
export class InlineEdit implements ControlValueAccessor {
  /** Field name, for example "Título". Used as the accessible name of the editor. */
  readonly label = input.required<string>();
  readonly type = input<InlineEditType>('text');
  /** Current value; `null` or an empty string means no value. Updated when a save succeeds. */
  readonly value = model<string | null>(null);
  /** Options for select and user (user: value = id, label = name). */
  readonly options = input<InlineEditOption[]>([]);
  readonly placeholder = input(EMPTY_TEXT);
  /** Look of the closed field. Use `lozenge` or `field` for values that are chosen from a list (status, priority). */
  readonly appearance = input<InlineEditAppearance>('text');
  /** Color of the `lozenge` appearance. */
  readonly tone = input<InlineEditTone>('neutral');
  /** Locks the field from the outside (for example a read-only project); the value stays readable. */
  readonly locked = input(false);
  /** Why the field cannot be edited. Shown as its tooltip while it is disabled. */
  readonly disabledReason = input<string>();
  /** Saves the new value. If it throws or rejects, its message is shown (or a generic one) and the previous value is kept. */
  readonly onSave = input<(next: string | null) => void | Promise<void>>(() => {});

  protected readonly editing = signal(false);
  protected readonly draft = signal('');
  protected readonly disabled = signal(false);
  protected readonly error = signal<string | null>(null);
  private readonly pendingValue = signal<string | null | undefined>(undefined);
  protected readonly editorInput = viewChild<ElementRef<HTMLInputElement>>('editorInput');

  // True only between startEdit and the first commit or cancel, so that a blur that follows
  // Enter or Escape (or the removal of the editor) does not save a second time.
  private active = false;
  private readonly injector = inject(Injector);
  private onChangeFn: (value: string | null) => void = () => {};
  private onTouchedFn: () => void = () => {};

  protected readonly current = computed(() => (this.value() === '' ? null : this.value()));
  protected readonly pending = computed(() => this.pendingValue() !== undefined);
  protected readonly shown = computed(() =>
    this.pending() ? (this.pendingValue() ?? null) : this.current(),
  );
  protected readonly hasChevron = computed(
    () => this.appearance() !== 'text' || this.isSelectType() || this.type() === 'date',
  );
  protected readonly isEmpty = computed(() => this.shown() === null || this.shown() === undefined);
  protected readonly shownText = computed(() => {
    const shown = this.shown();
    if (shown === null || shown === undefined) {
      return this.placeholder();
    }
    if (this.type() === 'date') {
      return formatDisplayDate(shown);
    }
    return this.options().find((option) => option.value === shown)?.label ?? shown;
  });
  protected readonly isSelectType = computed(
    () => this.type() === 'select' || this.type() === 'user',
  );

  protected startEdit(): void {
    this.active = true;
    this.draft.set(this.current() ?? '');
    this.error.set(null);
    this.editing.set(true);
    afterNextRender(() => this.editorInput()?.nativeElement.focus(), { injector: this.injector });
  }

  protected onEditorKeyDown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.preventDefault();
      this.cancelEdit();
    } else if (event.key === 'Enter' && !this.isSelectType()) {
      event.preventDefault();
      this.commit(this.draft());
    }
  }

  protected onEditorFocusOut(event: FocusEvent): void {
    if (this.type() === 'date') {
      // The calendar panel lives outside the editor: the date field reports when it closes instead.
      return;
    }
    const editor = event.currentTarget as HTMLElement;
    if (!editor.contains(event.relatedTarget as Node | null)) {
      this.commit(this.draft());
    }
  }

  protected onDraftInput(event: Event): void {
    this.draft.set((event.target as HTMLInputElement).value);
  }

  protected onDateChange(next: string | null): void {
    this.draft.set(next ?? '');
    this.commit(next ?? '');
  }

  /** The calendar closed without a pick (outside click or Escape): the edit ends like a blur. */
  protected onDateClosed(): void {
    this.commit(this.draft());
  }

  protected onSelectChange(next: string | null): void {
    const value = next ?? '';
    this.draft.set(value);
    this.commit(value);
  }

  writeValue(value: string | null): void {
    this.value.set(value);
  }

  registerOnChange(fn: (value: string | null) => void): void {
    this.onChangeFn = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouchedFn = fn;
  }

  setDisabledState(isDisabled: boolean): void {
    this.disabled.set(isDisabled);
  }

  private cancelEdit(): void {
    this.active = false;
    this.draft.set(this.current() ?? '');
    this.editing.set(false);
    this.onTouchedFn();
  }

  private commit(raw: string): void {
    if (!this.active) {
      return;
    }
    this.active = false;
    this.editing.set(false);
    this.onTouchedFn();
    const next = raw === '' ? null : raw;
    if (next === this.current()) {
      return;
    }
    void this.save(next);
  }

  private async save(next: string | null): Promise<void> {
    this.pendingValue.set(next);
    this.error.set(null);
    try {
      await this.onSave()(next);
      this.value.set(next);
      this.onChangeFn(next);
    } catch (error) {
      this.error.set(saveErrorMessage(error));
    } finally {
      this.pendingValue.set(undefined);
    }
  }
}

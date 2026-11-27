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

export type InlineEditType = 'text' | 'select' | 'date' | 'user';
export interface InlineEditOption {
  readonly value: string;
  readonly label: string;
}

export const INLINE_EDIT_SAVE_ERROR =
  'No se pudo guardar. Revisa tu conexión e inténtalo de nuevo.';
const EMPTY_TEXT = 'Sin valor';

/**
 * Inline editable field. Enter saves, Escape cancels, leaving the field saves.
 * While saving it shows a pending indicator; if saving fails it keeps the previous value and shows a message.
 * Works with `[(value)]` and with reactive forms (ControlValueAccessor).
 */
@Component({
  selector: 'app-inline-edit',
  imports: [FormsModule, InputText, Select],
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
  /** Saves the new value. If it throws or rejects, the error message is shown and the previous value is kept. */
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
  protected readonly isEmpty = computed(() => this.shown() === null || this.shown() === undefined);
  protected readonly shownText = computed(() => {
    const shown = this.shown();
    if (shown === null || shown === undefined) {
      return this.placeholder();
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
    const editor = event.currentTarget as HTMLElement;
    if (!editor.contains(event.relatedTarget as Node | null)) {
      this.commit(this.draft());
    }
  }

  protected onDraftInput(event: Event): void {
    this.draft.set((event.target as HTMLInputElement).value);
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
    } catch {
      this.error.set(INLINE_EDIT_SAVE_ERROR);
    } finally {
      this.pendingValue.set(undefined);
    }
  }
}

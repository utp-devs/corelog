import {
  CdkConnectedOverlay,
  CdkOverlayOrigin,
  Overlay,
  type ConnectedPosition,
} from '@angular/cdk/overlay';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  OnInit,
  afterRenderEffect,
  computed,
  effect,
  forwardRef,
  inject,
  input,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import {
  applyToolbarAction,
  insertAtSelection,
  mentionToken,
  type TextEdit,
  type ToolbarAction,
} from '@corelog/core';
import { Avatar } from '@corelog/ui/avatar';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { AppTooltip } from '@corelog/ui/tooltip';

import { clearDraft, readDraft, writeDraft } from './draft';
import { detectTrigger, type Trigger, type TriggerKind } from './trigger';
import type { IssueLite, UserLite } from './types';

export type { IssueLite, UserLite } from './types';

interface PickerItem {
  readonly id: string;
  /** User id and photo of a person suggestion; absent for issue suggestions. */
  readonly userId?: string;
  readonly avatarUrl?: string | null;
  readonly label: string;
  readonly detail?: string;
  readonly insert: string;
}

interface PopupState extends Trigger {
  readonly active: number;
}

interface PickerResults {
  readonly key: string;
  readonly items: PickerItem[];
}

/** Commands of the toolbar that are not text formatting. */
type EditorCommand = 'mention' | 'issue' | 'image';

interface FormatButton {
  readonly action: ToolbarAction | EditorCommand;
  readonly label: string;
  readonly icon: string;
}

interface ToolGroup {
  readonly name: string;
  readonly buttons: readonly FormatButton[];
}

const ACCEPTED_IMAGE_TYPES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp'];
const SEARCH_DEBOUNCE_MS = 150;
const DRAFT_DEBOUNCE_MS = 400;

/** Toolbar groups, separated by a divider: headings | text | lists | blocks | inserts. One icon set (PrimeIcons). */
const TOOL_GROUPS: readonly ToolGroup[] = [
  {
    name: 'Encabezados',
    buttons: [{ action: 'heading', label: 'Encabezado', icon: 'pi pi-heading' }],
  },
  {
    name: 'Texto',
    buttons: [
      { action: 'bold', label: 'Negrita', icon: 'pi pi-bold' },
      { action: 'italic', label: 'Cursiva', icon: 'pi pi-italic' },
      { action: 'strikethrough', label: 'Tachado', icon: 'pi pi-strikethrough' },
    ],
  },
  {
    name: 'Listas',
    buttons: [
      { action: 'bulletList', label: 'Lista con viñetas', icon: 'pi pi-list' },
      { action: 'numberedList', label: 'Lista numerada', icon: 'pi pi-list-ol' },
      { action: 'taskList', label: 'Lista de tareas', icon: 'pi pi-list-check' },
    ],
  },
  {
    name: 'Bloques',
    buttons: [
      { action: 'quote', label: 'Cita', icon: 'pi pi-block-quote' },
      { action: 'code', label: 'Código en línea', icon: 'pi pi-code' },
      { action: 'codeBlock', label: 'Bloque de código', icon: 'pi pi-file' },
      { action: 'link', label: 'Enlace', icon: 'pi pi-link' },
      { action: 'table', label: 'Tabla', icon: 'pi pi-table' },
    ],
  },
  {
    name: 'Insertar',
    buttons: [
      { action: 'mention', label: 'Mencionar persona', icon: 'pi pi-at' },
      { action: 'issue', label: 'Enlazar incidencia', icon: 'pi pi-hashtag' },
      { action: 'image', label: 'Subir imagen', icon: 'pi pi-image' },
    ],
  },
];

let nextEditorId = 0;

/**
 * Picker under the first line of the text, or above the text when there is no room below. The pixel
 * offsets match the inset the list always had inside the editor.
 */
const LIST_POSITIONS: ConnectedPosition[] = [
  {
    originX: 'start',
    originY: 'top',
    overlayX: 'start',
    overlayY: 'top',
    offsetX: 12,
    offsetY: 32,
  },
  { originX: 'start', originY: 'top', overlayX: 'start', overlayY: 'bottom', offsetX: 12 },
];

/**
 * Markdown editor with a formatting toolbar, write and preview tabs, @ and # pickers,
 * image upload (paste, drop or button), draft autosave and Ctrl+Enter submit.
 * Bound with ngModel or a reactive form control.
 */
@Component({
  selector: 'app-rich-text-editor',
  imports: [RichTextView, Avatar, AppTooltip, CdkConnectedOverlay, CdkOverlayOrigin],
  templateUrl: './rich-text-editor.html',
  styleUrl: './rich-text-editor.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [
    { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => RichTextEditor), multi: true },
  ],
  host: { class: 'rich-text-editor' },
})
export class RichTextEditor implements ControlValueAccessor, OnInit {
  readonly ariaLabel = input.required<string>();
  readonly placeholder = input('');
  readonly minRows = input(4);
  readonly maxRows = input(16);
  /** Users for the @ picker. Results are shown as a listbox. */
  readonly searchUsers = input<(query: string) => Promise<UserLite[]>>();
  /** Issues for the # and key picker. */
  readonly searchIssues = input<(query: string) => Promise<IssueLite[]>>();
  /** Uploads an image and returns its public URL. Enables paste, drop and the image button. */
  readonly uploadImage = input<(file: File) => Promise<{ url: string }>>();
  /** Enables draft autosave in localStorage under `corelog.draft.<draftKey>`. */
  readonly draftKey = input<string>();
  /** Enables the Ctrl+Enter submit shortcut and its hint. */
  readonly submittable = input(false);
  /** Blocks every edit and action (for example while the owning form is submitting). */
  readonly disabled = input(false);
  /** Emitted on Ctrl+Enter (or Cmd+Enter) when submittable. The saved draft is cleared first. */
  readonly submitted = output<void>();

  private readonly destroyRef = inject(DestroyRef);
  private readonly textarea = viewChild<ElementRef<HTMLTextAreaElement>>('textarea');
  private readonly fileInput = viewChild<ElementRef<HTMLInputElement>>('fileInput');

  /** Toolbar groups; the image button only exists when uploads are enabled. */
  protected readonly toolGroups = computed(() =>
    TOOL_GROUPS.map((group) => ({
      ...group,
      buttons: group.buttons.filter((button) => button.action !== 'image' || !!this.uploadImage()),
    })).filter((group) => group.buttons.length > 0),
  );
  protected readonly acceptedTypes = ACCEPTED_IMAGE_TYPES.join(',');
  protected readonly id = `rich-text-editor-${nextEditorId++}`;
  protected readonly listboxId = `${this.id}-listbox`;
  protected readonly listPositions = LIST_POSITIONS;
  /** The list closes when the page scrolls under it, so it never floats away from the text. */
  protected readonly scrollStrategy = inject(Overlay).scrollStrategies.close();

  protected readonly value = signal('');
  /** Disabled state written by the form control (ControlValueAccessor). */
  private readonly formDisabled = signal(false);
  protected readonly isDisabled = computed(() => this.disabled() || this.formDisabled());
  protected readonly mode = signal<'write' | 'preview'>('write');
  protected readonly popup = signal<PopupState | null>(null);
  protected readonly results = signal<PickerResults | null>(null);
  protected readonly uploading = signal(false);
  protected readonly uploadError = signal<string | null>(null);
  protected readonly dragging = signal(false);

  protected readonly popupKey = computed(() => {
    const p = this.popup();
    return p ? `${p.kind}:${p.query}` : null;
  });
  protected readonly currentItems = computed<PickerItem[]>(() => {
    const key = this.popupKey();
    const current = this.results();
    return key !== null && current?.key === key ? current.items : [];
  });
  protected readonly pickerLoading = computed(() => {
    const key = this.popupKey();
    return key !== null && this.results()?.key !== key;
  });
  protected readonly showListbox = computed(
    () => this.popup() !== null && (this.pickerLoading() || this.currentItems().length > 0),
  );
  protected readonly activeIndex = computed(() => this.popup()?.active ?? -1);
  protected readonly activeOptionId = computed(() => {
    const index = this.activeIndex();
    return this.showListbox() && index >= 0 && this.currentItems()[index]
      ? this.optionId(index)
      : null;
  });
  protected readonly pickerLabel = computed(() =>
    this.popup()?.kind === 'issue' ? 'Incidencias' : 'Personas',
  );

  private onChangeFn: (value: string) => void = () => undefined;
  private onTouchedFn: () => void = () => undefined;
  private pendingSelection: { start: number; end: number } | null = null;
  private dirty = false;
  private restored = false;
  private awaitingDraftEcho = false;
  private draftTimer: ReturnType<typeof setTimeout> | undefined;
  /** Height the user dragged the resize handle to; the autosize never goes below it. */
  private manualHeight = 0;
  private autoHeight = 0;

  constructor() {
    // Debounced search for the open picker, keyed by trigger kind and query.
    effect((onCleanup) => {
      const key = this.popupKey();
      if (key === null) return;
      const p = untracked(() => this.popup());
      if (!p) return;
      const timer = setTimeout(() => {
        void this.runSearch(p.kind, p.query, key);
      }, SEARCH_DEBOUNCE_MS);
      onCleanup(() => clearTimeout(timer));
    });

    // Autosize between minRows and maxRows, and restore the caret after toolbar or picker edits.
    afterRenderEffect(() => {
      this.value();
      this.minRows();
      this.maxRows();
      this.mode();
      const el = this.textarea()?.nativeElement;
      if (!el) return;
      this.resize(el);
      const pending = this.pendingSelection;
      if (pending) {
        this.pendingSelection = null;
        el.setSelectionRange(pending.start, pending.end);
      }
    });

    this.destroyRef.onDestroy(() => clearTimeout(this.draftTimer));
  }

  ngOnInit(): void {
    this.restoreDraft();
  }

  writeValue(value: string | null): void {
    // The form writes its initial empty value after the draft was restored: keep the draft.
    if (this.awaitingDraftEcho) {
      this.awaitingDraftEcho = false;
      if (!value) return;
    }
    this.value.set(value ?? '');
    this.restoreDraft();
  }

  registerOnChange(fn: (value: string) => void): void {
    this.onChangeFn = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouchedFn = fn;
  }

  setDisabledState(isDisabled: boolean): void {
    this.formDisabled.set(isDisabled);
  }

  protected setMode(mode: 'write' | 'preview'): void {
    this.mode.set(mode);
  }

  protected onInput(event: Event): void {
    const el = event.target as HTMLTextAreaElement;
    this.emit(el.value);
    this.openPickerFrom(el.value, el.selectionStart);
  }

  /** After a drag on the resize handle the new height becomes the floor of the autosize. */
  protected onResizeEnd(event: Event): void {
    const el = event.target as HTMLTextAreaElement;
    if (Math.abs(el.offsetHeight - this.autoHeight) > 2) {
      this.manualHeight = el.offsetHeight;
    }
  }

  /** The overlay detached while the list was still wanted: the page scrolled, so the picker closes. */
  protected onListDetach(): void {
    if (this.showListbox()) this.popup.set(null);
  }

  protected onBlur(): void {
    this.popup.set(null);
    this.onTouchedFn();
  }

  protected runTool(action: ToolbarAction | EditorCommand): void {
    if (action === 'mention') {
      this.insertTrigger('@');
    } else if (action === 'issue') {
      this.insertTrigger('#');
    } else if (action === 'image') {
      this.openFilePicker();
    } else {
      this.runToolbar(action);
    }
  }

  protected toolDisabled(action: ToolbarAction | EditorCommand): boolean {
    return this.isDisabled() || (action === 'image' && this.uploading());
  }

  protected runToolbar(action: ToolbarAction): void {
    const el = this.textarea()?.nativeElement;
    if (!el) return;
    this.applyEdit(applyToolbarAction(action, this.value(), el.selectionStart, el.selectionEnd));
  }

  protected pickItem(item: PickerItem): void {
    const p = this.popup();
    if (!p) return;
    const insertion = `${item.insert} `;
    const current = this.value();
    const next = current.slice(0, p.start) + insertion + current.slice(p.end);
    const caret = p.start + insertion.length;
    this.popup.set(null);
    this.applyEdit({ value: next, selectionStart: caret, selectionEnd: caret });
  }

  protected onKeyDown(event: KeyboardEvent): void {
    const p = this.popup();
    if (p) {
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopPropagation();
        this.popup.set(null);
        return;
      }
      const items = this.currentItems();
      if (items.length > 0) {
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
          event.preventDefault();
          const step = event.key === 'ArrowDown' ? 1 : -1;
          const count = items.length;
          this.popup.set({ ...p, active: (p.active + step + count) % count });
          return;
        }
        const plainEnter = event.key === 'Enter' && !event.ctrlKey && !event.metaKey;
        if (plainEnter || event.key === 'Tab') {
          event.preventDefault();
          this.pickItem(items[Math.min(p.active, items.length - 1)]);
          return;
        }
      }
    }
    if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
      event.preventDefault();
      if (this.submittable() && !this.isDisabled()) {
        this.submit();
      }
    }
  }

  protected onPaste(event: ClipboardEvent): void {
    const files = Array.from(event.clipboardData?.files ?? []);
    if (!this.uploadImage() || files.length === 0) return;
    event.preventDefault();
    void this.uploadFiles(files);
  }

  protected onDragOver(event: DragEvent): void {
    if (!this.uploadImage() || !event.dataTransfer?.types.includes('Files')) return;
    event.preventDefault();
    this.dragging.set(true);
  }

  protected onDragLeave(): void {
    this.dragging.set(false);
  }

  protected onDrop(event: DragEvent): void {
    this.dragging.set(false);
    const files = Array.from(event.dataTransfer?.files ?? []);
    if (!this.uploadImage() || files.length === 0) return;
    event.preventDefault();
    void this.uploadFiles(files);
  }

  protected openFilePicker(): void {
    this.fileInput()?.nativeElement.click();
  }

  protected onFileChange(event: Event): void {
    const input = event.target as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    input.value = '';
    if (files.length > 0) void this.uploadFiles(files);
  }

  protected optionId(index: number): string {
    return `${this.id}-option-${index}`;
  }

  private submit(): void {
    const key = this.draftKey();
    if (key) clearDraft(key);
    clearTimeout(this.draftTimer);
    this.dirty = false;
    this.submitted.emit();
  }

  /** Restores the draft once, only when the field starts empty. */
  private restoreDraft(): void {
    if (this.restored) return;
    const key = this.draftKey();
    if (key === undefined) return;
    if (this.value() !== '') {
      this.restored = true;
      return;
    }
    // Runs again on a later empty write (forms may write their initial value after init).
    const draft = readDraft(key);
    if (!draft) return;
    this.restored = true;
    this.awaitingDraftEcho = true;
    this.value.set(draft);
    this.onChangeFn(draft);
  }

  private emit(next: string): void {
    this.dirty = true;
    this.value.set(next);
    this.onChangeFn(next);
    this.scheduleDraftSave(next);
  }

  /** Debounced save after user edits; an empty value clears the draft. */
  private scheduleDraftSave(next: string): void {
    const key = this.draftKey();
    if (!key || !this.dirty) return;
    clearTimeout(this.draftTimer);
    this.draftTimer = setTimeout(() => {
      if (next.trim()) writeDraft(key, next);
      else clearDraft(key);
    }, DRAFT_DEBOUNCE_MS);
  }

  private applyEdit(edit: TextEdit): void {
    this.pendingSelection = { start: edit.selectionStart, end: edit.selectionEnd };
    this.emit(edit.value);
    this.textarea()?.nativeElement.focus();
  }

  private openPickerFrom(text: string, caret: number): void {
    const trigger = detectTrigger(text.slice(0, caret));
    this.popup.set(trigger ? { ...trigger, active: 0 } : null);
  }

  private insertTrigger(char: '@' | '#'): void {
    const el = this.textarea()?.nativeElement;
    if (!el) return;
    const edit = insertAtSelection(this.value(), el.selectionStart, el.selectionEnd, char, '');
    this.applyEdit(edit);
    this.openPickerFrom(edit.value, edit.selectionStart);
  }

  private async runSearch(kind: TriggerKind, query: string, key: string): Promise<void> {
    try {
      let items: PickerItem[] = [];
      const users = this.searchUsers();
      const issues = this.searchIssues();
      if (kind === 'user' && users) {
        const found = await users(query);
        items = found.map((u) => ({
          id: u.id,
          userId: u.id,
          avatarUrl: u.avatarUrl,
          label: u.name,
          detail: u.email,
          insert: mentionToken({ id: u.id, name: u.name }),
        }));
      } else if (kind === 'issue' && issues) {
        const found = await issues(query);
        items = found.map((i) => ({ id: i.key, label: i.key, detail: i.summary, insert: i.key }));
      }
      this.results.set({ key, items });
    } catch {
      this.results.set({ key, items: [] });
    }
  }

  private async uploadFiles(files: File[]): Promise<void> {
    const upload = this.uploadImage();
    if (!upload) return;
    const images = files.filter((file) => ACCEPTED_IMAGE_TYPES.includes(file.type));
    if (images.length === 0) {
      this.uploadError.set('Formato no admitido. Usa PNG, JPG, GIF o WEBP.');
      return;
    }
    const el = this.textarea()?.nativeElement;
    const caret = el?.selectionEnd ?? this.value().length;
    this.uploadError.set(null);
    this.uploading.set(true);
    try {
      let inserted = '';
      for (const file of images) {
        const { url } = await upload(file);
        inserted += `![](${url})\n`;
      }
      const current = this.textarea()?.nativeElement.value ?? this.value();
      const next = current.slice(0, caret) + inserted + current.slice(caret);
      const pos = caret + inserted.length;
      this.applyEdit({ value: next, selectionStart: pos, selectionEnd: pos });
    } catch {
      this.uploadError.set('No se pudo subir la imagen. Inténtalo de nuevo.');
    } finally {
      this.uploading.set(false);
    }
  }

  /** Runtime measurement of the textarea height between the row limits (no template styles). */
  private resize(el: HTMLTextAreaElement): void {
    el.style.height = 'auto';
    const computed = window.getComputedStyle(el);
    const lineHeight = parseFloat(computed.lineHeight) || 20;
    const padding =
      (parseFloat(computed.paddingTop) || 0) + (parseFloat(computed.paddingBottom) || 0);
    const min = Math.max(lineHeight * this.minRows() + padding, this.manualHeight);
    const max = Math.max(lineHeight * this.maxRows() + padding, this.manualHeight);
    const next = Math.min(Math.max(el.scrollHeight, min), max);
    this.autoHeight = next;
    el.style.height = `${next}px`;
    el.style.overflowY = el.scrollHeight > max ? 'auto' : 'hidden';
  }
}

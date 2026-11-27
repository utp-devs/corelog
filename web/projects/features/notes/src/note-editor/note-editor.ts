import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import {
  NOTE_BODY_MAX,
  NOTE_TITLE_MAX,
  NotesStore,
  noteHeadline,
  type Note,
  type NoteColor,
} from '@corelog/core';
import { RichTextEditor } from '@corelog/ui/rich-text-editor';
import { SidePanel } from '@corelog/ui/side-panel';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';

import { NoteColorPalette } from '../note-color-palette/note-color-palette';
import { NoteIssueLink } from '../note-issue-link/note-issue-link';
import { NoteTagsInput } from '../note-tags-input/note-tags-input';

/**
 * Editor of one note in a side panel. Every change is saved on its own after a short pause
 * (the indicator reads "Guardando…", "Guardado" or an error); closing the panel sends what is pending.
 */
@Component({
  selector: 'app-note-editor',
  imports: [
    Button,
    InputText,
    NoteColorPalette,
    NoteIssueLink,
    NoteTagsInput,
    ReactiveFormsModule,
    RichTextEditor,
    SidePanel,
  ],
  templateUrl: './note-editor.html',
  styleUrl: './note-editor.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NoteEditor {
  private readonly store = inject(NotesStore);

  /** The note being edited, or null when the panel is closed. */
  readonly note = input<Note | null>(null);
  /** Existing tags offered as quick picks. */
  readonly suggestions = input<readonly string[]>([]);

  readonly closed = output<void>();
  readonly deleted = output<Note>();

  protected readonly titleMax = NOTE_TITLE_MAX;
  protected readonly bodyMax = NOTE_BODY_MAX;
  protected readonly color = signal<NoteColor>('yellow');
  protected readonly tags = signal<readonly string[]>([]);
  protected readonly issueId = signal<string | null>(null);
  protected readonly pinned = signal(false);
  protected readonly saveState = this.store.saveState;
  protected readonly heading = computed(() => {
    const current = this.note();
    return current ? noteHeadline(current) : 'Nota';
  });

  protected readonly form = new FormGroup({
    title: new FormControl('', {
      nonNullable: true,
      validators: [Validators.maxLength(NOTE_TITLE_MAX)],
    }),
    body: new FormControl('', {
      nonNullable: true,
      validators: [Validators.maxLength(NOTE_BODY_MAX)],
    }),
  });

  /** Id of the note whose values are in the form. */
  private loadedId: string | null = null;

  constructor() {
    // Loads the form once per opened note; later store updates never overwrite what is being typed.
    effect(() => {
      const current = this.note();
      untracked(() => this.load(current));
    });

    this.form.controls.title.valueChanges.pipe(takeUntilDestroyed()).subscribe((title) => {
      if (this.form.controls.title.valid) this.save({ title: title.trim() });
    });
    this.form.controls.body.valueChanges.pipe(takeUntilDestroyed()).subscribe((body) => {
      if (this.form.controls.body.valid) this.save({ body });
    });
  }

  protected setColor(color: NoteColor): void {
    this.color.set(color);
    this.save({ color });
  }

  protected setTags(tags: readonly string[]): void {
    this.tags.set(tags);
    this.save({ tags: [...tags] });
  }

  protected setIssue(issueId: string | null): void {
    this.issueId.set(issueId);
    this.save({ issueId });
  }

  protected togglePin(): void {
    const current = this.note();
    if (!current) return;
    this.store.togglePin({ ...current, pinned: this.pinned() });
    this.pinned.update((value) => !value);
  }

  protected close(): void {
    this.store.flush();
    this.closed.emit();
  }

  protected remove(): void {
    const current = this.note();
    if (current) this.deleted.emit(current);
  }

  private load(note: Note | null): void {
    if (!note) {
      this.loadedId = null;
      return;
    }
    if (note.id === this.loadedId) return;
    this.loadedId = note.id;
    this.form.reset({ title: note.title, body: note.body }, { emitEvent: false });
    this.color.set(note.color);
    this.tags.set(note.tags);
    this.issueId.set(note.issueId);
    this.pinned.set(note.pinned);
    this.store.resetSaveState();
  }

  private save(change: Parameters<NotesStore['autosave']>[1]): void {
    if (this.loadedId) this.store.autosave(this.loadedId, change);
  }
}

import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import {
  DEFAULT_NOTE_COLOR,
  NOTE_BODY_MAX,
  NOTE_TITLE_MAX,
  NotesStore,
  NotificationService,
  type Note,
  type NoteColor,
} from '@corelog/core';
import { RichTextEditor } from '@corelog/ui/rich-text-editor';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';

import { NoteColorPalette } from '../note-color-palette/note-color-palette';
import { NoteTagsInput } from '../note-tags-input/note-tags-input';

/**
 * Quick-add composer at the top of the notes page. Collapsed it is a single line; expanded it has an
 * optional title, a markdown body, a color and tags. A note needs a title or a body.
 */
@Component({
  selector: 'app-note-composer',
  imports: [
    Button,
    InputText,
    NoteColorPalette,
    NoteTagsInput,
    ReactiveFormsModule,
    RichTextEditor,
  ],
  templateUrl: './note-composer.html',
  styleUrl: './note-composer.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NoteComposer {
  private readonly store = inject(NotesStore);
  private readonly notify = inject(NotificationService);
  private readonly injector = inject(Injector);
  private readonly titleField = viewChild<ElementRef<HTMLInputElement>>('titleField');

  /** Existing tags offered as quick picks. */
  readonly suggestions = input<readonly string[]>([]);
  /** Tag the page is filtered by: a new note starts with it. */
  readonly defaultTag = input<string | null>(null);
  readonly created = output<Note>();

  protected readonly titleMax = NOTE_TITLE_MAX;
  protected readonly bodyMax = NOTE_BODY_MAX;
  protected readonly expanded = signal(false);
  protected readonly saving = signal(false);
  protected readonly color = signal<NoteColor>(DEFAULT_NOTE_COLOR);
  protected readonly tags = signal<readonly string[]>([]);
  protected readonly showEmptyError = signal(false);

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

  /** Opens the composer and puts the cursor in the title (the empty-state call to action). */
  focusComposer(): void {
    this.expand();
  }

  protected expand(): void {
    this.expanded.set(true);
    const tag = this.defaultTag();
    if (tag && this.tags().length === 0) this.tags.set([tag]);
    afterNextRender(() => this.titleField()?.nativeElement.focus(), { injector: this.injector });
  }

  protected cancel(): void {
    this.reset();
  }

  /** The "write something" message goes away as soon as there is text. */
  protected onEdit(): void {
    if (this.showEmptyError() && !this.isBlank()) this.showEmptyError.set(false);
  }

  protected submit(): void {
    if (this.saving()) return;
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    if (this.isBlank()) {
      this.showEmptyError.set(true);
      return;
    }
    const { title, body } = this.form.getRawValue();
    this.saving.set(true);
    this.store
      .create({
        title: title.trim(),
        body: body.trim(),
        color: this.color(),
        tags: [...this.tags()],
      })
      .subscribe({
        next: (note) => {
          this.saving.set(false);
          this.reset();
          this.created.emit(note);
        },
        error: (error: unknown) => {
          this.saving.set(false);
          this.notify.error(error);
        },
      });
  }

  private isBlank(): boolean {
    const { title, body } = this.form.getRawValue();
    return title.trim() === '' && body.trim() === '';
  }

  private reset(): void {
    this.form.reset({ title: '', body: '' });
    this.color.set(DEFAULT_NOTE_COLOR);
    this.tags.set([]);
    this.showEmptyError.set(false);
    this.expanded.set(false);
  }
}

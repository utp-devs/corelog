import { ChangeDetectionStrategy, Component, computed, input, model, signal } from '@angular/core';
import { NOTE_TAGS_MAX, NOTE_TAG_MAX } from '@corelog/core';
import { AppTooltip } from '@corelog/ui/tooltip';
import { InputText } from 'primeng/inputtext';

import { addNoteTags } from '../shared/note-tags';

let nextTagsInputId = 0;

/**
 * Tags of a note as removable chips plus a field. Enter, comma or leaving the field adds what is
 * typed; tags are lowercased and never repeated, up to 10 of 30 characters each.
 */
@Component({
  selector: 'app-note-tags-input',
  imports: [AppTooltip, InputText],
  templateUrl: './note-tags-input.html',
  styleUrl: './note-tags-input.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NoteTagsInput {
  readonly tags = model<readonly string[]>([]);
  /** Existing tags offered as quick picks. */
  readonly suggestions = input<readonly string[]>([]);

  protected readonly draft = signal('');
  protected readonly error = signal<string | null>(null);
  protected readonly maxTagLength = NOTE_TAG_MAX;
  protected readonly maxTags = NOTE_TAGS_MAX;
  protected readonly id = `note-tags-${nextTagsInputId++}`;
  protected readonly errorId = `${this.id}-error`;
  protected readonly full = computed(() => this.tags().length >= NOTE_TAGS_MAX);
  protected readonly quickPicks = computed(() =>
    this.suggestions()
      .filter((tag) => !this.tags().includes(tag))
      .slice(0, 6),
  );

  protected onInput(event: Event): void {
    const field = event.target as HTMLInputElement;
    const value = field.value;
    if (value.includes(',')) {
      this.commit(value);
      // The draft may not change (still empty), so the field is cleared by hand.
      field.value = this.draft();
      return;
    }
    this.draft.set(value);
    this.error.set(null);
  }

  protected onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Enter') {
      event.preventDefault();
      this.commit(this.draft());
    } else if (event.key === 'Backspace' && this.draft() === '' && this.tags().length > 0) {
      this.remove(this.tags()[this.tags().length - 1]);
    }
  }

  protected commit(typed: string = this.draft()): void {
    if (typed.trim() === '' || typed.trim() === ',') {
      this.draft.set('');
      return;
    }
    const result = addNoteTags(this.tags(), typed);
    this.tags.set(result.tags);
    this.error.set(result.error);
    this.draft.set(result.error ? typed.replace(/,/g, '').trim() : '');
  }

  protected add(tag: string): void {
    this.commit(tag);
  }

  protected remove(tag: string): void {
    this.tags.set(this.tags().filter((item) => item !== tag));
    this.error.set(null);
  }
}

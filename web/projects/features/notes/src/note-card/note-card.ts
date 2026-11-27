import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { noteHeadline, type Note, type NoteColor } from '@corelog/core';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Popover } from 'primeng/popover';

import { NoteColorPalette } from '../note-color-palette/note-color-palette';

/**
 * One note of the grid: colored card with title, rendered markdown (clipped), tags and the actions
 * pin, change color and delete. The whole card opens the note in the editor.
 */
@Component({
  selector: 'app-note-card',
  imports: [AppTooltip, NoteColorPalette, Popover, RichTextView],
  templateUrl: './note-card.html',
  styleUrl: './note-card.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NoteCard {
  readonly note = input.required<Note>();

  readonly openNote = output<Note>();
  readonly togglePin = output<Note>();
  readonly changeColor = output<{ note: Note; color: NoteColor }>();
  readonly deleteNote = output<Note>();

  protected readonly headline = computed(() => noteHeadline(this.note()));
  protected readonly hasTitle = computed(() => this.note().title.trim() !== '');
  protected readonly hasBody = computed(() => this.note().body.trim() !== '');

  protected pickColor(color: NoteColor): void {
    if (color !== this.note().color) this.changeColor.emit({ note: this.note(), color });
  }
}

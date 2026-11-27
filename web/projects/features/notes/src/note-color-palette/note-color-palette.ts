import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  input,
  model,
  viewChildren,
} from '@angular/core';
import { NOTE_COLORS, NOTE_COLOR_LABELS, type NoteColor } from '@corelog/core';
import { AppTooltip } from '@corelog/ui/tooltip';

/** Palette of the eight note colors, a keyboard-navigable radio group. */
@Component({
  selector: 'app-note-color-palette',
  imports: [AppTooltip],
  templateUrl: './note-color-palette.html',
  styleUrl: './note-color-palette.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NoteColorPalette {
  readonly value = model<NoteColor>('yellow');
  /** Accessible name of the group. */
  readonly label = input('Color de la nota');

  protected readonly colors = NOTE_COLORS;
  protected readonly labels = NOTE_COLOR_LABELS;
  private readonly swatches = viewChildren<ElementRef<HTMLButtonElement>>('swatch');

  protected select(color: NoteColor): void {
    this.value.set(color);
  }

  protected onKeydown(event: KeyboardEvent, index: number): void {
    const last = this.colors.length - 1;
    let next: number | null = null;
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown')
      next = index === last ? 0 : index + 1;
    else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp')
      next = index === 0 ? last : index - 1;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = last;
    if (next === null) return;
    event.preventDefault();
    this.value.set(this.colors[next]);
    this.swatches()[next]?.nativeElement.focus();
  }
}

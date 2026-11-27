import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  model,
  signal,
  untracked,
  viewChild,
  ElementRef,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import {
  NOTE_BODY_MAX,
  NotesStore,
  NotificationService,
  noteHeadline,
  notePreview,
  type Note,
} from '@corelog/core';
import { AppTooltip } from '@corelog/ui/tooltip';
import { SidePanel } from '@corelog/ui/side-panel';
import { Button } from 'primeng/button';
import { Textarea } from 'primeng/textarea';

/**
 * Quick scratchpad of the signed-in user: a one-field composer and the five most recent notes.
 * Opens over any page. Notes are private; "Ver todas" goes to the notes page.
 */
@Component({
  selector: 'app-scratchpad-drawer',
  imports: [AppTooltip, Button, FormsModule, SidePanel, Textarea],
  templateUrl: './scratchpad-drawer.html',
  styleUrl: './scratchpad-drawer.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ScratchpadDrawer {
  private readonly store = inject(NotesStore);
  private readonly router = inject(Router);
  private readonly notify = inject(NotificationService);

  readonly open = model(false);
  private readonly field = viewChild<ElementRef<HTMLTextAreaElement>>('field');

  protected readonly draft = signal('');
  protected readonly saving = signal(false);
  protected readonly maxLength = NOTE_BODY_MAX;
  protected readonly recent = this.store.recent;
  protected readonly canSave = computed(() => this.draft().trim() !== '' && !this.saving());

  constructor() {
    effect(() => {
      if (this.open()) untracked(() => this.store.loadRecent());
    });
  }

  protected close(): void {
    this.open.set(false);
  }

  protected save(): void {
    const body = this.draft().trim();
    if (!body || this.saving()) return;
    this.saving.set(true);
    this.store.create({ body }).subscribe({
      next: () => {
        this.draft.set('');
        this.saving.set(false);
        this.notify.success('Nota guardada');
        // The save button is disabled again: keep the focus in the field for the next note.
        this.field()?.nativeElement.focus();
      },
      error: (error: unknown) => {
        this.saving.set(false);
        this.notify.error(error);
      },
    });
  }

  protected onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      this.save();
    }
  }

  protected togglePin(note: Note): void {
    this.store.togglePin(note);
  }

  protected headline(note: Note): string {
    return noteHeadline(note);
  }

  protected preview(note: Note): string {
    return note.title.trim() ? notePreview(note.body, 90) : '';
  }

  protected openNote(note: Note): void {
    this.close();
    void this.router.navigate(['/notes'], { queryParams: { note: note.id } });
  }

  protected openAll(): void {
    this.close();
    void this.router.navigate(['/notes']);
  }
}

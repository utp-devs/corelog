import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import {
  NOTE_BODY_MAX,
  NotesStore,
  NotificationService,
  noteHeadline,
  notePreview,
  type Note,
} from '@corelog/core';
import { Section } from '@corelog/ui/section';
import { Button } from 'primeng/button';
import { Textarea } from 'primeng/textarea';

/**
 * "Mis apuntes" of an issue: the private notes of the signed-in user linked to it, and a field that
 * adds a new one linked to the issue. Nobody else sees them.
 */
@Component({
  selector: 'app-issue-notes-section',
  imports: [Button, FormsModule, RouterLink, Section, Textarea],
  templateUrl: './issue-notes-section.html',
  styleUrl: './issue-notes-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueNotesSection {
  private readonly store = inject(NotesStore);
  private readonly notify = inject(NotificationService);

  /** Id of the issue the notes are linked to. */
  readonly issueId = input.required<string>();

  protected readonly notes = this.store.issueNotes;
  protected readonly draft = signal('');
  protected readonly saving = signal(false);
  protected readonly maxLength = NOTE_BODY_MAX;
  protected readonly canSave = computed(() => this.draft().trim() !== '' && !this.saving());

  constructor() {
    effect(() => {
      const id = this.issueId();
      untracked(() => this.store.loadForIssue(id));
    });
  }

  protected add(): void {
    const body = this.draft().trim();
    if (!body || this.saving()) return;
    this.saving.set(true);
    this.store.create({ body, issueId: this.issueId() }).subscribe({
      next: () => {
        this.draft.set('');
        this.saving.set(false);
        this.notify.success('Apunte añadido a Mis apuntes');
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
      this.add();
    }
  }

  protected headline(note: Note): string {
    return noteHeadline(note);
  }

  protected preview(note: Note): string {
    return note.title.trim() ? notePreview(note.body, 140) : '';
  }
}

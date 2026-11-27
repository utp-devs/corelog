import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { IssueActivity, IssueComment, MentionUser } from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { RichTextEditor } from '@corelog/ui/rich-text-editor';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { Section } from '@corelog/ui/section';
import { AppTooltip } from '@corelog/ui/tooltip';
import { UserAvatar } from '@corelog/ui/user-avatar';
import { Button } from 'primeng/button';

import { NameOf, describeActivity } from '../shared/activity-text';
import { exactDate, timeAgo } from '../shared/time';

export type ActivityTab = 'comments' | 'history' | 'all';

interface TabDefinition {
  readonly id: ActivityTab;
  readonly label: string;
}

const TABS: readonly TabDefinition[] = [
  { id: 'comments', label: 'Comentarios' },
  { id: 'history', label: 'Historial' },
  { id: 'all', label: 'Todo' },
];

interface CommentEntry {
  readonly kind: 'comment';
  readonly key: string;
  readonly at: string;
  readonly comment: IssueComment;
}

interface ActivityEntry {
  readonly kind: 'activity';
  readonly key: string;
  readonly at: string;
  readonly activity: IssueActivity;
}

type FeedEntry = CommentEntry | ActivityEntry;

/** A comment counts as edited when it changed more than a second after it was written. */
const EDITED_AFTER_MS = 1000;

let nextActivityId = 0;

/**
 * Comments and history of an issue in one block with the tabs "Comentarios", "Historial" and "Todo".
 * The composer stays one line high until it gets focus. The parent supplies the actions as callbacks
 * that resolve to true when they worked, so a form only closes or clears on success.
 */
@Component({
  selector: 'app-issue-activity',
  imports: [
    AppTooltip,
    Button,
    ConfirmModal,
    LoadingSkeleton,
    ReactiveFormsModule,
    RichTextEditor,
    RichTextView,
    Section,
    UserAvatar,
  ],
  templateUrl: './issue-activity.html',
  styleUrl: './issue-activity.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueActivityFeed {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly comments = input.required<readonly IssueComment[]>();
  readonly activities = input.required<readonly IssueActivity[]>();
  readonly loading = input(false);
  /** Person writing: their avatar sits next to the composer, and only they can edit their comments. */
  readonly currentUser = input<{ readonly id: string; readonly name: string } | null>(null);
  readonly nameOf = input.required<NameOf>();
  readonly searchUsers = input<(query: string) => Promise<MentionUser[]>>();
  /** Adds a comment. Resolves true when it was saved. */
  readonly onCreate = input.required<(body: string) => Promise<boolean>>();
  /** Saves the new text of a comment. Resolves true when it was saved. */
  readonly onUpdate = input.required<(commentId: string, body: string) => Promise<boolean>>();
  /** Deletes a comment. Resolves true when it was deleted. */
  readonly onDelete = input.required<(commentId: string) => Promise<boolean>>();

  protected readonly tabs = TABS;
  protected readonly panelId = `issue-activity-${nextActivityId++}-panel`;
  protected readonly tab = signal<ActivityTab>('comments');

  protected readonly composerOpen = signal(false);
  protected readonly sending = signal(false);
  protected readonly composerControl = new FormControl('', { nonNullable: true });
  private readonly composerValue = toSignal(this.composerControl.valueChanges, {
    initialValue: '',
  });
  protected readonly canSend = computed(() => this.composerValue().trim().length > 0);

  protected readonly editingId = signal<string | null>(null);
  protected readonly savingEdit = signal(false);
  protected readonly editControl = new FormControl('', { nonNullable: true });

  protected readonly deletingId = signal<string | null>(null);
  protected readonly deleteBusy = signal(false);

  protected readonly showComposer = computed(
    () => this.tab() !== 'history' && this.currentUser() !== null,
  );

  protected readonly counts = computed<Record<ActivityTab, number>>(() => ({
    comments: this.comments().length,
    history: this.activities().length,
    all: this.comments().length + this.activities().length,
  }));

  /** Newest first, filtered by the selected tab. */
  protected readonly entries = computed<FeedEntry[]>(() => {
    const tab = this.tab();
    const items: FeedEntry[] = [];
    if (tab !== 'history') {
      for (const comment of this.comments()) {
        items.push({ kind: 'comment', key: `c-${comment.id}`, at: comment.createdAt, comment });
      }
    }
    if (tab !== 'comments') {
      for (const activity of this.activities()) {
        items.push({ kind: 'activity', key: `a-${activity.id}`, at: activity.createdAt, activity });
      }
    }
    return items.sort((a, b) => Date.parse(b.at) - Date.parse(a.at));
  });

  protected readonly emptyText = computed(() => {
    switch (this.tab()) {
      case 'comments':
        return 'Aún no hay comentarios';
      case 'history':
        return 'Aún no hay actividad';
      default:
        return 'Aún no hay actividad ni comentarios';
    }
  });

  protected selectTab(tab: ActivityTab): void {
    this.tab.set(tab);
  }

  protected tabId(tab: ActivityTab): string {
    return `${this.panelId}-${tab}`;
  }

  /** Arrow keys, Home and End move between tabs, as the tabs pattern asks. */
  protected onTabKeydown(event: KeyboardEvent): void {
    const index = TABS.findIndex((item) => item.id === this.tab());
    let next: number;
    if (event.key === 'ArrowRight') next = (index + 1) % TABS.length;
    else if (event.key === 'ArrowLeft') next = (index - 1 + TABS.length) % TABS.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = TABS.length - 1;
    else return;
    event.preventDefault();
    this.tab.set(TABS[next].id);
    this.focusLater('[role="tab"][tabindex="0"]');
  }

  // Composer.

  protected openComposer(): void {
    this.composerOpen.set(true);
    this.focusLater('.issue-activity__composer textarea');
  }

  protected cancelComposer(): void {
    this.composerControl.setValue('');
    this.composerOpen.set(false);
  }

  protected async submitComposer(): Promise<void> {
    const body = this.composerControl.value;
    if (!body.trim() || this.sending()) return;
    this.sending.set(true);
    let saved = false;
    try {
      saved = await this.onCreate()(body);
    } finally {
      this.sending.set(false);
    }
    if (saved) this.cancelComposer();
  }

  // Edit.

  protected startEdit(comment: IssueComment): void {
    this.editControl.setValue(comment.body);
    this.editingId.set(comment.id);
  }

  protected cancelEdit(): void {
    this.editingId.set(null);
  }

  protected async saveEdit(comment: IssueComment): Promise<void> {
    const body = this.editControl.value;
    if (!body.trim() || this.savingEdit()) return;
    this.savingEdit.set(true);
    let saved = false;
    try {
      saved = await this.onUpdate()(comment.id, body);
    } finally {
      this.savingEdit.set(false);
    }
    if (saved) this.editingId.set(null);
  }

  // Delete.

  protected askDelete(comment: IssueComment): void {
    this.deletingId.set(comment.id);
  }

  protected cancelDelete(): void {
    if (!this.deleteBusy()) this.deletingId.set(null);
  }

  protected async confirmDelete(): Promise<void> {
    const id = this.deletingId();
    if (!id || this.deleteBusy()) return;
    this.deleteBusy.set(true);
    try {
      await this.onDelete()(id);
    } finally {
      this.deleteBusy.set(false);
      this.deletingId.set(null);
    }
  }

  // Display helpers.

  protected isOwn(comment: IssueComment): boolean {
    return comment.authorId === this.currentUser()?.id;
  }

  protected isEdited(comment: IssueComment): boolean {
    return Date.parse(comment.updatedAt) - Date.parse(comment.createdAt) > EDITED_AFTER_MS;
  }

  protected ago(iso: string): string {
    return timeAgo(iso);
  }

  protected exact(iso: string): string {
    return exactDate(iso);
  }

  protected activityText(activity: IssueActivity): string {
    return describeActivity(activity, this.nameOf());
  }

  private focusLater(selector: string): void {
    afterNextRender(() => this.host.nativeElement.querySelector<HTMLElement>(selector)?.focus(), {
      injector: this.injector,
    });
  }
}

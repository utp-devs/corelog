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
import { FormControl, FormsModule, ReactiveFormsModule } from '@angular/forms';
import {
  ActivityEntry,
  ActivityFilter,
  AuthStore,
  COMMENT_REACTIONS,
  CommentInternalRole,
  CommentReactionKey,
  CommentRevision,
  CommentThread,
  CommentV2,
  CommentV2ApiService,
  IssueViewStore,
  NotificationService,
  asApiError,
} from '@corelog/core';
import { ActivityFeed, ActivityItemView, ActivityLineView } from '@corelog/ui/activity-feed';
import {
  CommentReactionEvent,
  CommentThread as CommentThreadComponent,
  CommentThreadView as ThreadView,
  CommentView,
} from '@corelog/ui/comment-thread';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { Dialog } from '@corelog/ui/dialog';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { RichTextEditor } from '@corelog/ui/rich-text-editor';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { Section } from '@corelog/ui/section';
import { SelectField } from '@corelog/ui/select-field';
import { UserAvatar } from '@corelog/ui/user-avatar';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { firstValueFrom } from 'rxjs';

import { MentionSearch } from '../shared/mention-search';
import { exactDate, timeAgo } from '../shared/time';

type Tab = 'all' | 'comments' | 'history';

const TABS: readonly { id: Tab; label: string }[] = [
  { id: 'all', label: 'Todo' },
  { id: 'comments', label: 'Comentarios' },
  { id: 'history', label: 'Historial' },
];

const ROLE_OPTIONS: readonly { value: CommentInternalRole; label: string }[] = [
  { value: 'contributor', label: 'Colaboradores y líderes' },
  { value: 'lead', label: 'Solo líderes' },
];

const PAGE_SIZE = 25;

const STATUS_NAMES: Readonly<Record<string, string>> = {
  open: 'Abierto',
  in_progress: 'En progreso',
  resolved: 'Resuelto',
  closed: 'Cerrado',
};

/**
 * Conversation of one issue in three tabs: everything, threaded comments (composer with @ mentions,
 * replies, reactions, edit with revisions, delete, internal visibility) and the history of changes
 * grouped by update. Pages are loaded by cursor with a "Cargar más" button.
 */
@Component({
  selector: 'app-issue-conversation-section',
  imports: [
    ActivityFeed,
    Button,
    CommentThreadComponent,
    ConfirmModal,
    Dialog,
    FormsModule,
    LoadingSkeleton,
    Message,
    ReactiveFormsModule,
    RichTextEditor,
    RichTextView,
    Section,
    SelectField,
    UserAvatar,
  ],
  templateUrl: './issue-conversation-section.html',
  styleUrl: './issue-conversation-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueConversationSection {
  private readonly api = inject(CommentV2ApiService);
  private readonly auth = inject(AuthStore);
  private readonly viewStore = inject(IssueViewStore);
  private readonly notify = inject(NotificationService);
  protected readonly searchUsers = inject(MentionSearch).search;

  readonly issueId = input.required<string>();
  readonly archived = input(false);

  protected readonly tabs = TABS;
  protected readonly roleOptions = ROLE_OPTIONS;
  protected readonly reactionChoices = COMMENT_REACTIONS.map((r) => ({
    key: r.key,
    glyph: r.glyph,
    label: r.label,
  }));
  protected readonly tab = signal<Tab>('all');

  // Activity (all, history).
  protected readonly entries = signal<readonly ActivityEntry[]>([]);
  protected readonly entriesCursor = signal<string | null>(null);
  // Threads.
  protected readonly threads = signal<readonly CommentThread[]>([]);
  protected readonly threadsCursor = signal<string | null>(null);

  protected readonly loading = signal(true);
  protected readonly loadingMore = signal(false);
  protected readonly loadError = signal<string | null>(null);

  // Composer.
  protected readonly composerOpen = signal(false);
  protected readonly composer = new FormControl('', { nonNullable: true });
  protected readonly internal = signal(false);
  protected readonly internalRole = signal<CommentInternalRole>('contributor');
  protected readonly sending = signal(false);
  protected readonly composerError = signal<string | null>(null);

  // Replies, edits, deletes and revisions.
  protected readonly replyingId = signal<string | null>(null);
  protected readonly replyControl = new FormControl('', { nonNullable: true });
  protected readonly editingId = signal<string | null>(null);
  protected readonly editControl = new FormControl('', { nonNullable: true });
  protected readonly busyId = signal<string | null>(null);
  protected readonly deletingId = signal<string | null>(null);
  protected readonly revisions = signal<readonly CommentRevision[] | null>(null);
  protected readonly revisionsFor = signal<string | null>(null);

  protected readonly me = computed(() => {
    const user = this.auth.user();
    return user ? { id: user.id, name: user.name } : null;
  });
  protected readonly canComment = computed(
    () => !this.archived() && this.viewStore.can('comment_issue') && this.me() !== null,
  );
  protected readonly canManage = computed(() => this.viewStore.can('manage_comments'));
  protected readonly counts = computed(() => this.viewStore.view()?.counts.comments ?? 0);

  protected readonly threadViews = computed<ThreadView[]>(() =>
    this.threads().map((thread) => ({
      ...this.toView(thread),
      replies: thread.replies.map((reply) => this.toView(reply)),
    })),
  );
  protected readonly feedItems = computed<ActivityItemView[]>(() =>
    this.entries().map((entry) => ({
      id: `${entry.type}-${entry.id}`,
      kind: entry.type,
      actorId: entry.actor.id,
      actorName: entry.actor.name,
      timeText: timeAgo(entry.createdAt),
      timeTitle: exactDate(entry.createdAt),
      body: entry.comment?.body,
      edited: entry.comment?.edited,
      lines: entry.changes.map((change) => this.lineOf(change)),
    })),
  );
  protected readonly hasMore = computed(() =>
    this.tab() === 'comments' ? this.threadsCursor() !== null : this.entriesCursor() !== null,
  );

  constructor() {
    effect(() => {
      const id = this.issueId();
      const tab = this.tab();
      if (id) untracked(() => void this.load(tab));
    });
  }

  protected selectTab(tab: Tab): void {
    this.tab.set(tab);
  }

  protected tabId(tab: Tab): string {
    return `conversation-tab-${tab}`;
  }

  protected onTabKeydown(event: KeyboardEvent): void {
    const order = TABS.map((item) => item.id);
    const index = order.indexOf(this.tab());
    let next = index;
    if (event.key === 'ArrowRight') next = (index + 1) % order.length;
    else if (event.key === 'ArrowLeft') next = (index - 1 + order.length) % order.length;
    else return;
    event.preventDefault();
    this.tab.set(order[next]);
    queueMicrotask(() => document.getElementById(this.tabId(order[next]))?.focus());
  }

  protected openComposer(): void {
    this.composerOpen.set(true);
  }

  protected cancelComposer(): void {
    this.composerOpen.set(false);
    this.composer.reset('');
    this.internal.set(false);
    this.composerError.set(null);
  }

  protected canSend(): boolean {
    return this.composer.value.trim() !== '' && !this.sending();
  }

  protected async submitComposer(): Promise<void> {
    const body = this.composer.value.trim();
    if (!body || this.sending()) return;
    this.sending.set(true);
    this.composerError.set(null);
    try {
      const written = await firstValueFrom(
        this.api.create(this.issueId(), {
          body,
          visibility: this.internal() ? 'internal' : 'public',
          internalRole: this.internal() ? this.internalRole() : undefined,
        }),
      );
      this.warn(written.warnings);
      this.cancelComposer();
      this.viewStore.adjustCounts({ comments: 1 });
      await this.load(this.tab(), false);
    } catch (error) {
      this.composerError.set(asApiError(error).message);
    } finally {
      this.sending.set(false);
    }
  }

  protected startReply(threadId: string): void {
    this.editingId.set(null);
    this.replyControl.reset('');
    this.replyingId.set(threadId);
  }

  protected cancelReply(): void {
    this.replyingId.set(null);
    this.replyControl.reset('');
  }

  protected async submitReply(thread: ThreadView): Promise<void> {
    const body = this.replyControl.value.trim();
    if (!body) return;
    this.busyId.set(thread.id);
    try {
      const written = await firstValueFrom(
        this.api.create(this.issueId(), {
          body,
          parentId: thread.id,
          visibility: thread.internal ? 'internal' : 'public',
          internalRole: thread.internal ? 'contributor' : undefined,
        }),
      );
      this.warn(written.warnings);
      this.cancelReply();
      this.viewStore.adjustCounts({ comments: 1 });
      await this.load('comments', false);
    } catch (error) {
      this.notify.error(asApiError(error).message);
    } finally {
      this.busyId.set(null);
    }
  }

  protected startEdit(commentId: string): void {
    const comment = this.findComment(commentId);
    if (!comment) return;
    this.replyingId.set(null);
    this.editControl.setValue(comment.body);
    this.editingId.set(commentId);
  }

  protected cancelEdit(): void {
    this.editingId.set(null);
  }

  protected async saveEdit(commentId: string): Promise<void> {
    const body = this.editControl.value.trim();
    if (!body) return;
    this.busyId.set(commentId);
    try {
      const written = await firstValueFrom(this.api.update(commentId, body));
      this.warn(written.warnings);
      this.editingId.set(null);
      this.replaceComment(written.comment);
    } catch (error) {
      this.notify.error(asApiError(error).message);
    } finally {
      this.busyId.set(null);
    }
  }

  protected async confirmDelete(): Promise<void> {
    const id = this.deletingId();
    if (!id) return;
    this.busyId.set(id);
    try {
      await firstValueFrom(this.api.remove(id));
      this.deletingId.set(null);
      this.viewStore.adjustCounts({ comments: -1 });
      this.notify.success('Comentario eliminado');
      await this.load(this.tab(), false);
    } catch (error) {
      this.deletingId.set(null);
      this.notify.error(asApiError(error).message);
    } finally {
      this.busyId.set(null);
    }
  }

  protected async toggleReaction(event: CommentReactionEvent): Promise<void> {
    const comment = this.findComment(event.commentId);
    if (!comment) return;
    const key = event.key as CommentReactionKey;
    const mine = comment.reactions.find((r) => r.emoji === key)?.viewerReacted === true;
    try {
      const reactions = await firstValueFrom(this.api.react(comment.id, key, !mine));
      this.replaceComment({ ...comment, reactions });
    } catch (error) {
      this.notify.error(asApiError(error).message);
    }
  }

  protected async showRevisions(commentId: string): Promise<void> {
    this.revisionsFor.set(commentId);
    this.revisions.set(null);
    try {
      this.revisions.set(await firstValueFrom(this.api.revisions(commentId)));
    } catch {
      this.revisions.set([]);
    }
  }

  protected closeRevisions(): void {
    this.revisionsFor.set(null);
    this.revisions.set(null);
  }

  protected async loadMore(): Promise<void> {
    const tab = this.tab();
    this.loadingMore.set(true);
    try {
      if (tab === 'comments') {
        const page = await firstValueFrom(
          this.api.listThreads(this.issueId(), { cursor: this.threadsCursor(), limit: PAGE_SIZE }),
        );
        this.threads.update((list) => [...list, ...page.threads]);
        this.threadsCursor.set(page.nextCursor);
      } else {
        const page = await firstValueFrom(
          this.api.activity(this.issueId(), tab as ActivityFilter, this.entriesCursor(), PAGE_SIZE),
        );
        this.entries.update((list) => [...list, ...page.entries]);
        this.entriesCursor.set(page.nextCursor);
      }
    } catch (error) {
      this.notify.error(asApiError(error).message);
    } finally {
      this.loadingMore.set(false);
    }
  }

  private async load(tab: Tab, skeleton = true): Promise<void> {
    const issueId = this.issueId();
    if (skeleton) this.loading.set(true);
    this.loadError.set(null);
    try {
      if (tab === 'comments') {
        const page = await firstValueFrom(this.api.listThreads(issueId, { limit: PAGE_SIZE }));
        if (issueId !== this.issueId() || tab !== this.tab()) return;
        this.threads.set(page.threads);
        this.threadsCursor.set(page.nextCursor);
      } else {
        const page = await firstValueFrom(this.api.activity(issueId, tab, null, PAGE_SIZE));
        if (issueId !== this.issueId() || tab !== this.tab()) return;
        this.entries.set(page.entries);
        this.entriesCursor.set(page.nextCursor);
      }
    } catch (error) {
      this.loadError.set(asApiError(error).message);
    } finally {
      this.loading.set(false);
    }
  }

  private lineOf(change: ActivityEntry['changes'][number]): ActivityLineView {
    const statusName = (value: string | null): string | null =>
      value === null ? null : (STATUS_NAMES[value] ?? value);
    if (change.action === 'link_added') {
      return {
        label: change.label,
        from: null,
        to: null,
        text: 'Vínculo añadido: ' + (change.to ?? ''),
      };
    }
    if (change.action === 'link_removed') {
      return {
        label: change.label,
        from: null,
        to: null,
        text: 'Vínculo eliminado: ' + (change.from ?? ''),
      };
    }
    if (change.action === 'created' || (!change.label && !change.field)) {
      return { label: '', from: null, to: null, text: 'Creó la incidencia' };
    }
    const isStatus = change.field === 'status';
    return {
      label: change.label,
      from: isStatus ? statusName(change.from) : change.from,
      to: isStatus ? statusName(change.to) : change.to,
    };
  }

  private toView(comment: CommentV2): CommentView {
    const mine = comment.authorId === this.me()?.id;
    const open = !this.archived();
    return {
      id: comment.id,
      authorId: comment.authorId,
      authorName: comment.authorName,
      body: comment.body,
      timeText: timeAgo(comment.createdAt),
      timeTitle: exactDate(comment.createdAt),
      edited: comment.edited,
      deleted: comment.deleted,
      internal: comment.visibility === 'internal',
      reactions: comment.reactions.map((reaction) => {
        const choice = COMMENT_REACTIONS.find((item) => item.key === reaction.emoji);
        return {
          key: reaction.emoji,
          glyph: choice?.glyph ?? reaction.emoji,
          label: choice?.label ?? reaction.emoji,
          count: reaction.count,
          viewerReacted: reaction.viewerReacted,
        };
      }),
      canEdit: open && !comment.deleted && (mine || this.canManage()),
      canDelete: open && !comment.deleted && (mine || this.canManage()),
    };
  }

  private findComment(id: string): CommentV2 | undefined {
    for (const thread of this.threads()) {
      if (thread.id === id) return thread;
      const reply = thread.replies.find((item) => item.id === id);
      if (reply) return reply;
    }
    return undefined;
  }

  private replaceComment(next: CommentV2): void {
    this.threads.update((list) =>
      list.map((thread) =>
        thread.id === next.id
          ? { ...thread, ...next, replyCount: thread.replyCount, replies: thread.replies }
          : {
              ...thread,
              replies: thread.replies.map((reply) => (reply.id === next.id ? next : reply)),
            },
      ),
    );
  }

  private warn(warnings: readonly string[]): void {
    if (warnings.length > 0) {
      this.notify.info(
        'Algunas menciones se guardaron como texto porque la persona no puede ver el comentario.',
      );
    }
  }
}

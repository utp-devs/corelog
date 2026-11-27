import { DatePipe } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  Signal,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormControl, FormsModule, ReactiveFormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { Select } from 'primeng/select';
import { Subscription, firstValueFrom, map } from 'rxjs';
import {
  AsyncState,
  AuthStore,
  ISSUE_PRIORITY_LABELS,
  ISSUE_STATUSES,
  ISSUE_STATUS_META,
  ISSUE_TYPE_LABELS,
  IssueAttachment,
  IssueComment,
  IssueActivity,
  IssueLink,
  IssueLinkKind,
  IssuePriority,
  IssueStatus,
  NotificationService,
  ProjectApiService,
  ProjectMember,
  Ticket,
  TicketStore,
  UpdateIssueDetailsInput,
  canTransitionTo,
  effectiveProjectRole,
  hasProjectPermission,
  issueKeyOf,
  ISSUES_PATH,
  issuePath,
} from '@corelog/core';
import { trackRecent } from '@corelog/ui/command-palette';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { FieldRow } from '@corelog/ui/field-row';
import { InlineEdit, InlineEditOption } from '@corelog/ui/inline-edit';
import { IssueKey } from '@corelog/ui/issue-key';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer, PageSidebar } from '@corelog/ui/page-container';
import { RichTextEditor } from '@corelog/ui/rich-text-editor';
import { RichTextView } from '@corelog/ui/rich-text-view';
import { Section } from '@corelog/ui/section';
import { PickableUser, UserPicker } from '@corelog/ui/user-picker';

import { errorMessage } from '../shared/error-message';
import { MentionSearch } from '../shared/mention-search';
import { payloadFrom } from '../shared/issue-payload';
import { injectMemberNames } from '../hooks/member-names';
import { injectTeammateSearch } from '../hooks/teammate-search';
import { IssuePicker } from '../issue-picker/issue-picker';
import { TicketDescriptionSection } from './ticket-description-section';

const NOT_FOUND_TEXT = 'No se pudo encontrar la incidencia solicitada.';
const COLLABORATION_ERROR = 'No se pudo cargar toda la información de colaboración.';
const LINK_KIND_LABELS: Record<IssueLinkKind, string> = {
  blocks: 'Bloquea a',
  relates: 'Se relaciona con',
  duplicates: 'Duplica a',
};
const LINK_KIND_OPTIONS: { value: IssueLinkKind; label: string }[] = [
  { value: 'blocks', label: 'Bloquea' },
  { value: 'relates', label: 'Se relaciona con' },
  { value: 'duplicates', label: 'Duplica' },
];
const PRIORITY_OPTIONS: InlineEditOption[] = (
  Object.keys(ISSUE_PRIORITY_LABELS) as IssuePriority[]
).map((value) => ({ value, label: ISSUE_PRIORITY_LABELS[value] }));

interface ProjectMembership {
  readonly projectId: string;
  readonly members: ProjectMember[];
}

/**
 * Issue detail: inline-editable title, status and priority, description, links, files, comments,
 * history, assignee and follow toggle. Collaboration data is loaded per issue; a late response for
 * another issue is ignored by the store.
 */
@Component({
  selector: 'app-ticket-detail-page',
  imports: [
    Button,
    DatePipe,
    EmptyState,
    ErrorState,
    FieldRow,
    IssueKey,
    IssuePicker,
    InlineEdit,
    LoadingSkeleton,
    FormsModule,
    Message,
    PageContainer,
    PageSidebar,
    ReactiveFormsModule,
    RichTextEditor,
    RichTextView,
    RouterLink,
    Section,
    Select,
    TicketDescriptionSection,
    UserPicker,
  ],
  templateUrl: './ticket-detail-page.html',
  styleUrl: './ticket-detail-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TicketDetailPage {
  private readonly store = inject(TicketStore);
  private readonly auth = inject(AuthStore);
  private readonly projectApi = inject(ProjectApiService);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly names = injectMemberNames();
  protected readonly teammates = injectTeammateSearch();
  protected readonly mentionSearch = inject(MentionSearch).search;

  protected readonly id = toSignal(
    this.route.paramMap.pipe(map((params) => params.get('id') ?? '')),
    { initialValue: '' },
  );

  /** Issue shown on the page: loaded, and the one the route asks for. */
  protected readonly detail = computed<Ticket | undefined>(() => {
    const ticket = this.store.selected.value();
    return ticket && !this.store.selected.loading() && ticket.id === this.id() ? ticket : undefined;
  });
  protected readonly detailItems = computed<Ticket[]>(() => {
    const ticket = this.detail();
    return ticket ? [ticket] : [];
  });
  protected readonly showSkeleton = computed(
    () =>
      this.store.selected.loading() ||
      (!this.store.selected.value() && this.store.selected.error() === null),
  );
  protected readonly showError = computed(() => !this.showSkeleton() && !this.detail());
  protected readonly errorText = computed(() => {
    const error = this.store.selected.error();
    return error ? errorMessage(error, NOT_FOUND_TEXT) : NOT_FOUND_TEXT;
  });

  protected readonly issueKeyOf = issueKeyOf;
  protected readonly issueTypeLabels = ISSUE_TYPE_LABELS;
  protected readonly priorityOptions = PRIORITY_OPTIONS;
  protected readonly linkKindOptions = LINK_KIND_OPTIONS;

  protected readonly statusOptions = computed<InlineEditOption[]>(() => {
    const ticket = this.detail();
    if (!ticket) return [];
    return ISSUE_STATUSES.filter(
      (status) => status === ticket.status || canTransitionTo(ticket.status, status),
    ).map((status) => ({ value: status, label: ISSUE_STATUS_META[status].label }));
  });

  // Assignee: membership decides whether the change is offered here. The server enforces it too.
  private readonly membership = signal<ProjectMembership | null>(null);
  private membersRequest: Subscription | undefined;
  protected readonly projectId = computed(() => this.detail()?.projectId);
  protected readonly canChangeAssignee = computed(() => {
    const ticket = this.detail();
    const loaded = this.membership();
    if (!ticket || !loaded || loaded.projectId !== ticket.projectId) return false;
    const user = this.auth.user();
    return hasProjectPermission(
      effectiveProjectRole(user?.role, user?.id, loaded.members),
      'edit_issue',
    );
  });
  protected readonly assigning = signal(false);
  protected readonly assignAnnouncement = signal('');
  protected readonly assigneeHelper = computed(() => {
    const ticket = this.detail();
    const current = ticket?.assigneeName ?? 'Sin asignar';
    return this.canChangeAssignee()
      ? `Actual: ${current}`
      : `Actual: ${current}. No tienes permiso para cambiar el responsable.`;
  });
  protected readonly assigneeUsers = computed<PickableUser[]>(() => {
    const ticket = this.detail();
    const results = this.teammates.results();
    const current: PickableUser[] =
      ticket?.assigneeId && !results.some((user) => user.id === ticket.assigneeId)
        ? [{ id: ticket.assigneeId, name: ticket.assigneeName ?? ticket.assigneeId }]
        : [];
    return [
      ...current,
      ...results.map((user) => ({ id: user.id, name: user.name, email: user.email })),
    ];
  });

  // Watchers.
  protected readonly watcherState = computed(() =>
    this.store.watchersIssueId() === this.id() ? this.store.watchers.value() : undefined,
  );
  protected readonly watcherLoading = computed(
    () => this.store.watchersIssueId() !== this.id() && this.store.watchers.error() === null,
  );
  protected readonly watcherError = computed(() => {
    const error = this.store.watchers.error();
    return error ? errorMessage(error, 'No se pudo cargar el seguimiento') : undefined;
  });
  protected readonly watcherCount = computed(() => this.watcherState()?.count ?? 0);
  protected readonly watcherBusy = signal(false);

  // Collaboration: each list is shown only once its request for this issue has settled.
  protected readonly collaborationLoading = computed(
    () =>
      !(
        this.settled(this.store.comments, this.store.commentsIssueId) &&
        this.settled(this.store.activities, this.store.activitiesIssueId) &&
        this.settled(this.store.links, this.store.linksIssueId)
      ),
  );
  protected readonly collaborationError = computed(() =>
    [this.store.comments, this.store.activities, this.store.links].some(
      (state) => state.error() !== null,
    )
      ? COLLABORATION_ERROR
      : undefined,
  );
  private readonly collaborationReady = computed(
    () => !this.collaborationLoading() && this.collaborationError() === undefined,
  );
  protected readonly visibleLinks = computed<readonly IssueLink[]>(() =>
    this.collaborationReady() && this.store.linksIssueId() === this.id()
      ? (this.store.links.value() ?? [])
      : [],
  );
  protected readonly visibleComments = computed<readonly IssueComment[]>(() =>
    this.collaborationReady() && this.store.commentsIssueId() === this.id()
      ? (this.store.comments.value() ?? [])
      : [],
  );
  protected readonly visibleActivities = computed<readonly IssueActivity[]>(() =>
    this.collaborationReady() && this.store.activitiesIssueId() === this.id()
      ? (this.store.activities.value() ?? [])
      : [],
  );

  // Attachments.
  protected readonly attachmentsLoading = computed(
    () => this.store.attachmentsIssueId() !== this.id() && this.store.attachments.error() === null,
  );
  protected readonly attachmentError = computed(() => {
    const error = this.store.attachments.error();
    return error ? errorMessage(error, 'No se pudieron cargar los archivos') : undefined;
  });
  protected readonly visibleAttachments = computed<readonly IssueAttachment[]>(() =>
    this.attachmentsLoading() || this.attachmentError()
      ? []
      : (this.store.attachments.value() ?? []),
  );

  // Links.
  protected readonly linkTarget = signal('');
  protected readonly linkKind = signal<IssueLinkKind>('relates');

  // Comments.
  protected readonly commentControl = new FormControl('', { nonNullable: true });
  /** True while a comment is being posted; the editor and the button stay disabled. */
  protected readonly commentSending = signal(false);
  private readonly commentBody = toSignal(this.commentControl.valueChanges, { initialValue: '' });
  protected readonly canComment = computed(
    () =>
      !this.collaborationLoading() &&
      !this.commentSending() &&
      this.commentBody().trim().length > 0,
  );

  constructor() {
    effect(() => {
      const id = this.id();
      if (id) untracked(() => this.store.fetchTicket(id));
    });

    effect(() => {
      const id = this.id();
      if (id) untracked(() => this.loadCollaboration(id));
    });

    effect(() => {
      const projectId = this.projectId();
      if (projectId) untracked(() => this.loadMembership(projectId));
    });

    effect(() => {
      const ticket = this.detail();
      if (ticket) {
        trackRecent({
          kind: 'issue',
          id: ticket.id,
          title: ticket.title,
          subtitle: 'Incidencia',
          link: issuePath(ticket.id),
        });
      }
    });

    this.destroyRef.onDestroy(() => this.membersRequest?.unsubscribe());
  }

  // Inline edits. Arrow properties keep `this` for the inline editors that call them.

  protected readonly saveTitle = async (next: string | null): Promise<void> => {
    const ticket = this.detail();
    if (!ticket) return;
    const title = next?.trim();
    if (!title) throw new Error('El título es obligatorio');
    await firstValueFrom(this.store.updateDetails(ticket.id, payloadFrom(ticket, { title })));
    this.notify.success('Título actualizado');
  };

  protected readonly saveStatus = async (next: string | null): Promise<void> => {
    const ticket = this.detail();
    if (!ticket) return;
    if (!next) throw new Error('El estado es obligatorio');
    await firstValueFrom(this.store.updateStatus(ticket.id, next as IssueStatus));
    this.notify.success('Estado actualizado');
  };

  protected readonly savePriority = async (next: string | null): Promise<void> => {
    const ticket = this.detail();
    if (!ticket) return;
    if (!next) throw new Error('La prioridad es obligatoria');
    await firstValueFrom(
      this.store.updateDetails(ticket.id, payloadFrom(ticket, { priority: next as IssuePriority })),
    );
    this.notify.success('Prioridad actualizada');
  };

  protected readonly saveDetails = async (input: UpdateIssueDetailsInput): Promise<boolean> => {
    const ticket = this.detail();
    if (!ticket) return false;
    try {
      await firstValueFrom(this.store.updateDetails(ticket.id, input));
      this.notify.success('Incidencia actualizada');
      return true;
    } catch (error) {
      this.notify.error(errorMessage(error, 'No se pudo actualizar la incidencia'));
      return false;
    }
  };

  // Assignee.

  protected assign(assigneeId: string | undefined): void {
    const ticket = this.detail();
    if (!ticket || !assigneeId || assigneeId === ticket.assigneeId) return;
    this.assigning.set(true);
    this.store.assign(ticket.id, assigneeId).subscribe({
      next: () => {
        this.assigning.set(false);
        this.store.fetchWatchers(ticket.id);
        this.notify.success('Incidencia asignada');
        this.assignAnnouncement.set('Responsable actualizado');
      },
      error: (error: unknown) => {
        this.assigning.set(false);
        this.notify.error(errorMessage(error, 'No se pudo asignar la incidencia'));
      },
    });
  }

  protected unassign(): void {
    const ticket = this.detail();
    if (!ticket) return;
    this.assigning.set(true);
    this.store.unassign(ticket.id).subscribe({
      next: () => {
        this.assigning.set(false);
        this.store.fetchWatchers(ticket.id);
        this.notify.success('Responsable quitado');
        this.assignAnnouncement.set('Sin asignar');
      },
      error: (error: unknown) => {
        this.assigning.set(false);
        this.notify.error(errorMessage(error, 'No se pudo quitar el responsable'));
      },
    });
  }

  // Follow.

  protected toggleWatch(): void {
    const ticket = this.detail();
    const state = this.watcherState();
    if (!ticket || !state) return;
    this.watcherBusy.set(true);
    const request = state.isWatching ? this.store.unwatch(ticket.id) : this.store.watch(ticket.id);
    request.subscribe({
      next: (next) => {
        this.watcherBusy.set(false);
        this.notify.success(
          next.isWatching ? 'Seguirás esta incidencia' : 'Dejaste de seguir esta incidencia',
        );
      },
      error: (error: unknown) => {
        this.watcherBusy.set(false);
        this.notify.error(errorMessage(error, 'No se pudo actualizar el seguimiento'));
      },
    });
  }

  // Links.

  protected onLinkKind(kind: IssueLinkKind): void {
    this.linkKind.set(kind);
  }

  protected createLink(): void {
    const ticket = this.detail();
    const target = this.linkTarget();
    if (!ticket || !target) return;
    this.store.createLink(ticket.id, target, this.linkKind()).subscribe({
      next: () => {
        this.linkTarget.set('');
        this.notify.success('Vinculo creado');
      },
      error: (error: unknown) =>
        this.notify.error(errorMessage(error, 'No se pudo crear el vinculo')),
    });
  }

  protected removeLink(linkId: string): void {
    const ticket = this.detail();
    if (!ticket) return;
    this.store.deleteLink(ticket.id, linkId).subscribe({
      error: (error: unknown) => this.notify.error(errorMessage(error, 'No se pudo eliminar')),
    });
  }

  protected linkLabel(link: IssueLink): string {
    const otherId = this.otherIssueId(link);
    return `${LINK_KIND_LABELS[link.kind]}: incidencia ${otherId.slice(0, 8)}`;
  }

  protected linkPath(link: IssueLink): string {
    return issuePath(this.otherIssueId(link));
  }

  // Comments.

  protected submitComment(): void {
    const ticket = this.detail();
    const body = this.commentControl.value;
    if (!ticket || !body.trim() || this.commentSending()) return;
    this.commentSending.set(true);
    this.commentControl.disable({ emitEvent: false });
    this.store.createComment(ticket.id, body).subscribe({
      next: () => {
        this.commentSending.set(false);
        this.commentControl.enable({ emitEvent: false });
        this.store.fetchWatchers(ticket.id);
        this.commentControl.setValue('');
        this.notify.success('Comentario agregado');
      },
      error: (error: unknown) => {
        this.commentSending.set(false);
        this.commentControl.enable({ emitEvent: false });
        this.notify.error(errorMessage(error, 'No se pudo agregar el comentario'));
      },
    });
  }

  protected removeComment(commentId: string): void {
    const ticket = this.detail();
    if (!ticket) return;
    this.store.deleteComment(ticket.id, commentId).subscribe({
      next: () => this.notify.success('Comentario eliminado'),
      error: (error: unknown) => this.notify.error(errorMessage(error, 'No se pudo eliminar')),
    });
  }

  // Attachments.

  protected onFileSelected(event: Event): void {
    const input = event.target instanceof HTMLInputElement ? event.target : null;
    const file = input?.files?.[0];
    if (input) input.value = '';
    const ticket = this.detail();
    if (!ticket || !file) return;
    this.store.uploadAttachment(ticket.id, file).subscribe({
      next: () => this.notify.success('Archivo adjuntado'),
      error: (error: unknown) =>
        this.notify.error(errorMessage(error, 'No se pudo adjuntar el archivo')),
    });
  }

  protected downloadAttachment(attachment: IssueAttachment): void {
    const ticket = this.detail();
    if (!ticket) return;
    this.store.downloadAttachment(ticket.id, attachment).subscribe({
      error: (error: unknown) =>
        this.notify.error(errorMessage(error, 'No se pudo descargar el archivo')),
    });
  }

  protected sizeInKb(bytes: number): string {
    return (bytes / 1024).toFixed(1);
  }

  protected retry(): void {
    this.store.fetchTicket(this.id());
  }

  protected back(): void {
    void this.router.navigateByUrl(ISSUES_PATH);
  }

  protected pathOf(ticket: Ticket): string {
    return issuePath(ticket.id);
  }

  protected isResolved(ticket: Ticket): boolean {
    return ticket.status === 'resolved' || ticket.status === 'closed';
  }

  protected activityText(activity: IssueActivity): string {
    return activity.fieldName ? `${activity.action} - ${activity.fieldName}` : activity.action;
  }

  private settled<T>(state: AsyncState<T>, issueId: Signal<string | null>): boolean {
    return !state.loading() && (issueId() === this.id() || state.error() !== null);
  }

  private otherIssueId(link: IssueLink): string {
    return link.sourceIssueId === this.id() ? link.targetIssueId : link.sourceIssueId;
  }

  private loadCollaboration(id: string): void {
    this.store.fetchComments(id);
    this.store.fetchActivities(id);
    this.store.fetchLinks(id);
    this.store.fetchAttachments(id);
    this.store.fetchWatchers(id);
  }

  private loadMembership(projectId: string): void {
    this.membersRequest?.unsubscribe();
    this.membersRequest = this.projectApi.listMembers(projectId).subscribe({
      next: (members) => this.membership.set({ projectId, members }),
      error: () => this.membership.set({ projectId, members: [] }),
    });
  }
}

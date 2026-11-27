import { DatePipe, NgTemplateOutlet } from '@angular/common';
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
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import {
  AsyncState,
  AuthStore,
  BreakpointService,
  ISSUE_PRIORITY_LABELS,
  ISSUES_PATH,
  ISSUE_STATUSES,
  ISSUE_STATUS_META,
  ISSUE_TYPE_LABELS,
  IssueActivity,
  IssueAttachment,
  IssueComment,
  IssueLink,
  IssueLinkKind,
  IssuePriority,
  IssueStatus,
  NotificationService,
  ProjectApiService,
  ProjectMember,
  ProjectStore,
  Ticket,
  TicketStore,
  UpdateIssueDetailsInput,
  canTransitionTo,
  effectiveProjectRole,
  hasProjectPermission,
  issueKeyOf,
  shortIssueKeyOf,
  issuePath,
} from '@corelog/core';
import { BreadcrumbItem, Breadcrumbs } from '@corelog/ui/breadcrumbs';
import { trackRecent } from '@corelog/ui/command-palette';
import { ErrorState } from '@corelog/ui/error-state';
import { FieldRow } from '@corelog/ui/field-row';
import { InlineEdit, InlineEditOption, InlineEditTone } from '@corelog/ui/inline-edit';
import { IssueKey } from '@corelog/ui/issue-key';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer, PageSidebar } from '@corelog/ui/page-container';
import { Section } from '@corelog/ui/section';
import { SelectField } from '@corelog/ui/select-field';
import { AppTooltip } from '@corelog/ui/tooltip';
import { PickableUser, UserPicker } from '@corelog/ui/user-picker';
import { UserAvatar } from '@corelog/ui/user-avatar';
import { MenuItem } from 'primeng/api';
import { Button } from 'primeng/button';
import { Menu } from 'primeng/menu';
import { Message } from 'primeng/message';
import { Subscription, firstValueFrom, map } from 'rxjs';

import { injectMemberNames } from '../hooks/member-names';
import { injectTeammateSearch } from '../hooks/teammate-search';
import { IssuePicker } from '../issue-picker/issue-picker';
import { errorMessage } from '../shared/error-message';
import { payloadFrom } from '../shared/issue-payload';
import { MentionSearch } from '../shared/mention-search';
import { exactDate, timeAgo } from '../shared/time';
import { TicketActivity } from './ticket-activity';
import { TicketDescriptionSection } from './ticket-description-section';

const COLLABORATION_ERROR = 'No se pudo cargar toda la información de colaboración.';
const NO_PERMISSION_TO_ASSIGN = 'No tienes permiso para cambiar el responsable.';
/** Same breakpoints as the page container: the side panel stacks below 992 px, phones are below 768 px. */
const STACKED_QUERY = '(max-width: 991.98px)';
const PHONE_QUERY = '(max-width: 767.98px)';

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
const PRIORITY_ICONS: Readonly<Record<IssuePriority, string>> = {
  low: 'pi pi-angle-down',
  medium: 'pi pi-equals',
  high: 'pi pi-angle-up',
  critical: 'pi pi-angle-double-up',
};
const STATUS_TONES: Readonly<Record<IssueStatus, InlineEditTone>> = {
  open: 'neutral',
  in_progress: 'progress',
  resolved: 'done',
  closed: 'done',
};

interface ProjectMembership {
  readonly projectId: string;
  readonly members: ProjectMember[];
}

/**
 * Issue detail: breadcrumbs, key and actions in the header, inline-editable title, description,
 * attachments, links and activity in the main column, and a sticky "Detalles" side panel (a collapsible
 * block after the title on tablets and phones). Collaboration data is loaded per issue; a late response
 * for another issue is ignored by the store. A failed load shows one page-level error, never a toast stack.
 */
@Component({
  selector: 'app-ticket-detail-page',
  imports: [
    AppTooltip,
    Breadcrumbs,
    Button,
    DatePipe,
    ErrorState,
    FieldRow,
    FormsModule,
    InlineEdit,
    IssueKey,
    IssuePicker,
    LoadingSkeleton,
    Menu,
    Message,
    NgTemplateOutlet,
    PageContainer,
    PageSidebar,
    RouterLink,
    Section,
    SelectField,
    TicketActivity,
    TicketDescriptionSection,
    UserAvatar,
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
  private readonly projects = inject(ProjectStore);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly breakpoints = inject(BreakpointService);

  protected readonly names = injectMemberNames();
  protected readonly teammates = injectTeammateSearch();
  protected readonly mentionSearch = inject(MentionSearch).search;
  protected readonly nameOf = (userId: string): string => this.names.nameOf(userId);

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
  /** Id of the issue on screen; only changes when another issue is opened, not on every edit. */
  private readonly loadedId = computed(() => this.detail()?.id);
  protected readonly showSkeleton = computed(
    () =>
      this.store.selected.loading() ||
      (!this.store.selected.value() && this.store.selected.error() === null),
  );
  protected readonly showError = computed(() => !this.showSkeleton() && !this.detail());

  /** HTTP status of the failed load; null when the failure has no known cause. */
  protected readonly errorStatus = computed<number | null>(() => {
    const error = this.store.selected.error();
    return error && error.code !== 'unknown' ? error.status : null;
  });
  /** A refused or missing issue does not come back by retrying. */
  protected readonly canRetryLoad = computed(() => {
    const status = this.errorStatus();
    return this.id() !== '' && status !== 401 && status !== 403 && status !== 404;
  });

  protected readonly issueKeyOf = issueKeyOf;
  protected readonly issueTypeLabels = ISSUE_TYPE_LABELS;
  protected readonly priorityOptions = PRIORITY_OPTIONS;
  protected readonly linkKindOptions = LINK_KIND_OPTIONS;
  protected readonly noAssignPermission = NO_PERMISSION_TO_ASSIGN;

  protected readonly statusOptions = computed<InlineEditOption[]>(() => {
    const ticket = this.detail();
    if (!ticket) return [];
    return ISSUE_STATUSES.filter(
      (status) => status === ticket.status || canTransitionTo(ticket.status, status),
    ).map((status) => ({ value: status, label: ISSUE_STATUS_META[status].label }));
  });

  // Layout: details beside the content on desktop, a collapsible block after the title below 992 px.
  protected readonly compact = this.breakpoints.matches(STACKED_QUERY);
  private readonly phone = this.breakpoints.matches(PHONE_QUERY);
  protected readonly detailsOpen = signal(!this.phone());

  protected readonly project = computed(() => {
    const projectId = this.detail()?.projectId;
    return projectId ? this.projects.projects().find((item) => item.id === projectId) : undefined;
  });
  protected readonly projectPath = computed(() => {
    const projectId = this.detail()?.projectId;
    return projectId ? `/projects/${encodeURIComponent(projectId)}/board` : ISSUES_PATH;
  });
  protected readonly breadcrumbs = computed<BreadcrumbItem[]>(() => {
    const ticket = this.detail();
    if (!ticket) return [];
    return [
      { label: this.project()?.name ?? 'Proyecto', to: this.projectPath() },
      { label: this.shortKeyOf(ticket) },
    ];
  });
  protected readonly menuItems = computed<MenuItem[]>(() => {
    const ticket = this.detail();
    if (!ticket) return [];
    return [
      { label: 'Copiar clave', icon: 'pi pi-copy', command: () => void this.copyKey(ticket) },
      {
        label: 'Ver el proyecto',
        icon: 'pi pi-folder-open',
        command: () => void this.router.navigateByUrl(this.projectPath()),
      },
      { separator: true },
      {
        label: 'Volver a incidencias',
        icon: 'pi pi-arrow-left',
        command: () => this.back(),
      },
    ];
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
  protected readonly currentUser = computed(() => {
    const user = this.auth.user();
    return user ? { id: user.id, name: user.name } : null;
  });
  protected readonly canAssignToMe = computed(() => {
    const user = this.auth.user();
    return this.canChangeAssignee() && !!user && this.detail()?.assigneeId !== user.id;
  });
  /**
   * People the picker offers: the organization members (loaded when the page opens, so the list is
   * never empty), plus the remote search results, which add the email, plus the current assignee.
   */
  protected readonly assigneeUsers = computed<PickableUser[]>(() => {
    const byId = new Map<string, PickableUser>();
    for (const member of this.names.members()) byId.set(member.id, { ...member });
    for (const user of this.teammates.results()) {
      byId.set(user.id, { id: user.id, name: user.name, email: user.email });
    }
    const ticket = this.detail();
    if (ticket?.assigneeId && !byId.has(ticket.assigneeId)) {
      byId.set(ticket.assigneeId, {
        id: ticket.assigneeId,
        name: ticket.assigneeName ?? this.names.nameOf(ticket.assigneeId),
      });
    }
    return Array.from(byId.values()).sort((a, b) => a.name.localeCompare(b.name, 'es'));
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

  constructor() {
    effect(() => {
      const id = this.id();
      if (id) untracked(() => this.store.fetchTicket(id));
    });

    // The rest of the page loads once the issue itself is known: a refused issue makes one error, not seven.
    effect(() => {
      const id = this.loadedId();
      if (id) untracked(() => this.loadCollaboration(id));
    });

    effect(() => {
      const projectId = this.projectId();
      if (projectId) untracked(() => this.loadMembership(projectId));
    });

    // The breadcrumb names the project: the projects list is loaded when it is not there yet.
    effect(() => {
      const projectId = this.projectId();
      const known = this.project() !== undefined;
      if (projectId && !known) untracked(() => this.projects.fetchProjects());
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

    effect(() => {
      const project = this.project();
      if (project) {
        trackRecent({
          kind: 'project',
          id: project.id,
          title: project.name,
          subtitle: project.key,
          link: `/projects/${encodeURIComponent(project.id)}/board`,
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

  protected assignToMe(): void {
    this.assign(this.auth.user()?.id);
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

  protected onLinkKind(kind: string | null): void {
    if (kind) this.linkKind.set(kind as IssueLinkKind);
  }

  protected createLink(): void {
    const ticket = this.detail();
    const target = this.linkTarget();
    if (!ticket || !target) return;
    this.store.createLink(ticket.id, target, this.linkKind()).subscribe({
      next: () => {
        this.linkTarget.set('');
        this.notify.success('Vínculo creado');
      },
      error: (error: unknown) =>
        this.notify.error(errorMessage(error, 'No se pudo crear el vínculo')),
    });
  }

  protected removeLink(linkId: string): void {
    const ticket = this.detail();
    if (!ticket) return;
    this.store.deleteLink(ticket.id, linkId).subscribe({
      error: (error: unknown) => this.notify.error(errorMessage(error, 'No se pudo eliminar')),
    });
  }

  protected linkKindLabel(link: IssueLink): string {
    return LINK_KIND_LABELS[link.kind];
  }

  protected linkKey(link: IssueLink): string {
    return shortIssueKeyOf(this.otherIssueId(link));
  }

  protected linkPath(link: IssueLink): string {
    return issuePath(this.otherIssueId(link));
  }

  // Comments. The activity block calls these and reads the result to close its forms.

  protected readonly createComment = async (body: string): Promise<boolean> => {
    const ticket = this.detail();
    if (!ticket) return false;
    try {
      await firstValueFrom(this.store.createComment(ticket.id, body));
    } catch (error) {
      this.notify.error(errorMessage(error, 'No se pudo agregar el comentario'));
      return false;
    }
    this.store.fetchWatchers(ticket.id);
    this.notify.success('Comentario agregado');
    return true;
  };

  protected readonly updateComment = async (commentId: string, body: string): Promise<boolean> => {
    const ticket = this.detail();
    if (!ticket) return false;
    try {
      await firstValueFrom(this.store.updateComment(ticket.id, commentId, body));
    } catch (error) {
      this.notify.error(errorMessage(error, 'No se pudo guardar el comentario'));
      return false;
    }
    this.notify.success('Comentario actualizado');
    return true;
  };

  protected readonly deleteComment = async (commentId: string): Promise<boolean> => {
    const ticket = this.detail();
    if (!ticket) return false;
    try {
      await firstValueFrom(this.store.deleteComment(ticket.id, commentId));
    } catch (error) {
      this.notify.error(errorMessage(error, 'No se pudo eliminar'));
      return false;
    }
    this.notify.success('Comentario eliminado');
    return true;
  };

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

  // Page.

  protected retry(): void {
    this.store.fetchTicket(this.id());
  }

  protected back(): void {
    void this.router.navigateByUrl(ISSUES_PATH);
  }

  protected toggleDetails(): void {
    this.detailsOpen.update((open) => !open);
  }

  protected async copyLink(): Promise<void> {
    await this.copyText(window.location.href, 'Enlace copiado');
  }

  protected async copyKey(ticket: Ticket): Promise<void> {
    await this.copyText(issueKeyOf(ticket.id), 'Clave copiada');
  }

  protected pathOf(ticket: Ticket): string {
    return issuePath(ticket.id);
  }

  protected shortKeyOf(ticket: { readonly id: string }): string {
    return shortIssueKeyOf(ticket.id);
  }

  protected isResolved(ticket: Ticket): boolean {
    return ticket.status === 'resolved' || ticket.status === 'closed';
  }

  protected statusTone(status: IssueStatus): InlineEditTone {
    return STATUS_TONES[status];
  }

  protected priorityIcon(priority: IssuePriority): string {
    return PRIORITY_ICONS[priority];
  }

  protected ago(iso: string): string {
    return timeAgo(iso);
  }

  protected exact(iso: string): string {
    return exactDate(iso);
  }

  private async copyText(text: string, done: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      this.notify.error('No se pudo copiar. Cópialo manualmente.');
      return;
    }
    this.notify.success(done);
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
    this.membersRequest = this.projectApi.listMembers(projectId, { silent: true }).subscribe({
      next: (members) => this.membership.set({ projectId, members }),
      error: () => this.membership.set({ projectId, members: [] }),
    });
  }
}

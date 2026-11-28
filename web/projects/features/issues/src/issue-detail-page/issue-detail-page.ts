import { NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  computed,
  effect,
  inject,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import {
  AuthStore,
  CatalogStore,
  BreakpointService,
  ISSUES_PATH,
  IssueAttachment,
  IssuePermission,
  IssueViewApiService,
  IssueViewStore,
  IssueTransition,
  NotificationService,
  ProjectApiService,
  ProjectMember,
  ProjectStore,
  Issue,
  IssueStore,
  UpdateIssueDetailsInput,
  asApiError,
  transitionErrorMessage,
  effectiveProjectRole,
  hasProjectPermission,
  issueKeyOf,
  issueStatusView,
  shortIssueKeyOf,
  issuePath,
  projectPath,
} from '@corelog/core';
import { ArchivedNotice } from '@corelog/ui/archived-notice';
import { BreadcrumbItem, Breadcrumbs } from '@corelog/ui/breadcrumbs';
import { trackRecent } from '@corelog/ui/command-palette';
import { CatalogIcon } from '@corelog/ui/catalog-icon';
import { ErrorState } from '@corelog/ui/error-state';
import { FieldRow } from '@corelog/ui/field-row';
import { InlineEdit, InlineEditOption } from '@corelog/ui/inline-edit';
import { IssueKey } from '@corelog/ui/issue-key';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer, PageSidebar } from '@corelog/ui/page-container';
import { Section } from '@corelog/ui/section';
import { StatusTag } from '@corelog/ui/status-tag';
import { TransitionButton } from '@corelog/ui/transition-button';
import { TransitionRunner } from '@corelog/ui/transition-dialog';
import { AppTooltip } from '@corelog/ui/tooltip';
import { PickableUser, UserPicker } from '@corelog/ui/user-picker';
import { MenuItem } from 'primeng/api';
import { Button } from 'primeng/button';
import { Menu } from 'primeng/menu';
import { Message } from 'primeng/message';
import { Subscription, firstValueFrom, map } from 'rxjs';

import { injectMemberNames } from '../hooks/member-names';
import { injectTeammateSearch } from '../hooks/teammate-search';
import { errorMessage } from '../shared/error-message';
import { payloadFrom } from '../shared/issue-payload';
import { MentionSearch } from '../shared/mention-search';
import { exactDate, timeAgo } from '../shared/time';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { IssueOperation, IssueOperationDialogs } from './issue-operation-dialogs';
import { IssueConversationSection } from './issue-conversation-section';
import { IssueFieldsPanel } from './issue-fields-panel';
import { IssueRelationsSection } from './issue-relations-section';
import { IssueWorklogsSection } from './issue-worklogs-section';
import { IssueDescriptionSection } from './issue-description-section';
import { IssueNotesSection } from './issue-notes-section';

const ARCHIVED_REASON = 'El proyecto está archivado y es de solo lectura.';
const NO_PERMISSION_TO_ASSIGN = 'No tienes permiso para cambiar el responsable.';
/** Same breakpoints as the page container: the side panel stacks below 992 px, phones are below 768 px. */
const STACKED_QUERY = '(max-width: 991.98px)';
const PHONE_QUERY = '(max-width: 767.98px)';

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
  selector: 'app-issue-detail-page',
  imports: [
    AppTooltip,
    ArchivedNotice,
    Breadcrumbs,
    Button,
    CatalogIcon,
    ErrorState,
    FieldRow,
    InlineEdit,
    IssueKey,
    LoadingSkeleton,
    Menu,
    Message,
    NgTemplateOutlet,
    PageContainer,
    PageSidebar,
    RouterLink,
    Section,
    StatusTag,
    TransitionButton,
    TransitionRunner,
    ConfirmModal,
    IssueConversationSection,
    IssueOperationDialogs,
    IssueFieldsPanel,
    IssueRelationsSection,
    IssueWorklogsSection,
    IssueDescriptionSection,
    IssueNotesSection,
    UserPicker,
  ],
  templateUrl: './issue-detail-page.html',
  styleUrl: './issue-detail-page.scss',
  providers: [IssueViewStore],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueDetailPage {
  private readonly store = inject(IssueStore);
  protected readonly viewStore = inject(IssueViewStore);
  private readonly viewApi = inject(IssueViewApiService);
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

  /** What the route names: the issue key (CORE-12) or its UUID. */
  protected readonly ref = toSignal(
    this.route.paramMap.pipe(map((params) => params.get('issueKey') ?? '')),
    { initialValue: '' },
  );

  /** Issue shown on the page: loaded, and the one the route asks for. */
  protected readonly detail = computed<Issue | undefined>(() => {
    const issue = this.store.selected.value();
    return issue && !this.store.selected.loading() && this.isRequested(issue) ? issue : undefined;
  });
  protected readonly detailItems = computed<Issue[]>(() => {
    const issue = this.detail();
    return issue ? [issue] : [];
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
    return this.ref() !== '' && status !== 401 && status !== 403 && status !== 404;
  });

  protected readonly issueKeyOf = issueKeyOf;
  private readonly catalog = inject(CatalogStore);
  /** Priorities of the organization, as options of the inline editor (the value is the priority id). */
  protected readonly priorityOptions = computed<InlineEditOption[]>(() =>
    this.catalog.priorities().map((item) => ({ value: item.id, label: item.name })),
  );
  protected readonly noAssignPermission = NO_PERMISSION_TO_ASSIGN;

  // Workflow transitions of the issue: loaded again whenever its status changes.
  protected readonly runner = viewChild(TransitionRunner);
  protected readonly transitions = signal<readonly IssueTransition[]>([]);
  protected readonly transitionsLoading = signal(false);
  protected readonly transitionsError = signal<string | null>(null);
  /** The caller may not run transitions at all (read-only roles): the control is not shown. */
  protected readonly transitionsForbidden = signal(false);
  protected readonly transitionBusy = signal(false);
  private transitionsRequest: Subscription | null = null;
  protected readonly currentStatus = computed(() => {
    const issue = this.detail();
    if (!issue) return null;
    return (
      issue.workflowStatus ?? {
        id: '',
        name: issueStatusView(issue).label,
        category:
          issue.status === 'in_progress'
            ? ('in_progress' as const)
            : issue.status === 'open'
              ? ('todo' as const)
              : ('done' as const),
      }
    );
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
    const issue = this.detail();
    if (!issue) return [];
    return [
      { label: this.project()?.name ?? 'Proyecto', to: this.projectPath() },
      { label: this.shortKeyOf(issue) },
    ];
  });
  protected readonly linkRequest = signal(0);
  protected readonly operation = signal<IssueOperation | null>(null);
  protected readonly deleteOpen = signal(false);
  protected readonly deleteBusy = signal(false);
  protected readonly voteBusy = signal(false);
  private readonly fileInput = viewChild<ElementRef<HTMLInputElement>>('fileInput');

  protected readonly canVote = computed(() => !this.archived() && this.viewStore.can('vote_issue'));
  protected readonly menuItems = computed<MenuItem[]>(() => {
    const issue = this.detail();
    if (!issue) return [];
    const writable = !this.archived();
    const can = (permission: IssuePermission): boolean =>
      writable && this.viewStore.can(permission);
    return [
      { label: 'Copiar enlace', icon: 'pi pi-link', command: () => void this.copyLink() },
      { label: 'Copiar clave', icon: 'pi pi-copy', command: () => void this.copyKey(issue) },
      { separator: true },
      {
        label: 'Adjuntar archivo',
        icon: 'pi pi-paperclip',
        visible: can('add_attachment'),
        command: () => this.fileInput()?.nativeElement.click(),
      },
      {
        label: 'Añadir subtarea',
        icon: 'pi pi-sitemap',
        visible: can('edit_issue') && issue.issueType.hierarchyLevel !== -1,
        command: () => this.focusSubtask(),
      },
      {
        label: 'Vincular incidencia',
        icon: 'pi pi-share-alt',
        visible: can('link_issue'),
        command: () => this.linkRequest.update((n) => n + 1),
      },
      {
        label: 'Clonar',
        icon: 'pi pi-clone',
        visible: writable,
        command: () => this.operation.set('clone'),
      },
      {
        label: 'Mover a otro proyecto',
        icon: 'pi pi-arrow-right-arrow-left',
        visible: can('edit_issue'),
        command: () => this.operation.set('move'),
      },
      {
        label: 'Convertir tipo',
        icon: 'pi pi-sync',
        visible: can('edit_issue'),
        command: () => this.operation.set('convert'),
      },
      { separator: true },
      {
        label: 'Ver el proyecto',
        icon: 'pi pi-folder-open',
        command: () => void this.router.navigateByUrl(this.projectPath()),
      },
      {
        label: 'Eliminar incidencia',
        icon: 'pi pi-trash',
        visible: can('delete_issue'),
        command: () => this.deleteOpen.set(true),
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
    const issue = this.detail();
    const loaded = this.membership();
    if (!issue || !loaded || loaded.projectId !== issue.projectId) return false;
    const user = this.auth.user();
    return (
      !this.archived() &&
      hasProjectPermission(effectiveProjectRole(user?.role, user?.id, loaded.members), 'edit_issue')
    );
  });
  /** Whether the project of the issue is archived: the issue can be read but not changed. */
  protected readonly archived = signal(false);
  protected readonly canRestoreProject = computed(() => {
    const loaded = this.membership();
    const user = this.auth.user();
    if (!loaded || loaded.projectId !== this.projectId()) return false;
    return hasProjectPermission(
      effectiveProjectRole(user?.role, user?.id, loaded.members),
      'edit_project',
    );
  });
  /** Whether the caller may change the status of issues in this project (read-only roles may not). */
  protected readonly canTransition = computed(() => {
    const loaded = this.membership();
    const user = this.auth.user();
    if (!loaded || loaded.projectId !== this.projectId()) return false;
    return hasProjectPermission(
      effectiveProjectRole(user?.role, user?.id, loaded.members),
      'move_cards',
    );
  });
  /** Why an archived project blocks every edit. */
  protected readonly archivedReason = ARCHIVED_REASON;
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
    const issue = this.detail();
    if (issue?.assigneeId && !byId.has(issue.assigneeId)) {
      byId.set(issue.assigneeId, {
        id: issue.assigneeId,
        name: issue.assigneeName ?? this.names.nameOf(issue.assigneeId),
      });
    }
    return Array.from(byId.values()).sort((a, b) => a.name.localeCompare(b.name, 'es'));
  });

  // Watchers.
  protected readonly watcherState = computed(() =>
    this.store.watchersIssueId() === this.loadedId() ? this.store.watchers.value() : undefined,
  );
  protected readonly watcherLoading = computed(
    () => this.store.watchersIssueId() !== this.loadedId() && this.store.watchers.error() === null,
  );
  protected readonly watcherError = computed(() => {
    const error = this.store.watchers.error();
    return error ? errorMessage(error, 'No se pudo cargar el seguimiento') : undefined;
  });
  protected readonly watcherCount = computed(() => this.watcherState()?.count ?? 0);
  protected readonly watcherBusy = signal(false);

  // Attachments.
  protected readonly attachmentsLoading = computed(
    () =>
      this.store.attachmentsIssueId() !== this.loadedId() &&
      this.store.attachments.error() === null,
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

  constructor() {
    effect(() => {
      const ref = this.ref();
      if (ref) {
        untracked(() => {
          this.store.fetchIssue(ref);
          void this.viewStore.load(ref);
        });
      }
    });

    // The view store is the source of the editable fields: keep the shared issue in step with it.
    effect(() => {
      const view = this.viewStore.view();
      if (!view) return;
      untracked(() => {
        const current = this.store.selected.value();
        if (current && current.id === view.id && current.updatedAt !== view.updatedAt) {
          this.store.selected.set({ ...current, ...view });
        }
      });
    });

    this.catalog.ensureLoaded();

    // Available transitions: reloaded when the issue changes or moves to another status.
    effect(() => {
      const id = this.loadedId();
      this.detail()?.workflowStatus?.id;
      const allowed = this.canTransition();
      if (id && allowed) untracked(() => this.loadTransitions(id));
      else if (!allowed) untracked(() => this.transitions.set([]));
    });

    // The rest of the page loads once the issue itself is known: a refused issue makes one error, not seven.
    effect(() => {
      const id = this.loadedId();
      if (id) untracked(() => this.loadCollaboration(id));
    });

    effect(() => {
      const projectId = this.projectId();
      if (projectId) {
        untracked(() => {
          this.loadMembership(projectId);
          this.loadArchived(projectId);
        });
      }
    });

    // The breadcrumb names the project: the projects list is loaded when it is not there yet.
    effect(() => {
      const projectId = this.projectId();
      const known = this.project() !== undefined;
      if (projectId && !known) untracked(() => this.projects.fetchProjects());
    });

    effect(() => {
      const issue = this.detail();
      if (issue) {
        trackRecent({
          kind: 'issue',
          id: issue.id,
          title: issue.summary,
          subtitle: issue.key,
          link: issuePath(issue.key),
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
          link: projectPath(project, 'board'),
        });
      }
    });

    this.destroyRef.onDestroy(() => {
      this.transitionsRequest?.unsubscribe();
      this.membersRequest?.unsubscribe();
      this.archivedRequest?.unsubscribe();
    });
  }

  // Inline edits. Arrow properties keep `this` for the inline editors that call them.

  protected readonly saveSummary = async (next: string | null): Promise<void> => {
    const issue = this.detail();
    if (!issue) return;
    const summary = next?.trim();
    if (!summary) throw new Error('El resumen es obligatorio');
    await firstValueFrom(this.store.updateDetails(issue.id, payloadFrom(issue, { summary })));
    this.notify.success('Resumen actualizado');
  };

  protected readonly savePriority = async (next: string | null): Promise<void> => {
    const issue = this.detail();
    if (!issue) return;
    if (!next) throw new Error('La prioridad es obligatoria');
    await firstValueFrom(
      this.store.updateDetails(issue.id, payloadFrom(issue, { priorityId: next })),
    );
    this.notify.success('Prioridad actualizada');
  };

  protected readonly saveDetails = async (input: UpdateIssueDetailsInput): Promise<boolean> => {
    const issue = this.detail();
    if (!issue) return false;
    try {
      await firstValueFrom(this.store.updateDetails(issue.id, input));
      this.notify.success('Incidencia actualizada');
      return true;
    } catch (error) {
      this.notify.error(errorMessage(error, 'No se pudo actualizar la incidencia'));
      return false;
    }
  };

  // Assignee.

  protected assign(assigneeId: string | undefined): void {
    const issue = this.detail();
    if (!issue || !assigneeId || assigneeId === issue.assigneeId) return;
    this.assigning.set(true);
    this.store.assign(issue.id, assigneeId).subscribe({
      next: () => {
        this.assigning.set(false);
        this.store.fetchWatchers(issue.id);
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
    const issue = this.detail();
    if (!issue) return;
    this.assigning.set(true);
    this.store.unassign(issue.id).subscribe({
      next: () => {
        this.assigning.set(false);
        this.store.fetchWatchers(issue.id);
        this.notify.success('Responsable quitado');
        this.assignAnnouncement.set('Sin asignar');
      },
      error: (error: unknown) => {
        this.assigning.set(false);
        this.notify.error(errorMessage(error, 'No se pudo quitar el responsable'));
      },
    });
  }

  // Votes, delete and operations.

  protected async toggleVote(): Promise<void> {
    const view = this.viewStore.view();
    if (!view || this.voteBusy()) return;
    const next = !view.viewerHasVoted;
    this.voteBusy.set(true);
    this.viewStore.setVoted(next);
    try {
      await firstValueFrom(this.viewApi.vote(view.id, next));
    } catch (error) {
      this.viewStore.setVoted(!next);
      this.notify.error(errorMessage(error, 'No se pudo registrar el voto'));
    } finally {
      this.voteBusy.set(false);
    }
  }

  protected async confirmDelete(): Promise<void> {
    const issue = this.detail();
    if (!issue) return;
    this.deleteBusy.set(true);
    try {
      await firstValueFrom(this.viewApi.delete(issue.id));
      this.deleteOpen.set(false);
      this.notify.success('Incidencia eliminada');
      void this.router.navigateByUrl(this.projectPath());
    } catch (error) {
      this.deleteOpen.set(false);
      this.notify.error(errorMessage(error, 'No se pudo eliminar la incidencia'));
    } finally {
      this.deleteBusy.set(false);
    }
  }

  protected onOperationDone(operation: IssueOperation): void {
    if (operation === 'convert') {
      void this.viewStore.refresh();
      const id = this.loadedId();
      if (id) this.store.fetchIssue(id);
    }
  }

  private focusSubtask(): void {
    const input = document.querySelector<HTMLInputElement>(
      '[data-testid="issue-children"] input[name="subtask"]',
    );
    input?.scrollIntoView({ block: 'center' });
    input?.focus();
  }

  // Follow.

  protected toggleWatch(): void {
    const issue = this.detail();
    const state = this.watcherState();
    if (!issue || !state) return;
    this.watcherBusy.set(true);
    const request = state.isWatching ? this.store.unwatch(issue.id) : this.store.watch(issue.id);
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

  // Attachments.

  protected onFileSelected(event: Event): void {
    const input = event.target instanceof HTMLInputElement ? event.target : null;
    const file = input?.files?.[0];
    if (input) input.value = '';
    const issue = this.detail();
    if (!issue || !file) return;
    this.store.uploadAttachment(issue.id, file).subscribe({
      next: () => this.notify.success('Archivo adjuntado'),
      error: (error: unknown) =>
        this.notify.error(errorMessage(error, 'No se pudo adjuntar el archivo')),
    });
  }

  protected downloadAttachment(attachment: IssueAttachment): void {
    const issue = this.detail();
    if (!issue) return;
    this.store.downloadAttachment(issue.id, attachment).subscribe({
      error: (error: unknown) =>
        this.notify.error(errorMessage(error, 'No se pudo descargar el archivo')),
    });
  }

  protected sizeInKb(bytes: number): string {
    return (bytes / 1024).toFixed(1);
  }

  // Page.

  protected retry(): void {
    this.store.fetchIssue(this.ref());
    void this.viewStore.load(this.ref());
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

  protected async copyKey(issue: Issue): Promise<void> {
    await this.copyText(issueKeyOf(issue), 'Clave copiada');
  }

  protected pathOf(issue: Issue): string {
    return issuePath(issue.key);
  }

  protected shortKeyOf(issue: { readonly id: string }): string {
    return shortIssueKeyOf(issue);
  }

  private isRequested(issue: Issue): boolean {
    const ref = this.ref();
    return issue.id === ref || issue.key.toLowerCase() === ref.toLowerCase();
  }

  protected isResolved(issue: Issue): boolean {
    return issue.workflowStatus
      ? issue.workflowStatus.category === 'done'
      : issue.status === 'resolved' || issue.status === 'closed';
  }

  protected onTransitionSelected(transition: IssueTransition): void {
    this.runner()?.run(transition);
  }

  protected onTransitionFinished(): void {
    const id = this.loadedId();
    if (!id) return;
    // The change adds activity and may add a comment; the available transitions depend on the new status.
    void this.viewStore.refresh();
    this.loadTransitions(id);
  }

  protected onTransitionFailed(): void {
    const id = this.loadedId();
    if (!id) return;
    this.store.fetchIssue(id);
    this.loadTransitions(id);
  }

  protected loadTransitions(id: string = this.loadedId() ?? ''): void {
    if (!id) return;
    this.transitionsRequest?.unsubscribe();
    this.transitionsLoading.set(true);
    this.transitionsError.set(null);
    this.transitionsRequest = this.store.listTransitions(id).subscribe({
      next: (items) => {
        this.transitions.set(items);
        this.transitionsForbidden.set(false);
        this.transitionsLoading.set(false);
      },
      error: (raw: unknown) => {
        const error = asApiError(raw);
        this.transitions.set([]);
        this.transitionsLoading.set(false);
        if (error.status === 403) {
          this.transitionsForbidden.set(true);
        } else {
          this.transitionsError.set(transitionErrorMessage(error));
        }
      },
    });
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

  private loadCollaboration(id: string): void {
    this.store.fetchAttachments(id);
    this.store.fetchWatchers(id);
  }

  private archivedRequest: Subscription | undefined;

  private loadArchived(projectId: string): void {
    this.archivedRequest?.unsubscribe();
    this.archived.set(false);
    this.archivedRequest = this.projectApi.get(projectId, { silent: true }).subscribe({
      next: (project) => this.archived.set(!!project.archivedAt),
      error: () => this.archived.set(false),
    });
  }

  private loadMembership(projectId: string): void {
    this.membersRequest?.unsubscribe();
    this.membersRequest = this.projectApi.listMembers(projectId, { silent: true }).subscribe({
      next: (members) => this.membership.set({ projectId, members }),
      error: () => this.membership.set({ projectId, members: [] }),
    });
  }
}

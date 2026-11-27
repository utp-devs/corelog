import { HttpErrorResponse } from '@angular/common/http';
import {
  ChangeDetectionStrategy,
  Component,
  Signal,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { Observable, catchError, map, of, switchMap } from 'rxjs';
import {
  ISSUES_PATH,
  NotificationService,
  ProjectStore,
  StorageService,
  Ticket,
  TicketStore,
  asApiError,
  createIssueDraftForm,
  isApiError,
  issueKeyOf,
  issuePath,
  warnBeforeUnload,
} from '@corelog/core';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { PickableUser } from '@corelog/ui/user-picker';

import { injectMemberNames } from '../hooks/member-names';
import { MentionSearch } from '../shared/mention-search';
import { TicketForm } from '../ticket-form/ticket-form';

const CREATE_FAILED = 'No se pudo crear la incidencia';

/** Storage key of the project an issue was last created in (shared with the quick create dialog). */
const LAST_PROJECT_KEY = 'corelog.create-issue.project';

/** Message of a refused creation: the server message, the error text, or a generic fallback. */
function creationErrorMessage(error: unknown): string {
  if (isApiError(error) || error instanceof HttpErrorResponse) {
    return asApiError(error).message || CREATE_FAILED;
  }
  return error instanceof Error && error.message ? error.message : CREATE_FAILED;
}

/**
 * Page to register a new issue. The project defaults to the last one used. Leaving with typed values asks
 * for confirmation (unsavedChangesGuard on the route). A successful create clears the form before
 * navigating to the new issue, and the success message names its key.
 */
@Component({
  selector: 'app-new-ticket-page',
  imports: [PageContainer, PageHeader, TicketForm],
  templateUrl: './new-ticket-page.html',
  styleUrl: './new-ticket-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NewTicketPage {
  private readonly store = inject(TicketStore);
  private readonly projects = inject(ProjectStore);
  private readonly router = inject(Router);
  private readonly notify = inject(NotificationService);
  private readonly storage = inject(StorageService);
  private readonly mentions = inject(MentionSearch);
  private readonly memberNames = injectMemberNames();

  protected readonly form = createIssueDraftForm();

  /** True while the form holds edits that have not been saved (read by the unsaved-changes guard). */
  readonly dirty: Signal<boolean> = toSignal(this.form.events.pipe(map(() => this.form.dirty)), {
    initialValue: false,
  });

  protected readonly isSubmitting = signal(false);
  protected readonly searchUsers = this.mentions.search;

  protected readonly projectOptions = computed(() =>
    this.projects
      .projects()
      .filter((project) => !project.archivedAt)
      .map((project) => ({ value: project.id, label: `${project.name} (${project.key})` })),
  );
  protected readonly projectsLoading = computed(
    () => this.projects.status() === 'loading' && this.projectOptions().length === 0,
  );
  /** Organization members, ready when the page opens, so the assignee list is never empty. */
  protected readonly assigneeUsers = computed<PickableUser[]>(() =>
    this.memberNames.members().map((member) => ({ id: member.id, name: member.name })),
  );

  constructor() {
    warnBeforeUnload(this.dirty);
    // Loaded from an effect so the store's own session reset (which runs first) cannot wipe the answer.
    effect(() => untracked(() => this.projects.fetchProjects()));

    // Once the projects arrive, the last one used (or the first) is preselected.
    effect(() => {
      const options = this.projectOptions();
      untracked(() => {
        const control = this.form.controls.projectId;
        if (options.length > 0 && !options.some((option) => option.value === control.value)) {
          control.setValue(this.defaultProjectId(options.map((option) => option.value)));
        }
      });
    });
  }

  protected submit(): void {
    if (this.isSubmitting()) return;
    const values = this.form.getRawValue();
    this.isSubmitting.set(true);
    this.store
      .createTicket({
        projectId: values.projectId,
        title: values.title.trim(),
        description: values.description,
        priority: values.priority,
        issueType: values.issueType,
      })
      .pipe(
        switchMap((ticket) =>
          values.assigneeId ? this.assign(ticket, values.assigneeId) : of(ticket),
        ),
      )
      .subscribe({
        next: (ticket) => {
          this.isSubmitting.set(false);
          this.storage.setItem(LAST_PROJECT_KEY, values.projectId);
          this.notify.success(`Incidencia ${issueKeyOf(ticket.id)} creada`);
          // Reset first so the unsaved-changes guard sees a clean form when the navigation starts.
          this.form.reset({
            projectId: values.projectId,
            issueType: 'task',
            title: '',
            description: '',
            priority: 'medium',
            assigneeId: undefined,
          });
          void this.router.navigateByUrl(issuePath(ticket.id));
        },
        error: (error: unknown) => {
          this.isSubmitting.set(false);
          this.notify.error(creationErrorMessage(error));
        },
      });
  }

  protected cancel(): void {
    void this.router.navigateByUrl(ISSUES_PATH);
  }

  /** The issue exists even if the assignee could not be set: say so instead of failing the creation. */
  private assign(ticket: Ticket, assigneeId: string): Observable<Ticket> {
    return this.store.assign(ticket.id, assigneeId).pipe(
      catchError(() => {
        this.notify.info('La incidencia se creó, pero no se pudo asignar.');
        return of(ticket);
      }),
    );
  }

  private defaultProjectId(ids: readonly string[]): string {
    const last = this.storage.getItem(LAST_PROJECT_KEY);
    return last && ids.includes(last) ? last : (ids[0] ?? '');
  }
}

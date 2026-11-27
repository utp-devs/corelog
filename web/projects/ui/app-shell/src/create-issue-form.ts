import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ReactiveFormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import {
  AuthStore,
  CREATABLE_ISSUE_TYPE_OPTIONS,
  ISSUE_DESCRIPTION_MIN_LENGTH,
  ISSUE_PRIORITY_OPTIONS,
  ISSUE_SUMMARY_MAX_LENGTH,
  ISSUE_SUMMARY_MIN_LENGTH,
  MentionUser,
  NotificationService,
  ORGANIZATION_ID_PROVIDER,
  OrganizationApiService,
  OrganizationStore,
  ProjectStore,
  StorageService,
  Issue,
  IssueStore,
  UNSAVED_CANCEL_TEXT,
  UNSAVED_CONFIRM_TEXT,
  UNSAVED_DESCRIPTION,
  UNSAVED_TITLE,
  UserDirectoryApiService,
  createIssueDraftForm,
  issueKeyOf,
  issuePath,
  toMentionUsers,
  toastCopyOf,
} from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { Dialog } from '@corelog/ui/dialog';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { RichTextEditor } from '@corelog/ui/rich-text-editor';
import { PickableUser, UserPicker } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Select } from 'primeng/select';
import { Observable, catchError, firstValueFrom, map, of, switchMap } from 'rxjs';

/** Storage key of the project an issue was last created in. */
export const LAST_CREATE_PROJECT_KEY = 'corelog.create-issue.project';

const PROJECT_IN_URL = /^\/projects\/([^/?#]+)/;
const FILTERABLE_PROJECTS = 8;

/**
 * Body and actions of the quick create-issue dialog. It lives in its own component so the stores it
 * needs are only created when the dialog is opened. The user picks the project (the one in view or the
 * last used by default), type, summary, description, priority and assignee; the issue is created through
 * the issue store, then the dialog offers to open it or to create another one.
 */
@Component({
  selector: 'app-create-issue-form',
  imports: [
    ReactiveFormsModule,
    Button,
    ConfirmModal,
    Dialog,
    FieldControl,
    FieldRow,
    InputText,
    RichTextEditor,
    Select,
    UserPicker,
  ],
  templateUrl: './create-issue-form.html',
  styleUrl: './create-issue-form.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CreateIssueForm {
  private readonly injector = inject(Injector);
  private readonly destroyRef = inject(DestroyRef);
  private readonly router = inject(Router);
  private readonly projects = inject(ProjectStore);
  private readonly issues = inject(IssueStore);
  private readonly notify = inject(NotificationService);
  private readonly auth = inject(AuthStore);
  private readonly organizations = inject(OrganizationStore);
  private readonly organizationApi = inject(OrganizationApiService);
  private readonly directory = inject(UserDirectoryApiService);
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);
  private readonly storage = inject(StorageService);

  /** The dialog asks to close (cancel, Escape, X, or after the user chose what to do next). */
  readonly closed = output<void>();

  protected readonly form = createIssueDraftForm();
  protected readonly typeOptions = CREATABLE_ISSUE_TYPE_OPTIONS;
  protected readonly priorityOptions = ISSUE_PRIORITY_OPTIONS;
  protected readonly summaryMessages = {
    required: 'Escribe un resumen.',
    minlength: `El resumen debe tener al menos ${ISSUE_SUMMARY_MIN_LENGTH} caracteres.`,
    maxlength: `El resumen no puede superar ${ISSUE_SUMMARY_MAX_LENGTH} caracteres.`,
  };
  protected readonly descriptionMessages = {
    required: 'Describe la incidencia.',
    minlength: `La descripción debe tener al menos ${ISSUE_DESCRIPTION_MIN_LENGTH} caracteres.`,
  };
  protected readonly projectMessages = { required: 'Elige un proyecto.' };
  protected readonly summaryMaxLength = ISSUE_SUMMARY_MAX_LENGTH;

  protected readonly unsavedTitle = UNSAVED_TITLE;
  protected readonly unsavedDescription = UNSAVED_DESCRIPTION;
  protected readonly unsavedConfirm = UNSAVED_CONFIRM_TEXT;
  protected readonly unsavedCancel = UNSAVED_CANCEL_TEXT;

  protected readonly submitted = signal(false);
  protected readonly submitting = signal(false);
  protected readonly serverError = signal<string | null>(null);
  /** The issue just created: while it is set, the dialog offers to open it or to create another. */
  protected readonly created = signal<Issue | null>(null);
  protected readonly discardOpen = signal(false);
  protected readonly assigneeCandidates = signal<PickableUser[]>([]);

  protected readonly activeProjects = computed(() =>
    this.projects.projects().filter((project) => !project.archivedAt),
  );
  protected readonly projectOptions = computed(() =>
    this.activeProjects().map((project) => ({
      value: project.id,
      label: `${project.name} (${project.key})`,
    })),
  );
  protected readonly filterProjects = computed(
    () => this.projectOptions().length > FILTERABLE_PROJECTS,
  );
  protected readonly loadingProjects = computed(
    () => this.projects.status() === 'loading' && this.activeProjects().length === 0,
  );
  protected readonly projectsFailed = computed(
    () => this.projects.status() === 'error' && this.activeProjects().length === 0,
  );
  protected readonly noProjects = computed(
    () => !this.loadingProjects() && !this.projectsFailed() && this.activeProjects().length === 0,
  );

  /** Same rule as the server: owners and admins always; members when the organization policy allows it. */
  protected readonly canCreateProject = computed(() => {
    if (this.auth.user()?.role === 'admin') return true;
    const role = this.organizations.currentRole();
    if (role === 'owner' || role === 'admin') return true;
    return (
      role === 'member' &&
      this.organizations.currentOrg()?.settings.whoCanCreateProjects === 'members'
    );
  });

  /** What the dialog shows: the form, or the reason why there is nothing to fill in. */
  protected readonly view = computed<'done' | 'loading' | 'failed' | 'empty' | 'form'>(() => {
    if (this.created()) return 'done';
    if (this.loadingProjects()) return 'loading';
    if (this.projectsFailed()) return 'failed';
    return this.noProjects() ? 'empty' : 'form';
  });

  protected readonly showCancel = computed(
    () => this.view() !== 'done' && (this.view() !== 'empty' || this.canCreateProject()),
  );

  protected readonly heading = computed(() =>
    this.created() ? 'Incidencia creada' : 'Crear incidencia',
  );
  protected readonly createdKey = computed(() => {
    const issue = this.created();
    return issue ? issueKeyOf(issue) : '';
  });

  private readonly summaryField = viewChild<ElementRef<HTMLInputElement>>('summaryField');

  /** Teammates for the @ mention menu of the description. */
  protected readonly searchUsers = (query: string): Promise<MentionUser[]> => {
    const organizationId = this.organizationId();
    if (!organizationId) return Promise.resolve([]);
    return firstValueFrom(
      this.directory
        .searchTeammates(organizationId, query)
        .pipe(map((users) => toMentionUsers(users))),
    );
  };

  constructor() {
    this.reloadProjects();
    this.loadAssignees();
    // The form appears once the projects are known: the summary field takes the focus then.
    effect(() => {
      if (this.view() === 'form') {
        untracked(() =>
          afterNextRender(() => this.summaryField()?.nativeElement.focus(), {
            injector: this.injector,
          }),
        );
      }
    });
    // Once the projects arrive, the one in view (or the last used) is preselected.
    effect(() => {
      const list = this.activeProjects();
      untracked(() => {
        const control = this.form.controls.projectId;
        if (list.length > 0 && !list.some((project) => project.id === control.value)) {
          control.setValue(this.defaultProjectId(list.map((project) => project.id)));
        }
      });
    });
  }

  protected reloadProjects(): void {
    this.projects.fetchProjects();
  }

  /** Cancel, the X, Escape and the backdrop come through here: typed text is not thrown away silently. */
  protected requestClose(): void {
    if (this.form.dirty && !this.created() && !this.submitting()) {
      this.discardOpen.set(true);
    } else {
      this.closed.emit();
    }
  }

  protected discard(): void {
    this.discardOpen.set(false);
    this.closed.emit();
  }

  protected submit(): void {
    this.submitted.set(true);
    this.serverError.set(null);
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    if (this.submitting()) return;

    const values = this.form.getRawValue();
    this.submitting.set(true);
    this.issues
      .createIssue({
        projectId: values.projectId,
        summary: values.summary.trim(),
        description: values.description,
        priority: values.priority,
        issueType: values.issueType,
      })
      .pipe(
        switchMap((issue) =>
          values.assigneeId ? this.assign(issue, values.assigneeId) : of(issue),
        ),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe({
        next: (issue) => {
          this.submitting.set(false);
          this.storage.setItem(LAST_CREATE_PROJECT_KEY, values.projectId);
          this.notify.success(`Incidencia ${issueKeyOf(issue)} creada`);
          this.created.set(issue);
        },
        error: (error: unknown) => {
          this.submitting.set(false);
          this.serverError.set(toastCopyOf(error, 'No se pudo crear la incidencia.').detail);
        },
      });
  }

  protected openCreated(): void {
    const issue = this.created();
    this.closed.emit();
    if (issue) void this.router.navigateByUrl(issuePath(issue.key));
  }

  /** Back to an empty form that keeps the project, type and priority of the last issue. */
  protected createAnother(): void {
    const { projectId, issueType, priority } = this.form.getRawValue();
    this.form.reset({
      projectId,
      issueType,
      priority,
      summary: '',
      description: '',
      assigneeId: undefined,
    });
    this.submitted.set(false);
    this.created.set(null);
    afterNextRender(() => this.summaryField()?.nativeElement.focus(), { injector: this.injector });
  }

  protected goToProjects(): void {
    this.closed.emit();
    void this.router.navigateByUrl('/projects');
  }

  /** The issue exists even if the assignee could not be set: say so instead of failing the creation. */
  private assign(issue: Issue, assigneeId: string): Observable<Issue> {
    return this.issues.assign(issue.id, assigneeId).pipe(
      catchError(() => {
        this.notify.info('La incidencia se creó, pero no se pudo asignar.');
        return of(issue);
      }),
    );
  }

  private defaultProjectId(ids: readonly string[]): string {
    const inView = PROJECT_IN_URL.exec(this.router.url)?.[1];
    const fromUrl = inView ? decodeURIComponent(inView) : null;
    if (fromUrl && ids.includes(fromUrl)) return fromUrl;
    const last = this.storage.getItem(LAST_CREATE_PROJECT_KEY);
    if (last && ids.includes(last)) return last;
    return ids[0] ?? '';
  }

  private loadAssignees(): void {
    const organizationId = this.organizationId();
    if (!organizationId) return;
    this.organizationApi
      .listMembers(organizationId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (members) =>
          this.assigneeCandidates.set(
            members.map((member) => ({
              id: member.userId,
              name: member.displayName || member.name,
              email: member.email,
            })),
          ),
        error: () => this.assigneeCandidates.set([]),
      });
  }
}

import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  OnInit,
  computed,
  inject,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import {
  AuthStore,
  Project,
  ProjectMemberRole,
  ProjectStore,
  leadNameOf,
  projectAccessVariantOf,
  NotificationService,
} from '@corelog/core';
import { CardGrid } from '@corelog/ui/card-grid';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { Lozenge } from '@corelog/ui/lozenge';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { ProjectAvatar } from '@corelog/ui/project-avatar';
import { SearchInput } from '@corelog/ui/search-input';
import { Section } from '@corelog/ui/section';
import { Toolbar, ToolbarLeft } from '@corelog/ui/toolbar';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Select } from 'primeng/select';
import { SelectButton } from 'primeng/selectbutton';
import { Tooltip } from 'primeng/tooltip';

import { failureMessage, inputValue } from '../shared/failure-message';
import { ProjectLeadLookup, projectLeadLookup } from '../shared/project-lead-lookup';
import { ProjectCreateDialog } from './project-create-dialog';
import {
  ArchivedFilter,
  MEMBER_ROLE_OPTIONS,
  TYPE_LABELS,
  VISIBILITY_LABELS,
  VisibilityFilter,
  filterProjects,
} from './projects-page.utils';

const VISIBILITY_OPTIONS: { label: string; value: VisibilityFilter }[] = [
  { label: 'Todos', value: 'all' },
  { label: 'Abiertos', value: 'open' },
  { label: 'Privados', value: 'private' },
];

const ARCHIVED_OPTIONS: { value: ArchivedFilter; label: string }[] = [
  { value: 'active', label: 'Activos' },
  { value: 'archived', label: 'Archivados' },
  { value: 'all', label: 'Todos los estados' },
];

const LABEL_COLOR_OPTIONS = [
  { value: 'blue', label: 'Azul' },
  { value: 'green', label: 'Verde' },
  { value: 'gold', label: 'Dorado' },
  { value: 'red', label: 'Rojo' },
];

const SKELETON_SLOTS = [0, 1, 2];

/** Projects of the active organization: filters, cards, and the members, labels and components of a selected project. */
@Component({
  selector: 'app-projects-page',
  imports: [
    FormsModule,
    Button,
    InputText,
    Select,
    SelectButton,
    Tooltip,
    CardGrid,
    EmptyState,
    ErrorPage,
    ErrorState,
    Lozenge,
    LoadingSkeleton,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    ProjectAvatar,
    SearchInput,
    Section,
    Toolbar,
    ToolbarLeft,
    ProjectCreateDialog,
  ],
  templateUrl: './projects-page.html',
  styleUrl: './projects-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectsPage implements OnInit {
  private readonly store = inject(ProjectStore);
  private readonly auth = inject(AuthStore);
  private readonly router = inject(Router);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly projects = this.store.projects;
  protected readonly status = this.store.status;
  protected readonly error = this.store.error;
  protected readonly members = this.store.members;
  protected readonly labels = this.store.labels;
  protected readonly components = this.store.components;

  protected readonly isAdmin = computed(() => this.auth.user()?.role === 'admin');
  protected readonly accessVariant = computed(() => projectAccessVariantOf(this.error()));
  protected readonly loading = computed(
    () => this.status() === 'loading' && this.projects().length === 0,
  );
  protected readonly errorMessage = computed(
    () => this.error()?.message ?? 'No se pudieron cargar los proyectos.',
  );

  protected readonly query = signal('');
  protected readonly visibility = signal<VisibilityFilter>('all');
  protected readonly archived = signal<ArchivedFilter>('active');
  protected readonly visible = computed(() =>
    filterProjects(this.projects(), this.query(), this.visibility(), this.archived()),
  );

  protected readonly createOpen = signal(false);
  protected readonly selectedProjectId = signal<string | null>(null);
  protected readonly selectedProject = computed(() =>
    this.projects().find((project) => project.id === this.selectedProjectId()),
  );
  protected readonly membersTitle = computed(() => {
    const project = this.selectedProject();
    return project ? `Miembros de ${project.name}` : 'Miembros del proyecto';
  });

  protected readonly memberUserId = signal('');
  protected readonly memberRole = signal<ProjectMemberRole>('contributor');
  protected readonly labelName = signal('');
  protected readonly labelColor = signal('blue');
  protected readonly componentName = signal('');
  protected readonly componentDescription = signal('');

  // All projects of the list belong to the current organization: its members resolve the leads.
  protected readonly lead: ProjectLeadLookup = projectLeadLookup(
    computed(() => this.projects()[0]?.organizationId),
  );

  protected readonly visibilityOptions = VISIBILITY_OPTIONS;
  protected readonly archivedOptions = ARCHIVED_OPTIONS;
  protected readonly roleOptions = MEMBER_ROLE_OPTIONS;
  protected readonly labelColorOptions = LABEL_COLOR_OPTIONS;
  protected readonly skeletonSlots = SKELETON_SLOTS;

  ngOnInit(): void {
    this.store.fetchProjects();
  }

  protected visibilityLabel(project: Project): string {
    return VISIBILITY_LABELS[project.visibility];
  }

  protected typeLabel(project: Project): string {
    return TYPE_LABELS[project.type];
  }

  protected leadName(project: Project): string {
    return leadNameOf(project.leadId, this.lead.members(), this.lead.status());
  }

  protected openCreate(): void {
    if (this.isAdmin()) this.createOpen.set(true);
  }

  protected openBoard(project: Project): void {
    void this.router.navigate(['/projects', project.id, 'board']);
  }

  protected selectProject(project: Project): void {
    this.selectedProjectId.set(project.id);
    this.store.fetchMembers(project.id);
    this.store.fetchLabels(project.id);
    this.store.fetchComponents(project.id);
  }

  protected isSelected(project: Project): boolean {
    return this.selectedProjectId() === project.id;
  }

  protected addMember(): void {
    const projectId = this.selectedProjectId();
    if (!projectId || !this.memberUserId()) return;
    this.store
      .addMember(projectId, this.memberUserId(), this.memberRole())
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.memberUserId.set('');
          this.notify.success('Miembro agregado');
        },
        error: (error: unknown) =>
          this.notify.error(failureMessage(error, 'No se pudo agregar el miembro')),
      });
  }

  protected changeMemberRole(userId: string, role: ProjectMemberRole): void {
    const projectId = this.selectedProjectId();
    if (!projectId) return;
    this.store
      .updateMember(projectId, userId, role)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        error: (error: unknown) =>
          this.notify.error(failureMessage(error, 'No se pudo cambiar el rol')),
      });
  }

  protected removeMember(userId: string): void {
    const projectId = this.selectedProjectId();
    if (!projectId) return;
    this.store
      .removeMember(projectId, userId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        error: (error: unknown) =>
          this.notify.error(failureMessage(error, 'No se pudo eliminar el miembro')),
      });
  }

  protected createLabel(): void {
    const projectId = this.selectedProjectId();
    const name = this.labelName();
    if (!projectId || !name.trim()) return;
    this.store
      .createLabel(projectId, name, this.labelColor())
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => this.labelName.set(''),
        error: (error: unknown) =>
          this.notify.error(failureMessage(error, 'No se pudo crear la etiqueta')),
      });
  }

  protected deleteLabel(labelId: string): void {
    const projectId = this.selectedProjectId();
    if (!projectId) return;
    this.store
      .deleteLabel(projectId, labelId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        error: (error: unknown) =>
          this.notify.error(failureMessage(error, 'No se pudo eliminar la etiqueta')),
      });
  }

  protected createComponent(): void {
    const projectId = this.selectedProjectId();
    const name = this.componentName();
    if (!projectId || !name.trim()) return;
    this.store
      .createComponent(projectId, name, this.componentDescription())
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.componentName.set('');
          this.componentDescription.set('');
        },
        error: (error: unknown) =>
          this.notify.error(failureMessage(error, 'No se pudo crear el componente')),
      });
  }

  protected deleteComponent(componentId: string): void {
    const projectId = this.selectedProjectId();
    if (!projectId) return;
    this.store
      .deleteComponent(projectId, componentId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        error: (error: unknown) =>
          this.notify.error(failureMessage(error, 'No se pudo eliminar el componente')),
      });
  }

  protected onLabelNameInput(event: Event): void {
    this.labelName.set(inputValue(event));
  }

  protected onMemberInput(event: Event): void {
    this.memberUserId.set(inputValue(event));
  }

  protected onComponentNameInput(event: Event): void {
    this.componentName.set(inputValue(event));
  }

  protected onComponentDescriptionInput(event: Event): void {
    this.componentDescription.set(inputValue(event));
  }

  protected retry(): void {
    this.store.fetchProjects();
  }
}

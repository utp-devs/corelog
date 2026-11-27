import { CdkDrag, CdkDragDrop, CdkDragHandle, CdkDropList } from '@angular/cdk/drag-drop';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import {
  FormControl,
  FormGroup,
  FormsModule,
  ReactiveFormsModule,
  Validators,
} from '@angular/forms';
import {
  NotificationService,
  OrganizationStatus,
  OrganizationStore,
  StatusCatalogStore,
  WORKFLOW_CATEGORIES,
  WORKFLOW_CATEGORY_LABELS,
  WorkflowCategory,
  canManageOrg,
  isApiError,
  workflowCategoryVisual,
  workflowErrorMessage,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { Section } from '@corelog/ui/section';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';

const CATEGORY_OPTIONS: readonly SelectFieldOption[] = WORKFLOW_CATEGORIES.map((value) => ({
  value,
  label: WORKFLOW_CATEGORY_LABELS[value],
}));

/**
 * Status catalog of the organization: name and category of every status the project workflows can use.
 * Admins create, rename, recategorize, reorder and delete (moving the issues of a status in use to another
 * one); other members only read. Every refusal of the API is shown in Spanish next to what was attempted.
 */
@Component({
  selector: 'app-org-statuses-page',
  imports: [
    AppTooltip,
    Button,
    CdkDrag,
    CdkDragHandle,
    CdkDropList,
    Dialog,
    EmptyState,
    ErrorState,
    FieldControl,
    FieldRow,
    FormsModule,
    InputText,
    LoadingSkeleton,
    Message,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    ReactiveFormsModule,
    Section,
    SelectField,
  ],
  templateUrl: './org-statuses-page.html',
  styleUrl: './org-statuses-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgStatusesPage {
  private readonly store = inject(StatusCatalogStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly notify = inject(NotificationService);

  protected readonly categoryOptions = CATEGORY_OPTIONS;
  protected readonly labels = WORKFLOW_CATEGORY_LABELS;
  protected readonly visual = workflowCategoryVisual;

  protected readonly statuses = this.store.statuses;
  protected readonly canManage = computed(() => canManageOrg(this.orgs.currentRole()));
  protected readonly subtitle = computed(() => {
    const org = this.orgs.currentOrg();
    return org ? `Estados de ${org.name}` : undefined;
  });
  protected readonly loading = computed(() => this.store.loading() && !this.store.loaded());
  protected readonly loadError = computed(() => (this.store.loaded() ? null : this.store.error()));

  protected readonly busy = signal(false);
  protected readonly listError = signal<string | null>(null);
  protected readonly announcement = signal('');

  protected readonly formOpen = signal(false);
  protected readonly editing = signal<OrganizationStatus | null>(null);
  protected readonly formError = signal<string | null>(null);
  protected readonly submitted = signal(false);
  protected readonly form = new FormGroup({
    name: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.maxLength(60)],
    }),
    category: new FormControl<WorkflowCategory>('todo', { nonNullable: true }),
  });
  protected readonly nameMessages = {
    required: 'Escribe un nombre.',
    maxlength: 'El nombre es demasiado largo.',
  };

  protected readonly deleteOpen = signal(false);
  protected readonly deleting = signal<OrganizationStatus | null>(null);
  protected readonly deleteError = signal<string | null>(null);
  protected readonly deleteNeedsTarget = signal(false);
  protected readonly deleteTarget = signal<string | null>(null);
  protected readonly deleteTargets = computed<SelectFieldOption[]>(() => {
    const target = this.deleting();
    return this.statuses()
      .filter((status) => status.id !== target?.id)
      .map((status) => ({ value: status.id, label: status.name }));
  });

  constructor() {
    effect(() => {
      this.orgs.currentOrgId();
      untracked(() => this.store.ensureLoaded());
    });
  }

  protected retry(): void {
    this.store.refresh();
  }

  protected openCreate(): void {
    this.editing.set(null);
    this.openForm('', 'todo');
  }

  protected openEdit(status: OrganizationStatus): void {
    this.editing.set(status);
    this.openForm(status.name, status.category);
  }

  protected closeForm(): void {
    if (!this.busy()) this.formOpen.set(false);
  }

  protected save(): void {
    this.submitted.set(true);
    this.form.markAllAsTouched();
    if (this.form.invalid || this.busy()) return;
    const value = this.form.getRawValue();
    const name = value.name.trim();
    const item = this.editing();
    this.busy.set(true);
    this.formError.set(null);
    const request = item
      ? this.store.update(item.id, {
          name: name !== item.name ? name : undefined,
          category: value.category !== item.category ? value.category : undefined,
        })
      : this.store.create({ name, category: value.category });
    request.subscribe({
      next: (saved) => {
        this.busy.set(false);
        this.formOpen.set(false);
        this.notify.success(
          item ? `Se guardaron los cambios de ${saved.name}` : `Se creó ${saved.name}`,
        );
      },
      error: (error: unknown) => {
        this.busy.set(false);
        this.formError.set(workflowErrorMessage(error, 'No se pudo guardar. Inténtalo de nuevo.'));
      },
    });
  }

  protected drop(event: CdkDragDrop<unknown>): void {
    this.moveTo(event.previousIndex, event.currentIndex);
  }

  protected move(index: number, delta: -1 | 1): void {
    this.moveTo(index, index + delta);
  }

  protected openDelete(status: OrganizationStatus): void {
    this.deleting.set(status);
    this.deleteError.set(null);
    this.deleteNeedsTarget.set(false);
    this.deleteTarget.set(null);
    this.deleteOpen.set(true);
  }

  protected closeDelete(): void {
    if (!this.busy()) this.deleteOpen.set(false);
  }

  protected canConfirmDelete(): boolean {
    return !this.deleteNeedsTarget() || this.deleteTarget() !== null;
  }

  protected confirmDelete(): void {
    const status = this.deleting();
    if (!status || this.busy() || !this.canConfirmDelete()) return;
    this.busy.set(true);
    this.deleteError.set(null);
    this.store.remove(status.id, this.deleteTarget() ?? undefined).subscribe({
      next: () => {
        this.busy.set(false);
        this.deleteOpen.set(false);
        this.notify.success(`Se eliminó ${status.name}`);
      },
      error: (error: unknown) => {
        this.busy.set(false);
        if (isApiError(error) && error.code === 'workflow.status_in_use') {
          this.deleteNeedsTarget.set(true);
        }
        this.deleteError.set(
          workflowErrorMessage(error, 'No se pudo eliminar. Inténtalo de nuevo.'),
        );
      },
    });
  }

  private openForm(name: string, category: WorkflowCategory): void {
    this.submitted.set(false);
    this.formError.set(null);
    this.form.reset({ name, category });
    this.formOpen.set(true);
  }

  private moveTo(from: number, to: number): void {
    const ids = this.statuses().map((status) => status.id);
    if (from === to || to < 0 || to >= ids.length || this.busy()) return;
    const [moved] = ids.splice(from, 1);
    ids.splice(to, 0, moved);
    this.listError.set(null);
    this.busy.set(true);
    this.store.reorder(ids).subscribe({
      next: () => {
        this.busy.set(false);
        this.announcement.set('Orden actualizado');
      },
      error: (error: unknown) => {
        this.busy.set(false);
        this.listError.set(workflowErrorMessage(error, 'No se pudo cambiar el orden.'));
      },
    });
  }
}

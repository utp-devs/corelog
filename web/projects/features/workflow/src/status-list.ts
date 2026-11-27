import { CdkDrag, CdkDragDrop, CdkDragHandle, CdkDropList } from '@angular/cdk/drag-drop';
import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  DraftStatus,
  OrganizationStatus,
  WORKFLOW_CATEGORIES,
  WORKFLOW_CATEGORY_LABELS,
  WorkflowCategory,
  workflowCategoryVisual,
} from '@corelog/core';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';

export interface StatusMove {
  readonly from: number;
  readonly to: number;
}

export interface NewStatus {
  readonly name: string;
  readonly category: WorkflowCategory;
}

type AddMode = 'catalog' | 'new';

const CATEGORY_OPTIONS: readonly SelectFieldOption[] = WORKFLOW_CATEGORIES.map((value) => ({
  value,
  label: WORKFLOW_CATEGORY_LABELS[value],
}));

/**
 * Statuses of the workflow in order. Rename new statuses inline (saved ones are renamed by organization
 * admins), set the initial one, reorder by dragging or with the buttons, delete and add from the
 * organization catalog or by creating a new one. Read-only without edit rights.
 */
@Component({
  selector: 'app-workflow-status-list',
  imports: [
    AppTooltip,
    Button,
    CdkDrag,
    CdkDragHandle,
    CdkDropList,
    FormsModule,
    InputText,
    SelectField,
  ],
  templateUrl: './status-list.html',
  styleUrl: './status-list.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorkflowStatusList {
  readonly statuses = input.required<readonly DraftStatus[]>();
  readonly initialRef = input('');
  readonly catalog = input<readonly OrganizationStatus[]>([]);
  readonly canEdit = input(false);
  /** Organization admins may rename saved statuses; the rename changes the catalog. */
  readonly canRenameSaved = input(false);
  readonly busy = input(false);

  readonly added = output<string>();
  readonly created = output<NewStatus>();
  readonly renamed = output<{ ref: string; name: string }>();
  readonly categoryChanged = output<{ ref: string; category: WorkflowCategory }>();
  readonly initialChanged = output<string>();
  readonly removed = output<string>();
  readonly moved = output<StatusMove>();

  protected readonly categoryOptions = CATEGORY_OPTIONS;
  protected readonly labels = WORKFLOW_CATEGORY_LABELS;
  protected readonly visual = workflowCategoryVisual;

  protected readonly editing = signal<string | null>(null);
  protected readonly adding = signal<AddMode | null>(null);
  protected readonly newName = signal('');
  protected readonly newCategory = signal<WorkflowCategory>('in_progress');
  protected readonly catalogChoice = signal<string | null>(null);

  protected readonly available = computed<SelectFieldOption[]>(() => {
    const used = new Set(this.statuses().map((status) => status.id));
    return this.catalog()
      .filter((item) => !used.has(item.id))
      .map((item) => ({
        value: item.id,
        label: `${item.name} · ${WORKFLOW_CATEGORY_LABELS[item.category]}`,
      }));
  });

  protected canRename(status: DraftStatus): boolean {
    return this.canEdit() && (status.id === null || this.canRenameSaved());
  }

  protected countText(status: DraftStatus): string {
    if (status.issueCount === 0) return 'Sin incidencias';
    return status.issueCount === 1 ? '1 incidencia' : `${status.issueCount} incidencias`;
  }

  protected startRename(status: DraftStatus): void {
    if (this.canRename(status)) this.editing.set(status.ref);
  }

  protected commitRename(status: DraftStatus, value: string): void {
    this.editing.set(null);
    const name = value.trim();
    if (name && name !== status.name) this.renamed.emit({ ref: status.ref, name });
  }

  protected onRenameKey(event: KeyboardEvent, status: DraftStatus, input: HTMLInputElement): void {
    if (event.key === 'Enter') {
      event.preventDefault();
      this.commitRename(status, input.value);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      this.editing.set(null);
    }
  }

  protected openAdd(mode: AddMode): void {
    this.adding.set(mode);
    this.newName.set('');
    this.catalogChoice.set(null);
  }

  protected cancelAdd(): void {
    this.adding.set(null);
  }

  protected confirmAdd(): void {
    if (this.adding() === 'catalog') {
      const id = this.catalogChoice();
      if (!id) return;
      this.added.emit(id);
    } else {
      const name = this.newName().trim();
      if (!name) return;
      this.created.emit({ name, category: this.newCategory() });
    }
    this.adding.set(null);
  }

  protected drop(event: CdkDragDrop<unknown>): void {
    if (event.previousIndex !== event.currentIndex) {
      this.moved.emit({ from: event.previousIndex, to: event.currentIndex });
    }
  }

  protected move(index: number, delta: -1 | 1): void {
    this.moved.emit({ from: index, to: index + delta });
  }
}

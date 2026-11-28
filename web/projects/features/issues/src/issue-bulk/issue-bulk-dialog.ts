import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  BulkAction,
  BulkFailure,
  CatalogStore,
  IssueViewApiService,
  NotificationService,
  ProjectStore,
  asApiError,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { FieldRow } from '@corelog/ui/field-row';
import { SelectField } from '@corelog/ui/select-field';
import { UserPicker } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { firstValueFrom } from 'rxjs';

import { injectMemberNames } from '../hooks/member-names';
import { injectTeammateSearch } from '../hooks/teammate-search';

/** Operations of the bulk dialog (status changes use the status dialog). */
export type BulkKind = 'assign' | 'set_priority' | 'move' | 'delete';

const HEADINGS: Readonly<Record<BulkKind, string>> = {
  assign: 'Asignar incidencias',
  set_priority: 'Cambiar la prioridad',
  move: 'Mover a otro proyecto',
  delete: 'Eliminar incidencias',
};

const SUBMIT_LABELS: Readonly<Record<BulkKind, string>> = {
  assign: 'Asignar',
  set_priority: 'Cambiar prioridad',
  move: 'Mover',
  delete: 'Eliminar',
};

/** Message of the failure codes the server reports for one issue of a bulk operation. */
export function bulkFailureText(failure: BulkFailure): string {
  switch (failure.code) {
    case 'issue.forbidden':
      return 'No tienes permiso para cambiar esta incidencia.';
    case 'issue.not_found':
      return 'La incidencia ya no existe.';
    case 'issue.label_not_in_project':
      return 'Una etiqueta no existe en el proyecto de esta incidencia.';
    case 'issue.move_mapping_required':
      return 'El proyecto de destino no admite su tipo, prioridad o estado.';
    default:
      return failure.message || 'No se pudo cambiar.';
  }
}

/**
 * Bulk change of the selected issues: assign, priority, move and delete. The server applies the
 * change issue by issue and reports the ones it refused; the dialog stays open on a partial result and
 * lists each failure with the reason, so nothing fails silently.
 */
@Component({
  selector: 'app-issue-bulk-dialog',
  imports: [Button, Dialog, FieldRow, FormsModule, Message, SelectField, UserPicker],
  templateUrl: './issue-bulk-dialog.html',
  styleUrl: './issue-bulk-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueBulkDialog {
  private readonly api = inject(IssueViewApiService);
  private readonly catalog = inject(CatalogStore);
  private readonly projects = inject(ProjectStore);
  private readonly notify = inject(NotificationService);
  protected readonly names = injectMemberNames();
  protected readonly teammates = injectTeammateSearch();

  readonly kind = input<BulkKind | null>(null);
  /** Keys or ids of the selected issues. */
  readonly issues = input.required<readonly string[]>();

  readonly closed = output<void>();
  /** The operation finished for at least one issue. */
  readonly done = output<{ kind: BulkKind; succeeded: number; failed: number }>();

  protected readonly assignee = signal<string | undefined>(undefined);
  protected readonly priority = signal('');
  protected readonly project = signal('');
  protected readonly saving = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly failures = signal<readonly BulkFailure[]>([]);

  protected readonly heading = computed(() => {
    const kind = this.kind();
    return kind ? HEADINGS[kind] : '';
  });
  protected readonly submitLabel = computed(() => {
    const kind = this.kind();
    return kind ? SUBMIT_LABELS[kind] : '';
  });
  protected readonly priorityOptions = computed(() =>
    this.catalog.priorities().map((item) => ({ value: item.id, label: item.name })),
  );
  protected readonly projectOptions = computed(() =>
    this.projects
      .projects()
      .map((item) => ({ value: item.id, label: `${item.name} (${item.key})` })),
  );
  protected readonly users = computed(() => {
    const byId = new Map<string, { id: string; name: string; email?: string }>();
    for (const member of this.names.members()) byId.set(member.id, { ...member });
    for (const user of this.teammates.results()) {
      byId.set(user.id, { id: user.id, name: user.name, email: user.email });
    }
    return Array.from(byId.values());
  });
  protected readonly countText = computed(() => {
    const total = this.issues().length;
    return total === 1 ? '1 incidencia seleccionada' : `${total} incidencias seleccionadas`;
  });
  protected readonly bulkFailureText = bulkFailureText;

  constructor() {
    this.catalog.ensureLoaded();
    effect(() => {
      if (this.kind()) {
        untracked(() => {
          this.assignee.set(undefined);
          this.priority.set('');
          this.project.set('');
          this.error.set(null);
          this.failures.set([]);
          if (this.projects.projects().length === 0) this.projects.fetchProjects();
        });
      }
    });
  }

  protected close(): void {
    if (!this.saving()) this.closed.emit();
  }

  protected async submit(): Promise<void> {
    const kind = this.kind();
    if (!kind) return;
    const request = this.requestOf(kind);
    if (!request) return;
    this.saving.set(true);
    this.error.set(null);
    this.failures.set([]);
    try {
      const result = await firstValueFrom(
        this.api.bulk(this.issues(), request.action, request.params),
      );
      const succeeded = result.succeeded.length;
      const failed = result.failed.length;
      if (failed === 0) {
        this.notify.success(
          succeeded === 1 ? 'Se cambió 1 incidencia' : `Se cambiaron ${succeeded} incidencias`,
        );
        this.done.emit({ kind, succeeded, failed });
        this.closed.emit();
        return;
      }
      this.failures.set(result.failed);
      if (succeeded > 0) this.done.emit({ kind, succeeded, failed });
      this.error.set(
        succeeded > 0
          ? `Se cambiaron ${succeeded} y no se pudieron cambiar ${failed}.`
          : 'No se pudo cambiar ninguna incidencia.',
      );
    } catch (error) {
      this.error.set(asApiError(error).message);
    } finally {
      this.saving.set(false);
    }
  }

  private requestOf(
    kind: BulkKind,
  ): { action: BulkAction; params?: Record<string, unknown> } | null {
    switch (kind) {
      case 'assign':
        return { action: 'assign', params: { assignee_id: this.assignee() ?? null } };
      case 'set_priority':
        if (!this.priority()) {
          this.error.set('Elige una prioridad.');
          return null;
        }
        return { action: 'set_priority', params: { priority_id: this.priority() } };
      case 'move':
        if (!this.project()) {
          this.error.set('Elige el proyecto de destino.');
          return null;
        }
        return { action: 'move', params: { project_id: this.project() } };
      case 'delete':
        return { action: 'delete' };
    }
  }
}

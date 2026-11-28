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
import { Router } from '@angular/router';
import {
  CatalogItem,
  CatalogStore,
  CloneInclude,
  IssueView,
  IssueViewApiService,
  NotificationService,
  ProjectStore,
  asApiError,
  issuePath,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { FieldRow } from '@corelog/ui/field-row';
import { SelectField } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { firstValueFrom } from 'rxjs';

import { IssuePicker } from '../issue-picker/issue-picker';

export type IssueOperation = 'clone' | 'move' | 'convert';

const CLONE_OPTIONS: readonly { value: CloneInclude; label: string }[] = [
  { value: 'subtasks', label: 'Subtareas' },
  { value: 'links', label: 'Vínculos' },
  { value: 'attachments', label: 'Archivos adjuntos' },
  { value: 'watchers', label: 'Seguidores' },
];

const MAPPING_CODE = 'issue.move_mapping_required';

/**
 * Clone, move and convert dialogs of one issue. Clone asks for the title, the target project and what
 * to copy. Move asks for the target project and type and, when the server needs equivalences, shows
 * what is missing and asks for them. Convert changes the type and asks for a parent when needed.
 */
@Component({
  selector: 'app-issue-operation-dialogs',
  imports: [Button, Dialog, FieldRow, FormsModule, IssuePicker, Message, SelectField],
  templateUrl: './issue-operation-dialogs.html',
  styleUrl: './issue-operation-dialogs.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssueOperationDialogs {
  private readonly api = inject(IssueViewApiService);
  private readonly catalog = inject(CatalogStore);
  private readonly projects = inject(ProjectStore);
  private readonly notify = inject(NotificationService);
  private readonly router = inject(Router);

  readonly kind = input<IssueOperation | null>(null);
  readonly issue = input.required<IssueView>();

  readonly closed = output<void>();
  /** The operation finished; the owner reloads the issue. */
  readonly completed = output<IssueOperation>();

  protected readonly cloneOptions = CLONE_OPTIONS;
  protected readonly saving = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly warnings = signal<readonly string[]>([]);

  // Clone.
  protected readonly cloneSummary = signal('');
  protected readonly cloneProject = signal('');
  protected readonly cloneInclude = signal<readonly CloneInclude[]>([]);

  // Move.
  protected readonly moveProject = signal('');
  protected readonly moveType = signal('');
  protected readonly needsMapping = signal(false);
  protected readonly mappingMessages = signal<readonly string[]>([]);
  protected readonly priorityMap = signal('');
  protected readonly targetTypes = signal<readonly CatalogItem[]>([]);

  // Convert.
  protected readonly convertType = signal('');
  protected readonly convertParent = signal('');

  protected readonly projectOptions = computed(() =>
    this.projects.projects().map((project) => ({
      value: project.id,
      label: `${project.name} (${project.key})`,
    })),
  );
  protected readonly otherProjectOptions = computed(() =>
    this.projectOptions().filter((option) => option.value !== this.issue().projectId),
  );
  protected readonly typeOptions = computed(() =>
    this.catalog.issueTypes().map((item) => ({ value: item.id, label: item.name })),
  );
  protected readonly targetTypeOptions = computed(() =>
    this.targetTypes().map((item) => ({ value: item.id, label: item.name })),
  );
  protected readonly priorityOptions = computed(() =>
    this.catalog.priorities().map((item) => ({ value: item.id, label: item.name })),
  );
  protected readonly convertNeedsParent = computed(() => {
    const target = this.catalog.issueTypes().find((item) => item.id === this.convertType());
    return target?.hierarchyLevel === -1;
  });
  protected readonly heading = computed(() => {
    switch (this.kind()) {
      case 'clone':
        return 'Clonar incidencia';
      case 'move':
        return 'Mover incidencia';
      case 'convert':
        return 'Convertir tipo';
      default:
        return '';
    }
  });

  constructor() {
    this.catalog.ensureLoaded();
    effect(() => {
      const kind = this.kind();
      if (kind) untracked(() => this.reset(kind));
    });
    effect(() => {
      const projectId = this.moveProject();
      if (projectId) untracked(() => void this.loadTargetTypes(projectId));
    });
  }

  protected includes(value: CloneInclude): boolean {
    return this.cloneInclude().includes(value);
  }

  protected toggleInclude(value: CloneInclude, on: boolean): void {
    this.cloneInclude.update((list) => (on ? [...list, value] : list.filter((v) => v !== value)));
  }

  protected close(): void {
    if (!this.saving()) this.closed.emit();
  }

  protected async submit(): Promise<void> {
    const kind = this.kind();
    if (!kind) return;
    this.error.set(null);
    this.saving.set(true);
    try {
      if (kind === 'clone') await this.clone();
      else if (kind === 'move') await this.move();
      else await this.convert();
    } catch (error) {
      this.handle(error);
    } finally {
      this.saving.set(false);
    }
  }

  private async clone(): Promise<void> {
    const issue = this.issue();
    const created = await firstValueFrom(
      this.api.clone(issue.id, {
        summary: this.cloneSummary().trim() || undefined,
        projectId: this.cloneProject() || undefined,
        include: this.cloneInclude(),
      }),
    );
    this.notify.success(`Incidencia ${created.key} creada`);
    this.completed.emit('clone');
    this.closed.emit();
    await this.router.navigateByUrl(issuePath(created.key));
  }

  private async move(): Promise<void> {
    const issue = this.issue();
    if (!this.moveProject()) {
      this.error.set('Elige el proyecto de destino.');
      return;
    }
    const mapping = this.needsMapping()
      ? {
          issueType: this.moveType() ? { [issue.issueType.id]: this.moveType() } : undefined,
          priority: this.priorityMap() ? { [issue.priority.id]: this.priorityMap() } : undefined,
        }
      : undefined;
    const result = await firstValueFrom(
      this.api.move(issue.id, {
        projectId: this.moveProject(),
        issueTypeId: this.moveType() || undefined,
        fieldMapping: mapping,
      }),
    );
    if (result.warnings.length > 0) {
      this.notify.info(result.warnings.join(' '));
    }
    this.notify.success(`Incidencia movida a ${result.issue.key}`);
    this.completed.emit('move');
    this.closed.emit();
    await this.router.navigateByUrl(issuePath(result.issue.key));
  }

  private async convert(): Promise<void> {
    const issue = this.issue();
    if (!this.convertType()) {
      this.error.set('Elige el nuevo tipo.');
      return;
    }
    if (this.convertNeedsParent() && !this.convertParent()) {
      this.error.set('Una subtarea necesita una incidencia padre.');
      return;
    }
    await firstValueFrom(
      this.api.convert(issue.id, {
        issueTypeId: this.convertType(),
        parentId: this.convertNeedsParent() ? this.convertParent() : undefined,
      }),
    );
    this.notify.success('Tipo actualizado');
    this.completed.emit('convert');
    this.closed.emit();
  }

  private handle(error: unknown): void {
    const apiError = asApiError(error);
    if (apiError.code === MAPPING_CODE) {
      this.needsMapping.set(true);
      this.mappingMessages.set(apiError.details.map((detail) => detail.message));
      this.error.set(
        'El proyecto de destino no admite todos los valores. Indica sus equivalentes.',
      );
      return;
    }
    this.error.set(apiError.message);
  }

  private reset(kind: IssueOperation): void {
    const issue = this.issue();
    this.error.set(null);
    this.warnings.set([]);
    this.saving.set(false);
    if (kind === 'clone') {
      this.cloneSummary.set(`Copia de ${issue.summary}`.slice(0, 255));
      this.cloneProject.set(issue.projectId);
      this.cloneInclude.set([]);
    } else if (kind === 'move') {
      this.moveProject.set('');
      this.moveType.set('');
      this.priorityMap.set('');
      this.needsMapping.set(false);
      this.mappingMessages.set([]);
      this.targetTypes.set([]);
    } else {
      this.convertType.set('');
      this.convertParent.set('');
    }
    if (this.projects.projects().length === 0) this.projects.fetchProjects();
  }

  private async loadTargetTypes(projectId: string): Promise<void> {
    try {
      const types = await firstValueFrom(this.catalog.projectIssueTypes(projectId));
      this.targetTypes.set(types.items);
    } catch {
      this.targetTypes.set([]);
    }
  }
}

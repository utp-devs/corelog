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
  CurrentProjectStore,
  NotificationService,
  ReleaseComponent,
  ReleaseComponentsApiService,
  asApiError,
  type ComponentAssigneePolicy,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Section, SectionActions } from '@corelog/ui/section';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Select } from 'primeng/select';
import { Textarea } from 'primeng/textarea';
import { firstValueFrom } from 'rxjs';

import { ProjectSettingsContext } from '../project-settings-context';

type LoadStatus = 'loading' | 'ready' | 'error';

const POLICIES: Array<{ value: ComponentAssigneePolicy; label: string }> = [
  { value: 'none', label: 'Sin asignar' },
  { value: 'component_lead', label: 'Responsable del componente' },
  { value: 'project_lead', label: 'Líder del proyecto' },
];

/** Components of the project with their lead and who new issues are assigned to by default. */
@Component({
  selector: 'app-project-components-page',
  imports: [
    Button,
    Dialog,
    ErrorState,
    FormsModule,
    InputText,
    LoadingSkeleton,
    ReactiveFormsModule,
    Section,
    SectionActions,
    Select,
    Textarea,
  ],
  templateUrl: './project-components-page.html',
  styleUrl: './project-components-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectComponentsPage {
  private readonly context = inject(ProjectSettingsContext);
  private readonly current = inject(CurrentProjectStore);
  private readonly api = inject(ReleaseComponentsApiService);
  private readonly notify = inject(NotificationService);

  protected readonly status = signal<LoadStatus>('loading');
  protected readonly components = signal<readonly ReleaseComponent[]>([]);
  protected readonly canManage = computed(
    () => this.current.can('manage_components') && !this.context.archived(),
  );
  protected readonly policies = POLICIES;
  protected readonly leads = computed(() =>
    [...this.context.members()].map((member) => ({
      value: member.userId,
      label: this.context.personOf(member.userId).name,
    })),
  );
  protected readonly busy = signal(false);
  protected readonly error = signal<string | null>(null);

  protected readonly formOpen = signal(false);
  protected readonly editing = signal<ReleaseComponent | null>(null);
  protected readonly leadId = signal<string | null>(null);
  protected readonly policy = signal<ComponentAssigneePolicy>('none');
  protected readonly form = new FormGroup({
    name: new FormControl('', {
      nonNullable: true,
      validators: [Validators.required, Validators.maxLength(80)],
    }),
    description: new FormControl('', { nonNullable: true }),
  });

  protected readonly deleting = signal<ReleaseComponent | null>(null);
  protected readonly moveTo = signal<string | null>(null);
  protected readonly moveTargets = computed(() =>
    this.components()
      .filter((item) => item.id !== this.deleting()?.id)
      .map((item) => ({ value: item.id, label: item.name })),
  );

  constructor() {
    effect(() => {
      const id = this.context.projectId();
      if (id) untracked(() => void this.load(id));
    });
  }

  protected policyLabel(policy: ComponentAssigneePolicy): string {
    return POLICIES.find((item) => item.value === policy)?.label ?? policy;
  }

  protected retry(): void {
    const id = this.context.projectId();
    if (id) void this.load(id);
  }

  protected openForm(component: ReleaseComponent | null = null): void {
    this.editing.set(component);
    this.error.set(null);
    this.form.reset({ name: component?.name ?? '', description: component?.description ?? '' });
    this.leadId.set(component?.leadId ?? null);
    this.policy.set(component?.policy ?? 'none');
    this.formOpen.set(true);
  }

  protected async save(): Promise<void> {
    const projectId = this.context.projectId();
    if (!projectId || this.form.invalid || this.busy()) return;
    if (this.policy() === 'component_lead' && !this.leadId()) {
      this.error.set(
        'Elige un responsable para asignar por defecto al responsable del componente.',
      );
      return;
    }
    const value = this.form.getRawValue();
    const payload = {
      name: value.name.trim(),
      description: value.description.trim(),
      leadId: this.leadId(),
      policy: this.policy(),
    };
    await this.run(async () => {
      const editing = this.editing();
      await firstValueFrom(
        editing
          ? this.api.update(projectId, editing.id, payload)
          : this.api.create(projectId, payload),
      );
      this.formOpen.set(false);
      this.notify.success(editing ? 'Componente actualizado' : 'Componente creado');
    });
  }

  protected openDelete(component: ReleaseComponent): void {
    this.error.set(null);
    this.moveTo.set(null);
    this.deleting.set(component);
  }

  protected async confirmDelete(): Promise<void> {
    const projectId = this.context.projectId();
    const component = this.deleting();
    if (!projectId || !component) return;
    await this.run(async () => {
      await firstValueFrom(this.api.delete(projectId, component.id, this.moveTo() ?? undefined));
      this.deleting.set(null);
      this.notify.success('Componente eliminado');
    });
  }

  private async run(action: () => Promise<void>): Promise<void> {
    this.busy.set(true);
    this.error.set(null);
    try {
      await action();
      const id = this.context.projectId();
      if (id) await this.load(id);
    } catch (error) {
      this.error.set(asApiError(error).message);
    } finally {
      this.busy.set(false);
    }
  }

  private async load(projectId: string): Promise<void> {
    try {
      this.components.set(await firstValueFrom(this.api.list(projectId)));
      this.status.set('ready');
    } catch {
      this.status.set('error');
    }
  }
}

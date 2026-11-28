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
  AsyncState,
  DashboardShare,
  DashboardShareInput,
  DashboardSharePermission,
  DashboardShareType,
  OrganizationStore,
  PeopleApiService,
  ProjectApiService,
  TeamApiService,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { map } from 'rxjs';

const TYPE_OPTIONS: readonly SelectFieldOption[] = [
  { value: 'user', label: 'Una persona' },
  { value: 'team', label: 'Un equipo' },
  { value: 'project', label: 'Un proyecto' },
  { value: 'organization', label: 'Toda la organización' },
];

const PERMISSION_OPTIONS: readonly SelectFieldOption[] = [
  { value: 'view', label: 'Puede ver' },
  { value: 'edit', label: 'Puede editar' },
];

const TYPE_LABELS: Readonly<Record<DashboardShareType, string>> = {
  user: 'Persona',
  team: 'Equipo',
  project: 'Proyecto',
  organization: 'Organización',
};

/** Who can see or edit a dashboard: people, teams, projects or the whole organization. */
@Component({
  selector: 'app-dashboard-share-dialog',
  imports: [Button, Dialog, FormsModule, Message, SelectField],
  templateUrl: './share-dialog.html',
  styleUrl: './share-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DashboardShareDialog {
  private readonly people = inject(PeopleApiService);
  private readonly teamApi = inject(TeamApiService);
  private readonly projectApi = inject(ProjectApiService);
  private readonly organizations = inject(OrganizationStore);

  private readonly members = new AsyncState<SelectFieldOption[]>();
  private readonly teams = new AsyncState<SelectFieldOption[]>();
  private readonly projects = new AsyncState<SelectFieldOption[]>();

  readonly open = input(false);
  readonly dashboardName = input('');
  readonly organizationId = input('');
  readonly shares = input<readonly DashboardShare[]>([]);
  readonly busy = input(false);
  readonly error = input<string | null>(null);

  readonly saved = output<readonly DashboardShareInput[]>();
  readonly closeRequested = output<void>();

  protected readonly typeOptions = TYPE_OPTIONS;
  protected readonly permissionOptions = PERMISSION_OPTIONS;

  protected readonly draft = signal<readonly DashboardShare[]>([]);
  protected readonly newType = signal<DashboardShareType>('user');
  protected readonly newTarget = signal<string | null>(null);
  protected readonly newPermission = signal<DashboardSharePermission>('view');

  protected readonly targetOptions = computed<readonly SelectFieldOption[]>(() => {
    switch (this.newType()) {
      case 'user':
        return this.members.value() ?? [];
      case 'team':
        return this.teams.value() ?? [];
      case 'project':
        return this.projects.value() ?? [];
      default:
        return [];
    }
  });

  protected readonly needsTarget = computed(() => this.newType() !== 'organization');
  protected readonly canAdd = computed(() => !this.needsTarget() || this.newTarget() !== null);
  protected readonly dirty = computed(
    () =>
      JSON.stringify(this.toInputs(this.draft())) !== JSON.stringify(this.toInputs(this.shares())),
  );

  constructor() {
    effect(() => {
      if (!this.open()) return;
      const shares = this.shares();
      untracked(() => {
        this.draft.set([...shares]);
        this.newType.set('user');
        this.newTarget.set(null);
        this.newPermission.set('view');
        this.loadTargets();
      });
    });
  }

  protected typeLabel(type: DashboardShareType): string {
    return TYPE_LABELS[type];
  }

  protected setType(value: string | null): void {
    this.newType.set((value as DashboardShareType | null) ?? 'user');
    this.newTarget.set(null);
  }

  protected setPermissionOf(index: number, value: string | null): void {
    const permission = (value as DashboardSharePermission | null) ?? 'view';
    this.draft.update((list) =>
      list.map((share, at) => (at === index ? { ...share, permission } : share)),
    );
  }

  protected remove(index: number): void {
    this.draft.update((list) => list.filter((_, at) => at !== index));
  }

  protected add(): void {
    if (!this.canAdd()) return;
    const type = this.newType();
    const organization = this.organizations.currentOrg();
    const targetId = type === 'organization' ? this.organizationId() : (this.newTarget() as string);
    const name =
      type === 'organization'
        ? (organization?.name ?? 'Toda la organización')
        : (this.targetOptions().find((option) => option.value === targetId)?.label ?? targetId);
    this.draft.update((list) => [
      ...list.filter((share) => !(share.type === type && share.targetId === targetId)),
      { type, targetId, targetName: name, permission: this.newPermission() },
    ]);
    this.newTarget.set(null);
  }

  protected save(): void {
    this.saved.emit(this.toInputs(this.draft()));
  }

  private toInputs(list: readonly DashboardShare[]): DashboardShareInput[] {
    return list.map((share) => ({
      type: share.type,
      targetId: share.targetId,
      permission: share.permission,
    }));
  }

  private loadTargets(): void {
    const orgId = this.organizations.currentOrg()?.id ?? this.organizationId();
    if (!this.members.hasValue() && orgId) {
      this.members.load(
        this.people.listMembers(orgId).pipe(
          map((list) =>
            list.map((member) => ({
              value: member.userId,
              label: `${member.displayName || member.name} (${member.email})`,
            })),
          ),
        ),
      );
    }
    if (!this.teams.hasValue()) {
      this.teams.load(
        this.teamApi.list().pipe(map((list) => list.map((t) => ({ value: t.id, label: t.name })))),
      );
    }
    if (!this.projects.hasValue()) {
      this.projects.load(
        this.projectApi
          .list()
          .pipe(map((list) => list.map((p) => ({ value: p.id, label: `${p.name} (${p.key})` })))),
      );
    }
  }
}

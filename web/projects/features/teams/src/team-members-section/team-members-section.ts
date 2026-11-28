import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NgTemplateOutlet } from '@angular/common';
import { FormsModule } from '@angular/forms';
import {
  NotificationService,
  ApiError,
  OrganizationStore,
  TeamMemberView,
  TeamRole,
  TeamStore,
  UserDirectoryApiService,
} from '@corelog/core';
import { Button } from 'primeng/button';
import { firstValueFrom } from 'rxjs';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { DataTable, DataTableColumn, DataTableCell } from '@corelog/ui/data-table';
import { EmptyState } from '@corelog/ui/empty-state';
import { Lozenge } from '@corelog/ui/lozenge';
import { PickableUser, UserPicker } from '@corelog/ui/user-picker';
import { Section } from '@corelog/ui/section';
import { SelectField } from '@corelog/ui/select-field';
import { UserChip } from '@corelog/ui/user-chip';

import { memberErrorMessage } from '../shared/team-messages';

const SEARCH_DEBOUNCE_MS = 300;
const READ_ONLY_TEXT = 'Solo lectura: los responsables del equipo gestionan a sus miembros.';

export const TEAM_ROLE_OPTIONS: { value: TeamRole; label: string }[] = [
  { value: 'maintainer', label: 'Responsable' },
  { value: 'member', label: 'Miembro' },
];

export type TeamMembersStatus = 'idle' | 'loading' | 'error';

/**
 * Members tab of the team page: the table of memberships with roles, and adding people from the organization
 * directory. Owners, admins and team maintainers manage members (`canManage`); everybody else reads the table.
 */
@Component({
  selector: 'app-team-members-section',
  imports: [
    FormsModule,
    NgTemplateOutlet,
    Button,
    ConfirmModal,
    DataTable,
    DataTableCell,
    EmptyState,
    Lozenge,
    Section,
    SelectField,
    UserChip,
    UserPicker,
  ],
  templateUrl: './team-members-section.html',
  styleUrl: './team-members-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TeamMembersSection {
  private readonly store = inject(TeamStore);
  private readonly organizations = inject(OrganizationStore);
  private readonly directory = inject(UserDirectoryApiService);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  readonly teamId = input.required<string>();
  readonly members = input.required<TeamMemberView[]>();
  readonly status = input<TeamMembersStatus>('idle');
  readonly error = input<ApiError | null>(null);
  /** Owners, admins and team maintainers manage members; everybody else reads the table. */
  readonly canManage = input(false);
  readonly retry = output<void>();

  protected readonly roleOptions: { value: TeamRole; label: string }[] = TEAM_ROLE_OPTIONS;
  protected readonly readOnlyText = READ_ONLY_TEXT;
  protected readonly columns: DataTableColumn[] = [
    { key: 'person', title: 'Persona', mobilePrimary: true },
    { key: 'role', title: 'Rol', mobilePrimary: true },
    { key: 'actions', title: 'Acciones', align: 'right' },
  ];

  protected readonly pickedId = signal<string | undefined>(undefined);
  protected readonly newRole = signal<TeamRole>('member');
  protected readonly candidates = signal<PickableUser[]>([]);
  protected readonly searching = signal(false);
  protected readonly adding = signal(false);
  /** Member waiting for the confirmation to be removed from the team. */
  protected readonly removing = signal<TeamMemberView | null>(null);
  protected readonly removingBusy = signal(false);

  protected readonly pickable = computed(() => {
    const memberIds = new Set(this.members().map((member) => member.userId));
    return this.candidates().filter((user) => !memberIds.has(user.id));
  });

  protected readonly errorMessage = computed(() => {
    if (this.status() !== 'error') return null;
    const error = this.error();
    if (error?.code === 'team.forbidden')
      return 'No tienes permiso para ver los miembros de este equipo.';
    return error?.message ?? 'No se pudieron cargar los miembros.';
  });

  private searchTimer: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    this.destroyRef.onDestroy(() => clearTimeout(this.searchTimer));
  }

  protected roleLabel(role: TeamRole): string | undefined {
    return TEAM_ROLE_OPTIONS.find((option) => option.value === role)?.label;
  }

  protected handleSearch(term: string): void {
    clearTimeout(this.searchTimer);
    this.searching.set(true);
    this.searchTimer = setTimeout(() => this.runSearch(term.trim()), SEARCH_DEBOUNCE_MS);
  }

  protected async handleAdd(): Promise<void> {
    const userId = this.pickedId();
    if (!userId) return;
    this.adding.set(true);
    try {
      await firstValueFrom(this.store.addMember(this.teamId(), userId, this.newRole()));
      this.notify.success('Miembro agregado');
      this.pickedId.set(undefined);
    } catch (error) {
      this.notify.error(memberErrorMessage(error, 'No se pudo agregar el miembro'));
    } finally {
      this.adding.set(false);
    }
  }

  protected setNewRole(value: string | null): void {
    this.newRole.set(value === 'maintainer' ? 'maintainer' : 'member');
  }

  protected async changeRole(member: TeamMemberView, value: string | null): Promise<void> {
    const role: TeamRole = value === 'maintainer' ? 'maintainer' : 'member';
    if (role === member.role) return;
    try {
      await firstValueFrom(this.store.updateMemberRole(this.teamId(), member.userId, role));
      this.notify.success('Rol actualizado');
    } catch (error) {
      this.notify.error(memberErrorMessage(error, 'No se pudo cambiar el rol'));
    }
  }

  protected askRemove(member: TeamMemberView): void {
    this.removing.set(member);
  }

  protected async confirmRemove(): Promise<void> {
    const member = this.removing();
    if (!member) return;
    this.removingBusy.set(true);
    try {
      await firstValueFrom(this.store.removeMember(this.teamId(), member.userId));
      this.notify.success(`${member.name} ya no pertenece al equipo`);
    } catch (error) {
      this.notify.error(memberErrorMessage(error, 'No se pudo quitar el miembro'));
    } finally {
      this.removingBusy.set(false);
      this.removing.set(null);
    }
  }

  private runSearch(term: string): void {
    const organizationId = this.organizations.currentOrgId();
    if (!organizationId) {
      this.candidates.set([]);
      this.searching.set(false);
      return;
    }
    this.directory
      .searchTeammates(organizationId, term)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (users) =>
          this.candidates.set(
            users.map((user) => ({ id: user.id, name: user.name, email: user.email })),
          ),
        error: () => {
          this.candidates.set([]);
          this.searching.set(false);
        },
        complete: () => this.searching.set(false),
      });
  }
}

import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl, FormsModule, ReactiveFormsModule } from '@angular/forms';
import { AuthStore, NotificationService, ProjectMemberRole } from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { DataTable, DataTableCell, DataTableColumn } from '@corelog/ui/data-table';
import { EmptyState } from '@corelog/ui/empty-state';
import { Lozenge } from '@corelog/ui/lozenge';
import { Section } from '@corelog/ui/section';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { AppTooltip } from '@corelog/ui/tooltip';
import { PickableUser, UserPicker } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { NgTemplateOutlet } from '@angular/common';

import { ProjectSettingsContext } from '../project-settings-context';
import { ROLE_LABELS, ROLE_OPTIONS, settingsErrorMessage } from '../settings-copy';
import { AccessRow, TeamAccessRow, accessRowsOf, teamRowsOf } from './access-rows';
import { ProjectPerson } from './project-person';

const PEOPLE_COLUMNS: DataTableColumn[] = [
  { key: 'person', title: 'Persona' },
  { key: 'source', title: 'Cómo llega' },
  { key: 'role', title: 'Rol' },
  { key: 'actions', title: 'Acciones', align: 'right' },
];

const TEAM_COLUMNS: DataTableColumn[] = [
  { key: 'team', title: 'Equipo' },
  { key: 'role', title: 'Rol' },
  { key: 'actions', title: 'Acciones', align: 'right' },
];

type Removal =
  | { readonly kind: 'person'; readonly row: AccessRow }
  | { readonly kind: 'team'; readonly row: TeamAccessRow };

/** People and teams with access to the project, their roles, and the forms to add more. */
@Component({
  selector: 'app-project-access-page',
  imports: [
    FormsModule,
    ReactiveFormsModule,
    NgTemplateOutlet,
    Button,
    Message,
    Section,
    DataTable,
    DataTableCell,
    EmptyState,
    Lozenge,
    SelectField,
    UserPicker,
    ConfirmModal,
    AppTooltip,
    ProjectPerson,
  ],
  templateUrl: './project-access-page.html',
  styleUrl: './project-access-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectAccessPage {
  protected readonly context = inject(ProjectSettingsContext);
  private readonly auth = inject(AuthStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly peopleColumns = PEOPLE_COLUMNS;
  protected readonly teamColumns = TEAM_COLUMNS;
  protected readonly roleOptions: readonly SelectFieldOption[] = ROLE_OPTIONS;

  protected readonly canManage = this.context.canManageMembersNow;
  protected readonly myId = computed(() => this.auth.user()?.id ?? '');

  protected readonly rows = computed(() =>
    accessRowsOf(this.context.members(), (id) => this.context.personOf(id)),
  );
  protected readonly teamRows = computed(() => teamRowsOf(this.context.teamRoles()));
  protected readonly loading = computed(
    () => this.context.projects.status() === 'loading' && this.context.members().length === 0,
  );

  /** People of the organization that do not have a direct role yet. */
  protected readonly pickableUsers = computed<PickableUser[]>(() => {
    const direct = new Set(
      this.context
        .members()
        .filter((member) => member.viaTeam !== true)
        .map((member) => member.userId),
    );
    return [...this.context.people().values()]
      .filter((person) => !direct.has(person.userId))
      .map((person) => ({ id: person.userId, name: person.name, email: person.email }));
  });

  protected readonly teamOptions = computed<SelectFieldOption[]>(() => {
    const taken = new Set(this.context.teamRoles().map((item) => item.teamId));
    return this.context.teams
      .teams()
      .filter((team) => !taken.has(team.id))
      .map((team) => ({ value: team.id, label: team.name }));
  });

  protected readonly newUserId = signal<string | undefined>(undefined);
  protected readonly newUserRole = new FormControl<ProjectMemberRole>('contributor', {
    nonNullable: true,
  });
  protected readonly newTeamId = signal<string | null>(null);
  protected readonly newTeamRole = new FormControl<ProjectMemberRole>('contributor', {
    nonNullable: true,
  });

  protected readonly busy = signal(false);
  protected readonly failure = signal<string | null>(null);
  protected readonly removing = signal<Removal | null>(null);

  private readonly roleControls = new Map<string, FormControl<ProjectMemberRole>>();

  protected roleLabel(role: ProjectMemberRole): string {
    return ROLE_LABELS[role];
  }

  protected isSelf(row: AccessRow): boolean {
    return row.userId === this.myId();
  }

  protected retry(): void {
    this.context.reloadProjectData();
  }

  /** One select control per row, kept so a refused change can put the previous role back. */
  protected personRoleControl(row: AccessRow): FormControl<ProjectMemberRole> {
    return this.roleControlFor(`u:${row.userId}`, row.directRole ?? row.effectiveRole, (role) =>
      this.changePersonRole(row, role),
    );
  }

  protected teamRoleControl(row: TeamAccessRow): FormControl<ProjectMemberRole> {
    return this.roleControlFor(`t:${row.teamId}`, row.role, (role) =>
      this.changeTeamRole(row, role),
    );
  }

  private roleControlFor(
    key: string,
    current: ProjectMemberRole,
    onChange: (role: ProjectMemberRole) => void,
  ): FormControl<ProjectMemberRole> {
    let control = this.roleControls.get(key);
    if (!control) {
      control = new FormControl<ProjectMemberRole>(current, { nonNullable: true });
      control.valueChanges.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(onChange);
      this.roleControls.set(key, control);
    }
    return control;
  }

  private revert(key: string, role: ProjectMemberRole): void {
    this.roleControls.get(key)?.setValue(role, { emitEvent: false });
  }

  private changePersonRole(row: AccessRow, role: ProjectMemberRole): void {
    const id = this.context.projectId();
    if (!id || role === row.directRole) return;
    this.begin();
    this.context.projects
      .updateMember(id, row.userId, role)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.done(`Rol de ${row.name} actualizado`);
          this.context.reloadProjectData(id);
        },
        error: (error: unknown) => {
          this.revert(`u:${row.userId}`, row.directRole ?? row.effectiveRole);
          this.fail(error, 'No se pudo cambiar el rol.');
        },
      });
  }

  private changeTeamRole(row: TeamAccessRow, role: ProjectMemberRole): void {
    const id = this.context.projectId();
    if (!id || role === row.role) return;
    this.begin();
    this.context.permissions
      .setTeamRole(id, row.teamId, role)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.done(`Rol del equipo ${row.name} actualizado`);
          this.context.reloadProjectData(id);
        },
        error: (error: unknown) => {
          this.revert(`t:${row.teamId}`, row.role);
          this.fail(error, 'No se pudo cambiar el rol del equipo.');
        },
      });
  }

  protected addPerson(): void {
    const id = this.context.projectId();
    const userId = this.newUserId();
    if (!id || !userId) return;
    const name = this.context.personOf(userId).name;
    this.begin();
    this.context.projects
      .addMember(id, userId, this.newUserRole.value)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.newUserId.set(undefined);
          this.done(`${name} ya forma parte del proyecto`);
          this.context.reloadProjectData(id);
        },
        error: (error: unknown) => this.fail(error, 'No se pudo agregar a la persona.'),
      });
  }

  protected addTeam(): void {
    const id = this.context.projectId();
    const teamId = this.newTeamId();
    if (!id || !teamId) return;
    const name = this.teamOptions().find((option) => option.value === teamId)?.label ?? 'El equipo';
    this.begin();
    this.context.permissions
      .setTeamRole(id, teamId, this.newTeamRole.value)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.newTeamId.set(null);
          this.done(`${name} ya tiene acceso al proyecto`);
          this.context.reloadProjectData(id);
        },
        error: (error: unknown) => this.fail(error, 'No se pudo dar acceso al equipo.'),
      });
  }

  protected confirmRemoval(): void {
    const removal = this.removing();
    const id = this.context.projectId();
    if (!removal || !id) return;
    this.begin();
    const request =
      removal.kind === 'person'
        ? this.context.projects.removeMember(id, removal.row.userId)
        : this.context.permissions.removeTeamRole(id, removal.row.teamId);
    const name = removal.row.name;
    request.pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: () => {
        this.removing.set(null);
        this.roleControls.clear();
        this.done(
          removal.kind === 'person' ? `${name} salió del proyecto` : `${name} ya no tiene acceso`,
        );
        this.context.reloadProjectData(id);
      },
      error: (error: unknown) => {
        this.removing.set(null);
        this.fail(error, 'No se pudo quitar el acceso.');
      },
    });
  }

  protected removalTitle(): string {
    const removal = this.removing();
    if (!removal) return '';
    return removal.kind === 'person'
      ? `¿Quitar a ${removal.row.name} del proyecto?`
      : `¿Quitar el acceso del equipo ${removal.row.name}?`;
  }

  protected removalText(): string {
    const removal = this.removing();
    if (!removal) return '';
    return removal.kind === 'person'
      ? 'Perderá el acceso que se le dio directamente. Si pertenece a un equipo con acceso, seguirá entrando por el equipo. Sus incidencias no se borran.'
      : 'Sus integrantes pierden el acceso que les daba el equipo. Quienes tengan un rol propio en el proyecto lo conservan.';
  }

  private begin(): void {
    this.busy.set(true);
    this.failure.set(null);
  }

  private done(message: string): void {
    this.busy.set(false);
    this.notify.success(message);
    // The caller may have changed their own rights (for example by leaving a team).
    this.context.refreshOwnPermissions();
  }

  private fail(error: unknown, fallback: string): void {
    this.busy.set(false);
    this.failure.set(settingsErrorMessage(error, fallback));
  }
}

import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NgTemplateOutlet } from '@angular/common';
import { FormControl, FormsModule, ReactiveFormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { Select } from 'primeng/select';
import { Tooltip } from 'primeng/tooltip';
import {
  AuthStore,
  NotificationService,
  OrganizationAdminStore,
  OrganizationMember,
  OrganizationStore,
  OrgRole,
  ORG_ROLES,
  ROLE_LABELS,
  assignableRoles,
  canManageOrg,
  canRemoveMember,
  canViewMembers,
} from '@corelog/core';
import { BulkAction } from '@corelog/ui/bulk-bar';
import type { LozengeTone } from '@corelog/ui/lozenge';
import { DataTable, DataTableColumn, RowMenuItem } from '@corelog/ui/data-table';
import { DataTableCell } from '@corelog/ui/data-table';
import { EmptyState } from '@corelog/ui/empty-state';
import { Lozenge } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { SearchInput } from '@corelog/ui/search-input';
import { Toolbar, ToolbarLeft } from '@corelog/ui/toolbar';
import { ConfirmDangerModal } from '@corelog/ui/confirm-danger-modal';
import { ConfirmModal } from '@corelog/ui/confirm-modal';

import { adminErrorMessage } from '../shared/admin-error-message';
import { ORG_ROLE_TONES, formatOrgDate } from '../shared/org-format';
import { OrgMemberPerson } from './member-person';

type RoleFilter = OrgRole | 'all';

interface PendingRoleChange {
  readonly member: OrganizationMember;
  readonly role: OrgRole;
}

const ROLE_FILTER_OPTIONS: { label: string; value: RoleFilter }[] = [
  { value: 'all', label: 'Todos los roles' },
  ...ORG_ROLES.map((role) => ({ value: role, label: ROLE_LABELS[role] })),
];

const COLUMNS: DataTableColumn[] = [
  { key: 'name', title: 'Persona', mobilePrimary: true },
  { key: 'jobTitle', title: 'Cargo' },
  { key: 'email', title: 'Correo' },
  { key: 'role', title: 'Rol', mobilePrimary: true },
  { key: 'joinedAt', title: 'Se unió' },
];

function matchesQuery(member: OrganizationMember, query: string): boolean {
  const needle = query.trim().toLocaleLowerCase();
  if (!needle) return true;
  return (
    member.name.toLocaleLowerCase().includes(needle) ||
    member.email.toLocaleLowerCase().includes(needle)
  );
}

/** Members of the active organization. Role changes and removals are confirmed and enforced by the API. */
@Component({
  selector: 'app-org-members-page',
  imports: [
    NgTemplateOutlet,
    Button,
    Message,
    Select,
    Tooltip,
    FormsModule,
    ReactiveFormsModule,
    ConfirmDangerModal,
    ConfirmModal,
    DataTable,
    DataTableCell,
    EmptyState,
    Lozenge,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    SearchInput,
    Toolbar,
    ToolbarLeft,
    OrgMemberPerson,
  ],
  templateUrl: './org-members-page.html',
  styleUrl: './org-members-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgMembersPage {
  private readonly admin = inject(OrganizationAdminStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly auth = inject(AuthStore);
  private readonly notify = inject(NotificationService);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly org = this.orgs.currentOrg;
  protected readonly role = this.orgs.currentRole;
  protected readonly orgId = computed(() => this.org()?.id ?? null);
  protected readonly canManage = computed(() => canManageOrg(this.role()));
  protected readonly canView = computed(() => canViewMembers(this.role()));
  protected readonly subtitle = computed(() => {
    const current = this.org();
    return current ? `Personas de ${current.name}` : undefined;
  });

  protected readonly members = this.admin.members;
  protected readonly status = this.admin.membersStatus;
  protected readonly loadError = this.admin.membersError;

  protected readonly query = signal('');
  protected readonly roleFilter = signal<RoleFilter>('all');
  protected readonly selected = signal<string[]>([]);
  protected readonly pendingRole = signal<PendingRoleChange | null>(null);
  protected readonly roleError = signal<string | null>(null);
  protected readonly removing = signal<OrganizationMember | null>(null);
  protected readonly bulkOpen = signal(false);
  protected readonly busy = signal(false);

  protected readonly roleFilterOptions = ROLE_FILTER_OPTIONS;
  protected readonly columns = COLUMNS;
  protected readonly formatDate = formatOrgDate;

  protected readonly isSelf = (member: OrganizationMember): boolean =>
    member.userId === this.auth.user()?.id;

  protected readonly removable = computed(() =>
    this.members().filter((member) =>
      canRemoveMember(this.role(), member.role, this.isSelf(member)),
    ),
  );
  protected readonly removableIds = computed(() => new Set(this.removable().map((m) => m.userId)));
  protected readonly selectable = computed(() => this.canManage() && this.removable().length > 0);
  protected readonly selectedRemovable = computed(() =>
    this.selected().filter((id) => this.removableIds().has(id)),
  );

  protected readonly visible = computed(() =>
    this.members().filter(
      (member) =>
        (this.roleFilter() === 'all' || member.role === this.roleFilter()) &&
        matchesQuery(member, this.query()),
    ),
  );

  protected readonly loading = computed(() => {
    const status = this.status();
    return this.orgId() !== null && (status === 'idle' || status === 'loading');
  });
  protected readonly error = computed(() => {
    const failure = this.loadError();
    return this.orgId() && this.status() === 'error'
      ? (failure?.message ?? 'No se pudo cargar el directorio.')
      : null;
  });

  protected readonly bulkActions: BulkAction[] = [
    {
      key: 'remove',
      label: 'Quitar miembros',
      danger: true,
      onClick: () => this.bulkOpen.set(true),
    },
  ];

  /** One select control per member row; the pending choice stays in it until the change is confirmed. */
  private readonly roleControls = new Map<string, FormControl<OrgRole>>();

  constructor() {
    effect(() => {
      const id = this.orgId();
      const view = this.canView();
      if (id && view) untracked(() => this.load());
    });
  }

  protected load(): void {
    const id = this.orgId();
    if (!id) return;
    this.roleControls.clear();
    this.admin.fetchMembers(id);
  }

  protected rowActionsFor = (member: OrganizationMember): RowMenuItem[] => {
    if (!this.canManage() || !canRemoveMember(this.role(), member.role, this.isSelf(member))) {
      return [];
    }
    return [
      {
        key: 'remove',
        label: 'Quitar de la organización',
        danger: true,
        onClick: () => this.removing.set(member),
      },
    ];
  };

  protected roleTone(role: OrgRole): LozengeTone {
    return ORG_ROLE_TONES[role];
  }

  protected roleLabel(role: OrgRole): string {
    return ROLE_LABELS[role];
  }

  protected canChangeRole(member: OrganizationMember): boolean {
    return this.canManage() && this.roleOptions(member).length > 0;
  }

  protected roleOptions(member: OrganizationMember): { value: OrgRole; label: string }[] {
    return assignableRoles(this.role(), member.role, this.isSelf(member)).map((item) => ({
      value: item,
      label: ROLE_LABELS[item],
    }));
  }

  /** Why the role cannot be changed; null when no explanation is needed. */
  protected roleReason(member: OrganizationMember): string | null {
    const self = this.isSelf(member);
    if (this.canManage() || self) {
      if (self) return 'No puedes cambiar tu propio rol.';
      if (this.role() === 'admin') return 'Solo los propietarios pueden cambiar este rol.';
      return 'No tienes permiso para cambiar este rol.';
    }
    return null;
  }

  protected roleControl(member: OrganizationMember): FormControl<OrgRole> {
    let control = this.roleControls.get(member.userId);
    if (!control) {
      const userId = member.userId;
      control = new FormControl<OrgRole>(member.role, { nonNullable: true });
      control.valueChanges.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((next) => {
        this.onRoleSelected(userId, next);
      });
      this.roleControls.set(userId, control);
    }
    return control;
  }

  protected changeRole(): void {
    const pending = this.pendingRole();
    const id = this.orgId();
    if (!pending || !id) return;
    this.busy.set(true);
    this.roleError.set(null);
    this.admin
      .changeMemberRole(id, pending.member.userId, pending.role)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.busy.set(false);
          this.pendingRole.set(null);
          this.roleControls
            .get(pending.member.userId)
            ?.setValue(pending.role, { emitEvent: false });
          this.notify.success(`Rol de ${pending.member.name} actualizado`);
        },
        error: (error: unknown) => {
          this.busy.set(false);
          this.pendingRole.set(null);
          this.revertRole(pending.member);
          this.roleError.set(adminErrorMessage(error, 'No se pudo cambiar el rol.'));
        },
      });
  }

  protected cancelRoleChange(): void {
    const pending = this.pendingRole();
    this.pendingRole.set(null);
    if (pending) this.revertRole(pending.member);
  }

  protected removeOne(): void {
    const target = this.removing();
    const id = this.orgId();
    if (!target || !id) return;
    this.busy.set(true);
    this.admin
      .removeMember(id, target.userId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.busy.set(false);
          this.removing.set(null);
          this.notify.success(`${target.name} ya no es miembro de la organización`);
        },
        error: (error: unknown) => {
          this.busy.set(false);
          this.removing.set(null);
          this.notify.error(adminErrorMessage(error, 'No se pudo quitar al miembro.'));
        },
      });
  }

  protected async removeSelected(): Promise<void> {
    const id = this.orgId();
    if (!id) return;
    this.busy.set(true);
    const keys = this.selectedRemovable();
    const targets = this.members().filter((member) => keys.includes(member.userId));
    let failures = 0;
    for (const target of targets) {
      try {
        await firstValueFrom(this.admin.removeMember(id, target.userId));
      } catch {
        failures += 1;
      }
    }
    this.busy.set(false);
    this.bulkOpen.set(false);
    this.selected.set([]);
    if (failures === 0) {
      this.notify.success(`${targets.length} miembros quitados`);
    } else {
      this.notify.error(`No se pudieron quitar ${failures} de ${targets.length} miembros.`);
    }
  }

  protected roleChangeText(): string | undefined {
    const pending = this.pendingRole();
    if (!pending) return undefined;
    const ownerNote =
      pending.role === 'owner' ? ' Un propietario controla toda la organización.' : '';
    const member = pending.member.name;
    const from = ROLE_LABELS[pending.member.role];
    const to = ROLE_LABELS[pending.role];
    return `¿Cambiar el rol de ${member} de ${from} a ${to}?${ownerNote}`;
  }

  protected goToInvitations(): void {
    void this.router.navigate(['/organization/invitations']);
  }

  private onRoleSelected(userId: string, next: OrgRole): void {
    const member = this.members().find((item) => item.userId === userId);
    if (!member || next === member.role) return;
    this.pendingRole.set({ member, role: next });
  }

  private revertRole(member: OrganizationMember): void {
    this.roleControls.get(member.userId)?.setValue(member.role, { emitEvent: false });
  }
}

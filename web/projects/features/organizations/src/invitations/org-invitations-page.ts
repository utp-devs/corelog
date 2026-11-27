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
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { FormControl, FormGroup, FormsModule, ReactiveFormsModule } from '@angular/forms';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { SelectButton } from 'primeng/selectbutton';
import { map } from 'rxjs';
import {
  Invitation,
  InvitationScope,
  InvitationStatus,
  NotificationService,
  OrganizationAdminStore,
  OrganizationStore,
  ROLE_LABELS,
  canManageOrg,
  invitableRoles,
} from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { DataTable, DataTableColumn, RowMenuItem, DataTableCell } from '@corelog/ui/data-table';
import { EmptyState } from '@corelog/ui/empty-state';
import { FieldRow } from '@corelog/ui/field-row';
import { Lozenge, LozengeTone } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { Section } from '@corelog/ui/section';
import { SelectField } from '@corelog/ui/select-field';

import { adminErrorMessage } from '../shared/admin-error-message';
import { fieldMessage } from '../shared/field-message';
import { formatOrgDate } from '../shared/org-format';
import { InviteEmailsInput } from './invite-emails-input';
import { InviteLinksPanel } from './invite-links-panel';
import {
  InvitableRole,
  MAX_INVITATIONS_PER_REQUEST,
  inviteEmailsValidator,
  inviteItemsOf,
  parseInviteEmails,
} from './invite-form';

const STATUS_LABELS: Record<InvitationStatus, string> = {
  pending: 'Pendiente',
  accepted: 'Aceptada',
  revoked: 'Revocada',
  expired: 'Caducada',
};

const STATUS_TONES: Record<InvitationStatus, LozengeTone> = {
  pending: 'info',
  accepted: 'success',
  revoked: 'danger',
  expired: 'warning',
};

const SCOPE_OPTIONS: { label: string; value: InvitationScope }[] = [
  { label: 'Pendientes', value: 'pending' },
  { label: 'Todas', value: 'all' },
];

const COLUMNS: DataTableColumn[] = [
  { key: 'email', title: 'Correo', mobilePrimary: true },
  { key: 'role', title: 'Rol' },
  { key: 'status', title: 'Estado', mobilePrimary: true },
  { key: 'expiresAt', title: 'Caduca' },
  { key: 'invitedBy', title: 'Invitada por' },
];

/** Invitations of the active organization. The plain link is shown once, right after creation or resend. */
@Component({
  selector: 'app-org-invitations-page',
  imports: [
    Button,
    Message,
    SelectField,
    SelectButton,
    FormsModule,
    ReactiveFormsModule,
    ConfirmModal,
    DataTable,
    DataTableCell,
    EmptyState,
    FieldRow,
    Lozenge,
    PageContainer,
    PageHeader,
    Section,
    InviteEmailsInput,
    InviteLinksPanel,
  ],
  templateUrl: './org-invitations-page.html',
  styleUrl: './org-invitations-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgInvitationsPage {
  private readonly admin = inject(OrganizationAdminStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly org = this.orgs.currentOrg;
  protected readonly role = this.orgs.currentRole;
  protected readonly orgId = computed(() => this.org()?.id ?? null);
  protected readonly canManage = computed(() => canManageOrg(this.role()));
  protected readonly subtitle = computed(() => {
    const current = this.org();
    return current ? `Personas invitadas a ${current.name}` : undefined;
  });

  protected readonly invitations = this.admin.invitations;
  protected readonly scope = this.admin.invitationScope;
  protected readonly status = this.admin.invitationsStatus;
  protected readonly loadError = this.admin.invitationsError;
  protected readonly members = this.admin.members;
  protected readonly created = this.admin.createdInvitations;

  protected readonly submitError = signal<string | null>(null);
  protected readonly revoking = signal<Invitation | null>(null);
  protected readonly busy = signal(false);
  protected readonly submitted = signal(false);

  protected readonly columns = COLUMNS;
  protected readonly scopeOptions = SCOPE_OPTIONS;
  protected readonly maxInvitations = MAX_INVITATIONS_PER_REQUEST;
  protected readonly statusLabels = STATUS_LABELS;
  protected readonly formatDate = formatOrgDate;

  protected readonly roleOptions = computed(() =>
    invitableRoles(this.role()).map((item) => ({ value: item, label: ROLE_LABELS[item] })),
  );

  protected readonly form = new FormGroup({
    emails: new FormControl('', { nonNullable: true, validators: [inviteEmailsValidator] }),
    role: new FormControl<InvitableRole>('member', { nonNullable: true }),
  });

  private readonly emailsText = toSignal(
    this.form.controls.emails.valueChanges.pipe(map((value) => value ?? '')),
    { initialValue: '' },
  );
  protected readonly parsed = computed(() => parseInviteEmails(this.emailsText()));

  protected readonly loading = computed(() => {
    const state = this.status();
    return this.orgId() !== null && (state === 'idle' || state === 'loading');
  });
  protected readonly error = computed(() => {
    const failure = this.loadError();
    return this.orgId() && this.status() === 'error'
      ? (failure?.message ?? 'No se pudieron cargar las invitaciones.')
      : null;
  });

  constructor() {
    effect(() => {
      const id = this.orgId();
      const manage = this.canManage();
      if (id && manage) untracked(() => this.load());
    });
  }

  /** Field error: the invalid entries first (live), then the rule message after a submit. */
  protected emailsError(): string | undefined {
    const parsed = this.parsed();
    if (parsed.invalid.length > 0) return `No válidos: ${parsed.invalid.join(', ')}`;
    return this.submitted() ? (fieldMessage(this.form.controls.emails) ?? undefined) : undefined;
  }

  protected emailsHelper(): string {
    const parsed = this.parsed();
    if (parsed.duplicates.length > 0) {
      return `Se omitirán los repetidos: ${parsed.duplicates.join(', ')}`;
    }
    const count = parsed.emails.length;
    if (count === 0) return 'Aún no hay correos listos para invitar';
    return count === 1 ? '1 correo listo para invitar' : `${count} correos listos para invitar`;
  }

  protected load(next?: InvitationScope): void {
    const id = this.orgId();
    if (!id || !this.canManage()) return;
    this.admin.fetchInvitations(id, next);
    this.admin.fetchMembers(id);
  }

  protected onScope(value: InvitationScope): void {
    this.load(value);
  }

  protected submit(): void {
    const id = this.orgId();
    this.submitted.set(true);
    this.form.markAllAsTouched();
    if (!id || this.form.invalid) return;

    this.submitError.set(null);
    const values = this.form.getRawValue();
    const items = inviteItemsOf(values);
    this.admin
      .createInvitations(id, items)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.form.reset({ emails: '', role: values.role });
          this.submitted.set(false);
          this.notify.success(
            items.length === 1 ? 'Invitación creada' : `${items.length} invitaciones creadas`,
          );
        },
        error: (error: unknown) => {
          this.submitError.set(adminErrorMessage(error, 'No se pudieron enviar las invitaciones.'));
        },
      });
  }

  protected resend(invitation: Invitation): void {
    const id = this.orgId();
    if (!id) return;
    this.admin
      .resendInvitation(id, invitation.id)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => this.notify.success('Nuevo enlace generado. El anterior ya no funciona.'),
        error: (error: unknown) =>
          this.notify.error(adminErrorMessage(error, 'No se pudo reenviar la invitación.')),
      });
  }

  protected revoke(): void {
    const target = this.revoking();
    const id = this.orgId();
    if (!target || !id) return;
    this.busy.set(true);
    this.admin
      .revokeInvitation(id, target.id)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.busy.set(false);
          this.revoking.set(null);
          this.notify.success('Invitación revocada');
        },
        error: (error: unknown) => {
          this.busy.set(false);
          this.revoking.set(null);
          this.notify.error(adminErrorMessage(error, 'No se pudo revocar la invitación.'));
        },
      });
  }

  protected rowActionsFor = (invitation: Invitation): RowMenuItem[] => {
    if (invitation.status !== 'pending') return [];
    return [
      { key: 'resend', label: 'Reenviar (nuevo enlace)', onClick: () => this.resend(invitation) },
      {
        key: 'revoke',
        label: 'Revocar',
        danger: true,
        onClick: () => this.revoking.set(invitation),
      },
    ];
  };

  protected invitedByName(userId: string | null): string {
    if (!userId) return '—';
    return this.members().find((member) => member.userId === userId)?.name ?? '—';
  }

  protected roleLabel(role: InvitableRole | Invitation['role']): string {
    return ROLE_LABELS[role];
  }

  protected statusLabel(status: InvitationStatus): string {
    return STATUS_LABELS[status];
  }

  protected statusTone(status: InvitationStatus): LozengeTone {
    return STATUS_TONES[status];
  }

  protected dismissLinks(): void {
    this.admin.clearCreatedInvitations();
  }
}

import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { Button } from 'primeng/button';
import {
  AuthStore,
  LOCATION_ASSIGN,
  NotificationService,
  Organization,
  OrganizationAdminStore,
  OrganizationMember,
  OrganizationStore,
  OrgRole,
  canUseOwnerDangerZone,
} from '@corelog/core';
import { ConfirmDangerModal } from '@corelog/ui/confirm-danger-modal';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { Section } from '@corelog/ui/section';
import { SelectField } from '@corelog/ui/select-field';

import { adminErrorMessage, isLastOwnerError } from '../shared/admin-error-message';

/** Danger zone: leave for everyone, transfer ownership and delete for owners only. */
@Component({
  selector: 'app-org-danger-zone',
  imports: [Button, SelectField, FormsModule, ConfirmDangerModal, ConfirmModal, Section],
  templateUrl: './org-danger-zone.html',
  styleUrl: './org-danger-zone.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgDangerZone {
  private readonly admin = inject(OrganizationAdminStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly auth = inject(AuthStore);
  private readonly notify = inject(NotificationService);
  /** Full-page move to the portal home: the portal is a separate application. */
  private readonly assign = inject(LOCATION_ASSIGN);
  private readonly destroyRef = inject(DestroyRef);

  readonly org = input.required<Organization>();
  readonly role = input<OrgRole | null>(null);
  /** Members of the organization; only owners need them (transfer target list). */
  readonly members = input<readonly OrganizationMember[]>([]);

  protected readonly isOwner = computed(() => canUseOwnerDangerZone(this.role()));
  protected readonly leaveOpen = signal(false);
  protected readonly transferOpen = signal(false);
  protected readonly deleteOpen = signal(false);
  protected readonly transferTarget = signal<string | null>(null);
  protected readonly busy = signal(false);

  protected readonly transferCandidates = computed(() => {
    const me = this.auth.user()?.id ?? null;
    return this.members().filter((member) => member.userId !== me);
  });
  readonly transferOptions = computed(() =>
    this.transferCandidates().map((member) => ({
      value: member.userId,
      label: `${member.name} (${member.email})`,
    })),
  );
  protected readonly selectedTarget = computed(
    () =>
      this.transferCandidates().find((member) => member.userId === this.transferTarget()) ?? null,
  );

  protected leave(): void {
    const org = this.org();
    this.busy.set(true);
    this.admin
      .leaveOrganization(org.id)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.busy.set(false);
          this.leaveOpen.set(false);
          this.orgs.fetchOrgs();
          this.notify.success(`Has salido de ${org.name}`);
          this.assign('/');
        },
        error: (error: unknown) => {
          this.busy.set(false);
          this.leaveOpen.set(false);
          this.notify.error(
            isLastOwnerError(error)
              ? 'No puedes salir: eres el último propietario. Transfiere la propiedad antes de salir.'
              : adminErrorMessage(error, 'No se pudo salir de la organización.'),
          );
        },
      });
  }

  protected transfer(): void {
    const target = this.selectedTarget();
    if (!target) return;
    this.busy.set(true);
    this.admin
      .transferOwnership(this.org().id, target.userId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.busy.set(false);
          this.transferOpen.set(false);
          this.transferTarget.set(null);
          this.orgs.fetchOrgs();
          this.notify.success(`Propiedad transferida a ${target.name}. Ahora eres administrador.`);
        },
        error: (error: unknown) => {
          this.busy.set(false);
          this.transferOpen.set(false);
          this.notify.error(adminErrorMessage(error, 'No se pudo transferir la propiedad.'));
        },
      });
  }

  protected remove(): void {
    const org = this.org();
    this.busy.set(true);
    this.admin
      .deleteOrganization(org.id, org.slug)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.busy.set(false);
          this.deleteOpen.set(false);
          this.orgs.fetchOrgs();
          this.notify.success('Organización eliminada');
          this.assign('/');
        },
        error: (error: unknown) => {
          this.busy.set(false);
          this.deleteOpen.set(false);
          this.notify.error(adminErrorMessage(error, 'No se pudo eliminar la organización.'));
        },
      });
  }
}

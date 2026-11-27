import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import { firstValueFrom, map } from 'rxjs';
import {
  AuthStore,
  NotificationService,
  OrganizationStore,
  TeamStore,
  canManageTeam,
  warnBeforeUnload,
} from '@corelog/core';
import { Button } from 'primeng/button';
import { ConfirmDangerModal } from '@corelog/ui/confirm-danger-modal';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { ErrorState } from '@corelog/ui/error-state';

import { formatTeamDate, memberCountText, messageOf } from '../shared/team-messages';
import { TeamFormInitial, TeamFormValues } from '../team-form-modal/team-form';
import { TeamFormModal } from '../team-form-modal/team-form-modal';
import { TeamMembersSection } from '../team-members-section/team-members-section';
import { TeamTile } from '../team-tile/team-tile';

const DELETE_DESCRIPTION =
  'Los miembros dejarán de pertenecer a este equipo. Esta acción no se puede deshacer.';

/** Team detail: header card with its data and actions for owners, admins and maintainers, then the members. */
@Component({
  selector: 'app-team-page',
  imports: [
    Button,
    ConfirmDangerModal,
    ErrorState,
    LoadingSkeleton,
    PageContainer,
    TeamFormModal,
    TeamMembersSection,
    TeamTile,
  ],
  templateUrl: './team-page.html',
  styleUrl: './team-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TeamPage {
  private readonly store = inject(TeamStore);
  private readonly auth = inject(AuthStore);
  private readonly organizations = inject(OrganizationStore);
  private readonly notify = inject(NotificationService);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);

  private readonly teamId = toSignal(
    this.route.paramMap.pipe(map((params) => params.get('id') ?? '')),
    { initialValue: '' },
  );

  protected readonly team = this.store.currentTeam;
  protected readonly teamStatus = this.store.currentTeamStatus;
  protected readonly error = this.store.error;
  protected readonly members = this.store.members;
  protected readonly membersStatus = this.store.membersStatus;
  protected readonly membersError = this.store.membersError;

  protected readonly editOpen = signal(false);
  protected readonly deleteOpen = signal(false);
  protected readonly deleting = signal(false);
  /** True while the edit form holds unsaved edits. Written by the form, read by the unsaved-changes guard. */
  protected readonly formDirty = signal(false);
  protected readonly dirty = this.formDirty.asReadonly();

  protected readonly teamReady = computed(() => {
    const current = this.team();
    return current !== null && current.id === this.teamId();
  });
  protected readonly viewerId = computed(() => this.auth.user()?.id ?? null);
  protected readonly viewerTeamRole = computed(
    () => this.members().find((member) => member.userId === this.viewerId())?.role ?? null,
  );
  protected readonly canManage = computed(() =>
    canManageTeam(this.organizations.currentRole(), this.viewerTeamRole()),
  );
  protected readonly notFoundMessage = computed(() =>
    this.error()?.code === 'team.not_found'
      ? 'Este equipo no existe o no tienes acceso.'
      : (this.error()?.message ?? 'No se pudo cargar el equipo.'),
  );
  protected readonly creatorName = computed(() => {
    const createdBy = this.team()?.createdBy;
    if (!createdBy) return 'No disponible';
    return (
      this.members().find((member) => member.userId === createdBy)?.name ??
      'Persona de la organización'
    );
  });
  protected readonly membersSummary = computed(() =>
    this.membersStatus() === 'error'
      ? 'Miembros no disponibles'
      : memberCountText(this.members().length),
  );

  protected readonly deleteDescription = DELETE_DESCRIPTION;
  protected readonly formatDate = formatTeamDate;

  constructor() {
    warnBeforeUnload(this.dirty);

    effect(() => {
      const id = this.teamId();
      if (!id) return;
      untracked(() => this.load(id));
    });
  }

  /** Form starting values for the edit dialog, read from the loaded team. */
  protected initialValues(): TeamFormInitial {
    const current = this.team();
    return {
      name: current?.name ?? '',
      description: current?.description ?? '',
      avatarColor: current?.avatarColor ?? null,
    };
  }

  protected reloadTeam(): void {
    this.load(this.teamId());
  }

  protected reloadMembers(): void {
    const current = this.team();
    if (current) this.store.fetchMembers(current.id);
  }

  /** Saves the edited team; the dialog stays open on failure. */
  protected readonly saveEdit = async (values: TeamFormValues): Promise<void> => {
    const current = this.team();
    if (!current) return;
    await firstValueFrom(this.store.updateTeam(current.id, values));
    this.notify.success('Equipo actualizado');
    this.editOpen.set(false);
  };

  protected async confirmDelete(): Promise<void> {
    const current = this.team();
    if (!current) return;
    this.deleting.set(true);
    try {
      await firstValueFrom(this.store.removeTeam(current.id));
      this.notify.success('Equipo eliminado');
      await this.router.navigate(['..'], { relativeTo: this.route, replaceUrl: true });
    } catch (error) {
      this.notify.error(messageOf(error, 'No se pudo eliminar el equipo'));
    } finally {
      this.deleting.set(false);
      this.deleteOpen.set(false);
    }
  }

  private load(id: string): void {
    this.store.fetchTeam(id);
    this.store.fetchMembers(id);
  }
}

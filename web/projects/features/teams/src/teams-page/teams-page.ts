import {
  ChangeDetectionStrategy,
  Component,
  OnInit,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import {
  canCreateTeam,
  filterTeams,
  NotificationService,
  OrganizationStore,
  TeamStore,
  warnBeforeUnload,
} from '@corelog/core';
import { Button } from 'primeng/button';
import { CardGrid } from '@corelog/ui/card-grid';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { Section } from '@corelog/ui/section';
import { SearchInput } from '@corelog/ui/search-input';
import { Toolbar, ToolbarLeft, ToolbarRight } from '@corelog/ui/toolbar';

import { TeamCard } from '../team-card/team-card';
import { TeamFormInitial, TeamFormValues } from '../team-form-modal/team-form';
import { TeamFormModal } from '../team-form-modal/team-form-modal';

const SKELETON_CARDS = 6;
const EMPTY_TEAM: TeamFormInitial = { name: '', description: '', avatarColor: null };

/** Teams of the active organization. Anyone the organization policy allows can create one; guests only browse. */
@Component({
  selector: 'app-teams-page',
  imports: [
    Button,
    CardGrid,
    EmptyState,
    ErrorState,
    LoadingSkeleton,
    PageContainer,
    PageHeader,
    Section,
    SearchInput,
    TeamCard,
    TeamFormModal,
    Toolbar,
    ToolbarLeft,
    ToolbarRight,
  ],
  templateUrl: './teams-page.html',
  styleUrl: './teams-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TeamsPage implements OnInit {
  private readonly store = inject(TeamStore);
  private readonly organizations = inject(OrganizationStore);
  private readonly notify = inject(NotificationService);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);

  protected readonly teams = this.store.teams;
  protected readonly error = this.store.error;
  protected readonly canCreate = computed(() =>
    canCreateTeam(
      this.organizations.currentRole(),
      this.organizations.currentOrg()?.settings.whoCanCreateTeams,
    ),
  );

  protected readonly query = signal('');
  protected readonly createOpen = signal(false);
  /** True while the create form holds unsaved edits. Written by the form, read by the unsaved-changes guard. */
  protected readonly formDirty = signal(false);
  protected readonly dirty = this.formDirty.asReadonly();

  protected readonly visible = computed(() => filterTeams(this.teams(), this.query()));
  protected readonly loading = computed(
    () => this.store.status() === 'loading' && this.teams().length === 0,
  );
  protected readonly failed = computed(
    () => this.store.status() === 'error' && this.teams().length === 0,
  );
  protected readonly errorMessage = computed(
    () => this.error()?.message ?? 'No se pudieron cargar los equipos.',
  );
  protected readonly emptyDescription = computed(() =>
    this.canCreate()
      ? 'Crea el primer equipo para agrupar a las personas de la organización.'
      : 'Pide a un administrador de la organización que cree un equipo.',
  );

  protected readonly skeletonCards = Array.from({ length: SKELETON_CARDS }, (_, index) => index);
  protected readonly emptyTeam = EMPTY_TEAM;

  /** `?new=1` asks for the create dialog to open as soon as the caller is known to be allowed. */
  private readonly createRequested = signal(false);

  constructor() {
    warnBeforeUnload(this.dirty);

    effect(() => {
      if (!this.createRequested()) return;
      const role = this.organizations.currentRole();
      // Without a role the organization is still loading: wait for it before deciding.
      if (role === null) return;
      const allowed = this.canCreate();
      untracked(() => {
        this.createRequested.set(false);
        if (allowed) this.createOpen.set(true);
      });
    });
  }

  ngOnInit(): void {
    this.load();
    if (this.route.snapshot.queryParamMap.get('new') === '1') {
      this.createRequested.set(true);
      void this.router.navigate([], {
        relativeTo: this.route,
        queryParams: { new: null },
        queryParamsHandling: 'merge',
        replaceUrl: true,
      });
    }
  }

  protected load(): void {
    this.store.fetchTeams();
  }

  protected openCreate(): void {
    this.createOpen.set(true);
  }

  protected closeCreate(): void {
    this.createOpen.set(false);
  }

  /** Saves a new team; the modal stays open on failure so the user can correct it. */
  protected readonly handleCreate = async (values: TeamFormValues): Promise<void> => {
    await firstValueFrom(this.store.createTeam(values));
    this.notify.success('Equipo creado');
    this.createOpen.set(false);
  };
}

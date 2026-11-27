import {
  ChangeDetectionStrategy,
  Component,
  OnInit,
  computed,
  effect,
  inject,
  untracked,
} from '@angular/core';
import {
  AuthStore,
  BreakpointService,
  EffectiveProjectRole,
  PROJECT_MOBILE_QUERY,
  PermissionDefinition,
  ProjectMemberRole,
  ProjectStore,
  effectiveProjectRole,
  hasProjectPermission,
  projectAccessVariantOf,
  projectPermissions,
} from '@corelog/core';
import { ErrorPage } from '@corelog/ui/error-page';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Lozenge } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { Section } from '@corelog/ui/section';
import { StatTile } from '@corelog/ui/stat-tile';
import { Message } from 'primeng/message';

import { injectProjectId } from '../shared/project-route';

type GrantedRole = Exclude<EffectiveProjectRole, null>;

// Matrix columns, read from the widest scope to the narrowest.
const COLUMNS: { key: GrantedRole; label: string }[] = [
  { key: 'admin', label: 'Administrador' },
  { key: 'lead', label: 'Líder' },
  { key: 'contributor', label: 'Colaborador' },
  { key: 'viewer', label: 'Observador' },
];

const ROLE_NAMES: Record<GrantedRole, string> = {
  admin: 'Administrador',
  lead: 'Líder',
  contributor: 'Colaborador',
  viewer: 'Observador',
};

const ROLE_DESCRIPTIONS: Record<GrantedRole, string> = {
  admin: 'Global: acceso a todos los proyectos.',
  lead: 'Dirige el proyecto y su configuración.',
  contributor: 'Trabaja en las incidencias.',
  viewer: 'Solo consulta y sigue incidencias.',
};

const MEMBER_ROLES: ProjectMemberRole[] = ['lead', 'contributor', 'viewer'];

/** Permission matrix of the project and the signed-in user's role in it. */
@Component({
  selector: 'app-project-permissions-page',
  imports: [
    PageContainer,
    PageHeader,
    Section,
    StatTile,
    Lozenge,
    LoadingSkeleton,
    ErrorPage,
    Message,
  ],
  templateUrl: './project-permissions-page.html',
  styleUrl: './project-permissions-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectPermissionsPage implements OnInit {
  private readonly store = inject(ProjectStore);
  private readonly auth = inject(AuthStore);
  private readonly breakpoints = inject(BreakpointService);

  protected readonly projectId = injectProjectId();
  protected readonly isMobile = this.breakpoints.matches(PROJECT_MOBILE_QUERY);

  protected readonly members = this.store.members;
  protected readonly status = this.store.status;

  protected readonly accessVariant = computed(() =>
    this.status() === 'error' ? projectAccessVariantOf(this.store.error()) : null,
  );
  protected readonly loading = computed(
    () => this.status() === 'loading' && this.members().length === 0,
  );

  // The matrix does not depend on the data: it stays visible when the members fail to load.
  protected readonly myRole = computed(() =>
    effectiveProjectRole(this.auth.user()?.role, this.auth.user()?.id, this.members()),
  );

  protected readonly roleLabel = computed(() => {
    const role = this.myRole();
    return role ? ROLE_NAMES[role] : 'Sin acceso como miembro';
  });

  protected readonly roleText = computed(() => {
    const role = this.myRole();
    if (role === 'admin') return 'Tienes todos los permisos por ser administrador global.';
    if (role) {
      return `${ROLE_DESCRIPTIONS[role]} Los permisos que no tienes aparecen con un guion.`;
    }
    return 'No apareces como miembro del proyecto. Pide acceso a un líder.';
  });

  protected readonly roleCounts = computed<Record<ProjectMemberRole, number>>(() => {
    const counts: Record<ProjectMemberRole, number> = { lead: 0, contributor: 0, viewer: 0 };
    for (const member of this.members()) {
      counts[member.role] += 1;
    }
    return counts;
  });

  protected readonly columns = COLUMNS;
  protected readonly memberRoles = MEMBER_ROLES;
  protected readonly roleNames = ROLE_NAMES;
  protected readonly permissions: PermissionDefinition[] = projectPermissions;

  private loadedId: string | null = null;

  constructor() {
    // Follows route changes that reuse this component (another project). The first load runs in ngOnInit.
    effect(() => {
      const id = this.projectId();
      untracked(() => this.load(id));
    });
  }

  ngOnInit(): void {
    this.load(this.projectId());
  }

  private load(id: string): void {
    if (id === this.loadedId) return;
    this.loadedId = id;
    this.store.fetchMembers(id);
  }

  protected granted(role: GrantedRole, permission: PermissionDefinition): boolean {
    return hasProjectPermission(role, permission.key);
  }
}

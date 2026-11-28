import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  AuditApiService,
  AuthStore,
  ORGANIZATION_ID_PROVIDER,
  OrganizationApiService,
  OrganizationStore,
  ProjectApiService,
  TeamApiService,
  type AuditEntry,
} from '@corelog/core';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { Section } from '@corelog/ui/section';
import { StatTile } from '@corelog/ui/stat-tile';
import { firstValueFrom } from 'rxjs';

import { relativeTimeLabel } from '../platform/platform-format';

interface Shortcut {
  readonly path: string;
  readonly label: string;
  readonly hint: string;
  readonly icon: string;
  readonly platform?: boolean;
}

/** Pages of the administration area, as the overview offers them. */
const SHORTCUTS: readonly Shortcut[] = [
  {
    path: '/organization/settings',
    label: 'Configuración',
    hint: 'Nombre, logo, políticas y zona horaria.',
    icon: 'pi pi-cog',
  },
  {
    path: '/organization/members',
    label: 'Miembros',
    hint: 'Roles y accesos de las personas.',
    icon: 'pi pi-users',
  },
  {
    path: '/organization/invitations',
    label: 'Invitaciones',
    hint: 'Invita a más personas y revisa las pendientes.',
    icon: 'pi pi-envelope',
  },
  {
    path: '/organization/catalogs',
    label: 'Tipos, prioridades y resoluciones',
    hint: 'Catálogos de las incidencias.',
    icon: 'pi pi-tags',
  },
  {
    path: '/organization/custom-fields',
    label: 'Campos personalizados',
    hint: 'Datos extra para tus incidencias.',
    icon: 'pi pi-list',
  },
  {
    path: '/organization/statuses',
    label: 'Estados',
    hint: 'Estados disponibles en los flujos de trabajo.',
    icon: 'pi pi-sitemap',
  },
  {
    path: '/organization/notifications',
    label: 'Notificaciones',
    hint: 'Avisos por defecto de los proyectos.',
    icon: 'pi pi-bell',
  },
  {
    path: '/organization/audit',
    label: 'Auditoría',
    hint: 'Quién hizo qué y cuándo.',
    icon: 'pi pi-shield',
  },
  {
    path: '/organization/legal',
    label: 'Documentos legales',
    hint: 'Políticas y términos que aceptan las personas.',
    icon: 'pi pi-file',
    platform: true,
  },
  {
    path: '/organization/platform-audit',
    label: 'Auditoría de la plataforma',
    hint: 'Registro de toda la plataforma.',
    icon: 'pi pi-shield',
    platform: true,
  },
  {
    path: '/organization/email-outbox',
    label: 'Cola de correo',
    hint: 'Correos salientes y reintentos.',
    icon: 'pi pi-send',
    platform: true,
  },
  {
    path: '/organization/jobs',
    label: 'Trabajos programados',
    hint: 'Tareas automáticas y su última ejecución.',
    icon: 'pi pi-clock',
    platform: true,
  },
];

type Count = number | null | undefined;

/** Landing of the organization administration: numbers, pending work, recent activity and shortcuts. */
@Component({
  selector: 'app-org-overview-page',
  imports: [PageContainer, PageHeader, RouterLink, Section, StatTile],
  templateUrl: './org-overview-page.html',
  styleUrl: './org-overview-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgOverviewPage {
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);
  private readonly organizations = inject(OrganizationStore);
  private readonly organizationApi = inject(OrganizationApiService);
  private readonly projectApi = inject(ProjectApiService);
  private readonly teamApi = inject(TeamApiService);
  private readonly auditApi = inject(AuditApiService);
  private readonly auth = inject(AuthStore);

  /** `undefined` while loading, `null` when the number could not be read. */
  protected readonly members = signal<Count>(undefined);
  protected readonly pendingInvitations = signal<Count>(undefined);
  protected readonly projects = signal<Count>(undefined);
  protected readonly teams = signal<Count>(undefined);
  protected readonly recent = signal<readonly AuditEntry[] | null | undefined>(undefined);
  protected readonly now = signal(new Date());

  protected readonly name = computed(() => this.organizations.currentOrg()?.name ?? 'Organización');
  protected readonly ready = computed(
    () =>
      this.members() !== undefined && this.projects() !== undefined && this.recent() !== undefined,
  );
  protected readonly shortcuts = computed(() =>
    SHORTCUTS.filter((shortcut) => !shortcut.platform || this.auth.isPlatformAdmin()),
  );

  constructor() {
    effect(() => {
      const organization = this.organizationId();
      untracked(() => {
        if (organization) void this.load(organization);
      });
    });
  }

  protected display(value: Count): string {
    if (value === undefined) return '...';
    return value === null ? '-' : String(value);
  }

  protected ago(iso: string): string {
    return relativeTimeLabel(iso, this.now());
  }

  private async load(organization: string): Promise<void> {
    this.now.set(new Date());
    const read = async <T>(source: Promise<T>): Promise<T | null> => {
      try {
        return await source;
      } catch {
        return null;
      }
    };
    const [members, invitations, projects, teams, audit] = await Promise.all([
      read(firstValueFrom(this.organizationApi.listMembers(organization, { silent: true }))),
      read(firstValueFrom(this.organizationApi.listInvitations(organization, 'pending'))),
      read(firstValueFrom(this.projectApi.listPage({ page: 1 }))),
      read(firstValueFrom(this.teamApi.list())),
      read(
        firstValueFrom(
          this.auditApi.list(
            { kind: 'organization', organizationId: organization },
            {},
            { limit: 5 },
          ),
        ),
      ),
    ]);
    this.members.set(members ? members.length : null);
    this.pendingInvitations.set(invitations ? invitations.length : null);
    this.projects.set(projects ? projects.meta.total : null);
    this.teams.set(teams ? teams.length : null);
    this.recent.set(audit ? audit.entries.slice(0, 5) : null);
  }
}

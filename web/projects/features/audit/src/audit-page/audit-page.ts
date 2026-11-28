import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import {
  AuditStore,
  ORGANIZATION_ID_PROVIDER,
  OrganizationApiService,
  UserDirectoryApiService,
  asApiError,
  type AuditEntry,
  type AuditFilters,
  type AuditScope,
} from '@corelog/core';
import { DateField } from '@corelog/ui/date-field';
import { ErrorState } from '@corelog/ui/error-state';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { SelectField, type SelectFieldOption } from '@corelog/ui/select-field';
import { UserPicker, type PickableUser } from '@corelog/ui/user-picker';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { firstValueFrom } from 'rxjs';

import { AuditDrawer } from '../audit-drawer/audit-drawer';

const ENTITY_TYPES = [
  'issue',
  'project',
  'organization',
  'user',
  'team',
  'sprint',
  'version',
  'comment',
  'board',
  'workflow',
  'filter',
  'dashboard',
  'calendar_event',
  'webhook',
  'api_token',
  'automation',
];

const ENTITY_LABELS: Readonly<Record<string, string>> = {
  issue: 'Incidencia',
  project: 'Proyecto',
  organization: 'Organización',
  user: 'Usuario',
  team: 'Equipo',
  sprint: 'Sprint',
  version: 'Versión',
  comment: 'Comentario',
  board: 'Tablero',
  workflow: 'Flujo de trabajo',
  filter: 'Filtro',
  dashboard: 'Panel',
  calendar_event: 'Evento',
  webhook: 'Webhook',
  api_token: 'Token de API',
  automation: 'Automatización',
};

const ALL = '';

/** Read-only audit log of an organization or of the whole platform, with filters, paging and CSV export. */
@Component({
  selector: 'app-audit-page',
  imports: [
    AuditDrawer,
    Button,
    DateField,
    ErrorState,
    FieldControl,
    FieldRow,
    FormsModule,
    InputText,
    LoadingSkeleton,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    SelectField,
    UserPicker,
  ],
  templateUrl: './audit-page.html',
  styleUrl: './audit-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuditPage {
  protected readonly store = inject(AuditStore);
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);
  private readonly organizations = inject(OrganizationApiService);
  private readonly directory = inject(UserDirectoryApiService);
  private readonly platform = inject(ActivatedRoute).snapshot.data['scope'] === 'platform';

  protected readonly title = this.platform ? 'Auditoría de la plataforma' : 'Auditoría';
  protected readonly subtitle = this.platform
    ? 'Registro de lo que ocurre en toda la plataforma. Solo lectura.'
    : 'Registro de lo que ocurre en tu organización: quién hizo qué y cuándo. Solo lectura.';

  protected readonly entityOptions: readonly SelectFieldOption[] = [
    { value: ALL, label: 'Todos los tipos' },
    ...ENTITY_TYPES.map((value) => ({ value, label: ENTITY_LABELS[value] ?? value })),
  ];

  protected readonly actor = signal<string | undefined>(undefined);
  protected readonly entityType = signal<string>(ALL);
  protected readonly action = signal('');
  protected readonly from = signal<string | null>(null);
  protected readonly to = signal<string | null>(null);
  protected readonly people = signal<readonly PickableUser[]>([]);
  protected readonly selected = signal<AuditEntry | null>(null);
  protected readonly exporting = signal(false);
  protected readonly exportError = signal<string | null>(null);
  protected readonly loadFailed = 'No se pudo cargar el registro de auditoría. Inténtalo de nuevo.';

  protected readonly rangeInvalid = computed(() => {
    const from = this.from();
    const to = this.to();
    return from !== null && to !== null && from > to;
  });
  protected readonly firstLoad = computed(
    () => !this.store.loaded() && this.store.error() === null,
  );
  protected readonly forbidden = computed(() => this.store.error()?.status === 403);

  private started = false;

  constructor() {
    effect(() => {
      // The organization can change from the switcher: the log follows it.
      const organization = this.platform ? 'platform' : this.organizationId();
      untracked(() => {
        if (!organization) return;
        this.started = true;
        void this.store.open(this.scope(), this.filters());
        void this.loadPeople();
      });
    });
  }

  private scope(): AuditScope {
    if (this.platform) return { kind: 'platform' };
    return { kind: 'organization', organizationId: this.organizationId() as string };
  }

  private filters(): AuditFilters {
    const filters: { -readonly [K in keyof AuditFilters]: AuditFilters[K] } = {};
    const actor = this.actor();
    if (actor) filters.actor = actor;
    if (this.entityType()) filters.entityType = this.entityType();
    if (this.action().trim()) filters.action = this.action().trim();
    if (this.from()) filters.from = this.from() as string;
    if (this.to()) filters.to = this.to() as string;
    return filters;
  }

  private async loadPeople(): Promise<void> {
    if (this.platform) return;
    const organization = this.organizationId();
    if (!organization) return;
    try {
      const members = await firstValueFrom(
        this.organizations.listMembers(organization, { silent: true }),
      );
      this.people.set(members.map((m) => ({ id: m.userId, name: m.name, email: m.email })));
    } catch {
      // The filter by person is optional.
    }
  }

  /** Platform admins search the whole directory as they type. */
  protected async searchPeople(term: string): Promise<void> {
    if (!this.platform || term.trim().length < 2) return;
    try {
      const users = await firstValueFrom(this.directory.search({ query: term.trim() }));
      this.people.set(
        users.map((u) => ({ id: u.id, name: u.displayName || u.name, email: u.email })),
      );
    } catch {
      // Keep the previous list.
    }
  }

  protected apply(): void {
    if (this.rangeInvalid() || !this.started) return;
    void this.store.applyFilters(this.filters());
  }

  protected clear(): void {
    this.actor.set(undefined);
    this.entityType.set(ALL);
    this.action.set('');
    this.from.set(null);
    this.to.set(null);
    void this.store.applyFilters({});
  }

  protected entityLabel(type: string): string {
    return ENTITY_LABELS[type] ?? type;
  }

  protected async exportCsv(): Promise<void> {
    this.exporting.set(true);
    this.exportError.set(null);
    try {
      const blob = await this.store.exportCsv();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `auditoria-${new Date().toISOString().slice(0, 10)}.csv`;
      document.body.append(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch (error) {
      this.exportError.set(asApiError(error).message);
    } finally {
      this.exporting.set(false);
    }
  }

  protected formatDate(iso: string): string {
    return new Date(iso).toLocaleString('es-ES', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }
}

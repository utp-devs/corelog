import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { rxResource } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import {
  describeDirectoryError,
  filterMembers,
  OrganizationStore,
  ORG_ROLE_LABELS,
  ORG_ROLE_TONES,
  OrgRole,
  PeopleApiService,
  PeopleMember,
} from '@corelog/core';
import { DataTable, DataTableCell, DataTableColumn } from '@corelog/ui/data-table';
import { EmptyState } from '@corelog/ui/empty-state';
import { Lozenge, LozengeTone } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { SearchInput } from '@corelog/ui/search-input';
import { Toolbar, ToolbarLeft } from '@corelog/ui/toolbar';
import { UserAvatar } from '@corelog/ui/user-avatar';

/** Directory of the members of the active organization. */
@Component({
  selector: 'app-people-page',
  imports: [
    DataTable,
    DataTableCell,
    EmptyState,
    Lozenge,
    PageContainer,
    PageHeader,
    SearchInput,
    Toolbar,
    ToolbarLeft,
    UserAvatar,
  ],
  templateUrl: './people-page.html',
  styleUrl: './people-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PeoplePage {
  private readonly organizations = inject(OrganizationStore);
  private readonly peopleApi = inject(PeopleApiService);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);

  protected readonly orgId = computed(() => this.organizations.currentOrg()?.id ?? null);
  protected readonly subtitle = computed(() => {
    const org = this.organizations.currentOrg();
    return org ? `Miembros de ${org.name}` : undefined;
  });

  protected readonly query = signal('');

  private readonly directory = rxResource({
    params: () => this.orgId() ?? undefined,
    stream: ({ params: orgId }) => this.peopleApi.listMembers(orgId),
  });

  protected readonly loading = computed(() => this.directory.isLoading());
  protected readonly errorMessage = computed(() => {
    const error = this.directory.error();
    return error ? describeDirectoryError(error) : null;
  });
  protected readonly ready = computed(() => !this.loading() && this.errorMessage() === null);
  protected readonly visible = computed<PeopleMember[]>(() => {
    if (!this.orgId()) return [];
    const loaded = this.directory.hasValue() ? this.directory.value() : [];
    return filterMembers(loaded, this.query());
  });

  protected readonly columns: DataTableColumn[] = [
    { key: 'name', title: 'Persona', mobilePrimary: true },
    { key: 'jobTitle', title: 'Cargo' },
    { key: 'role', title: 'Rol en la organización' },
    { key: 'email', title: 'Correo' },
  ];

  protected toneOf(role: OrgRole): LozengeTone {
    return ORG_ROLE_TONES[role];
  }

  protected labelOf(role: OrgRole): string {
    return ORG_ROLE_LABELS[role];
  }

  protected emptyTitle(): string {
    if (!this.orgId()) return 'Elige una organización';
    return this.query().trim() ? 'Sin resultados' : 'Aún no hay personas';
  }

  protected emptyDescription(): string {
    if (!this.orgId()) return 'El directorio muestra las personas de la organización activa.';
    const query = this.query().trim();
    return query
      ? `No hay personas que coincidan con "${query}".`
      : 'Invita a tus compañeros para verlos aquí.';
  }

  protected reload(): void {
    this.directory.reload();
  }

  protected openPerson(member: PeopleMember): void {
    void this.router.navigate([member.userId], { relativeTo: this.route });
  }
}

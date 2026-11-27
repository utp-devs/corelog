import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { rxResource, toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, RouterLink } from '@angular/router';
import {
  asApiError,
  OrganizationStore,
  ORG_ROLE_LABELS,
  ORG_ROLE_TONES,
  OrgRole,
  PeopleApiService,
  PersonProfile,
  Team,
  TeamApiService,
} from '@corelog/core';
import { Breadcrumbs, BreadcrumbItem } from '@corelog/ui/breadcrumbs';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { Lozenge, LozengeTone } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { Section } from '@corelog/ui/section';
import { SplitLayout } from '@corelog/ui/split-layout';
import { TeamChip } from '@corelog/ui/team-chip';
import { UserAvatar } from '@corelog/ui/user-avatar';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { catchError, map, of, switchMap } from 'rxjs';

const LOAD_FAILED_MESSAGE = 'No se pudo cargar la persona.';
const NOT_FOUND_TITLE = 'Persona no encontrada';
const NOT_FOUND_DESCRIPTION =
  'Esta persona no forma parte de la organización activa o ya no existe.';

interface PersonData {
  readonly person: PersonProfile;
  /** Null when the team list could not be loaded; the profile still shows. */
  readonly teams: Team[] | null;
}

/** Public card of one member of the active organization, with the teams of the person. */
type PersonView =
  | { readonly kind: 'no-org' }
  | { readonly kind: 'forbidden' }
  | { readonly kind: 'not-found' }
  | { readonly kind: 'error'; readonly message: string }
  | { readonly kind: 'loading' }
  | ({ readonly kind: 'ready' } & PersonData);

@Component({
  selector: 'app-person-page',
  imports: [
    DatePipe,
    Breadcrumbs,
    ErrorPage,
    ErrorState,
    LoadingSkeleton,
    Lozenge,
    PageContainer,
    RouterLink,
    Section,
    SplitLayout,
    TeamChip,
    UserAvatar,
  ],
  templateUrl: './person-page.html',
  styleUrl: './person-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PersonPage {
  private readonly organizations = inject(OrganizationStore);
  private readonly peopleApi = inject(PeopleApiService);
  private readonly teamApi = inject(TeamApiService);
  private readonly route = inject(ActivatedRoute);

  protected toneOf(role: OrgRole): LozengeTone {
    return ORG_ROLE_TONES[role];
  }

  protected labelOf(role: OrgRole): string {
    return ORG_ROLE_LABELS[role];
  }

  /** Job title and department on one line, whichever of them is filled in. */
  protected roleLine(person: PersonProfile): string {
    return [person.jobTitle, person.department].filter((part) => part !== '').join(' · ');
  }

  protected readonly notFoundTitle = NOT_FOUND_TITLE;
  protected readonly notFoundDescription = NOT_FOUND_DESCRIPTION;

  private readonly orgId = computed(() => this.organizations.currentOrg()?.id ?? null);
  private readonly userId = toSignal(
    this.route.paramMap.pipe(map((params) => params.get('id') ?? '')),
    { initialValue: this.route.snapshot.paramMap.get('id') ?? '' },
  );

  private readonly card = rxResource({
    params: () => {
      const orgId = this.orgId();
      return orgId ? { orgId, id: this.userId() } : undefined;
    },
    stream: ({ params }) =>
      this.peopleApi.getPerson(params.orgId, params.id).pipe(
        switchMap((person) =>
          // The team list is secondary: when it fails the profile still renders.
          this.teamApi.listForMember(params.orgId, params.id).pipe(
            catchError(() => of(null)),
            map((teams): PersonData => ({ person, teams })),
          ),
        ),
      ),
  });

  protected readonly view = computed<PersonView>(() => {
    if (!this.orgId()) return { kind: 'no-org' };
    const error = this.card.error();
    if (error) {
      const apiError = asApiError(error);
      if (apiError.status === 403) return { kind: 'forbidden' };
      if (apiError.status === 404) return { kind: 'not-found' };
      return {
        kind: 'error',
        message:
          apiError.status === 0 && apiError.code === 'unknown'
            ? LOAD_FAILED_MESSAGE
            : apiError.message,
      };
    }
    const data = this.card.value();
    if (!data || data.person.userId !== this.userId()) return { kind: 'loading' };
    return { kind: 'ready', ...data };
  });

  protected readonly displayName = computed(() => {
    const view = this.view();
    if (view.kind !== 'ready') return '';
    return view.person.displayName || view.person.name;
  });

  protected readonly crumbs = computed<BreadcrumbItem[]>(() => [
    { label: 'Personas', to: '..' },
    { label: this.displayName() },
  ]);

  protected retry(): void {
    this.card.reload();
  }
}

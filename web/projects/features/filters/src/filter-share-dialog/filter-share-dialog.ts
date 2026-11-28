import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import {
  FilterStore,
  NotificationService,
  OrganizationStore,
  SearchApiService,
  asApiError,
  type FilterShare,
  type FilterShareType,
  type FilterSharePermission,
  type SavedQuery,
  type SearchKind,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Select } from 'primeng/select';
import { Subject, debounceTime, of, switchMap, catchError } from 'rxjs';

interface Candidate {
  readonly type: FilterShareType;
  readonly targetId: string;
  readonly name: string;
  readonly detail: string;
}

const KIND_TO_TYPE: Readonly<Partial<Record<SearchKind, FilterShareType>>> = {
  person: 'user',
  team: 'team',
  project: 'project',
};

const TYPE_LABELS: Readonly<Record<FilterShareType, string>> = {
  user: 'Persona',
  team: 'Equipo',
  project: 'Proyecto',
  organization: 'Organización',
};

const PERMISSIONS: Array<{ value: FilterSharePermission; label: string }> = [
  { value: 'view', label: 'Puede ver' },
  { value: 'edit', label: 'Puede editar' },
];

/** Who a filter is shared with: people, teams, projects or the whole organization, with view or edit access. */
@Component({
  selector: 'app-filter-share-dialog',
  imports: [Button, Dialog, FormsModule, InputText, Select],
  templateUrl: './filter-share-dialog.html',
  styleUrl: './filter-share-dialog.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FilterShareDialog {
  private readonly store = inject(FilterStore);
  private readonly search = inject(SearchApiService);
  private readonly organizations = inject(OrganizationStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly typed = new Subject<string>();

  readonly open = input(false);
  readonly filter = input<SavedQuery | null>(null);
  readonly closeRequested = output<void>();
  readonly changed = output<void>();

  protected readonly shares = signal<readonly FilterShare[]>([]);
  protected readonly query = signal('');
  protected readonly candidates = signal<readonly Candidate[]>([]);
  protected readonly busy = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly permissions = PERMISSIONS;
  protected readonly organization = computed(() => this.organizations.currentOrg());
  protected readonly organizationShared = computed(() =>
    this.shares().some((share) => share.type === 'organization'),
  );

  constructor() {
    effect(() => {
      if (!this.open()) return;
      this.shares.set(untracked(() => this.filter()?.shares ?? []));
      this.query.set('');
      this.candidates.set([]);
      this.error.set(null);
    });
    this.typed
      .pipe(
        debounceTime(250),
        switchMap((text) =>
          text.trim().length < 2
            ? of([])
            : this.search
                .searchAll(text.trim(), { limit: 6, types: ['person', 'team', 'project'] })
                .pipe(
                  switchMap((found) =>
                    of(
                      found.hits.flatMap((hit): Candidate[] => {
                        const type = KIND_TO_TYPE[hit.kind];
                        return type
                          ? [
                              {
                                type,
                                targetId: hit.id,
                                name: hit.title,
                                detail: hit.subtitle,
                              },
                            ]
                          : [];
                      }),
                    ),
                  ),
                  catchError(() => of([])),
                ),
        ),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe((items) => this.candidates.set(items));
  }

  protected typeLabel(type: FilterShareType): string {
    return TYPE_LABELS[type];
  }

  protected onQuery(value: string): void {
    this.query.set(value);
    this.typed.next(value);
  }

  protected isShared(candidate: Candidate): boolean {
    return this.shares().some(
      (share) => share.type === candidate.type && share.targetId === candidate.targetId,
    );
  }

  protected add(candidate: Candidate): void {
    if (this.isShared(candidate)) return;
    this.shares.update((list) => [
      ...list,
      {
        type: candidate.type,
        targetId: candidate.targetId,
        targetName: candidate.name,
        permission: 'view',
      },
    ]);
    this.query.set('');
    this.candidates.set([]);
  }

  protected shareOrganization(): void {
    const organization = this.organization();
    if (!organization || this.organizationShared()) return;
    this.shares.update((list) => [
      ...list,
      {
        type: 'organization',
        targetId: organization.id,
        targetName: organization.name,
        permission: 'view',
      },
    ]);
  }

  protected setPermission(index: number, permission: FilterSharePermission): void {
    this.shares.update((list) =>
      list.map((item, at) => (at === index ? { ...item, permission } : item)),
    );
  }

  protected remove(index: number): void {
    this.shares.update((list) => list.filter((_, at) => at !== index));
  }

  protected async save(): Promise<void> {
    const filter = this.filter();
    if (!filter || this.busy()) return;
    this.busy.set(true);
    this.error.set(null);
    try {
      await this.store.share(
        filter.id,
        this.shares().map((s) => ({
          type: s.type,
          targetId: s.targetId,
          permission: s.permission,
        })),
      );
      this.notify.success('Filtro compartido');
      this.changed.emit();
      this.closeRequested.emit();
    } catch (error) {
      this.error.set(asApiError(error).message);
    } finally {
      this.busy.set(false);
    }
  }
}

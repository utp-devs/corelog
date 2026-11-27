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
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  CatalogItem,
  CatalogStore,
  HIERARCHY_STANDARD,
  NotificationService,
  catalogErrorMessage,
} from '@corelog/core';
import { CatalogIcon } from '@corelog/ui/catalog-icon';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Section } from '@corelog/ui/section';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';

import { ProjectSettingsContext } from '../project-settings-context';

type LoadStatus = 'loading' | 'ready' | 'error';

/**
 * Issue types the project uses. Every type of the organization is listed; the standard ones can be
 * switched on or off (at least one stays on) and the epic and subtask types are always on. Without a
 * saved choice the project uses all of them.
 */
@Component({
  selector: 'app-project-issue-types-page',
  imports: [Button, CatalogIcon, ErrorState, LoadingSkeleton, Message, Section],
  templateUrl: './project-issue-types-page.html',
  styleUrl: './project-issue-types-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectIssueTypesPage {
  private readonly context = inject(ProjectSettingsContext);
  private readonly catalog = inject(CatalogStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly status = signal<LoadStatus>('loading');
  protected readonly saving = signal(false);
  protected readonly error = signal<string | null>(null);
  /** The project has a saved choice (false: it uses every organization type). */
  protected readonly configured = signal(false);
  /** Ids the project uses now, as saved. */
  private readonly saved = signal<ReadonlySet<string>>(new Set());
  /** Ids ticked in the list. */
  protected readonly selected = signal<ReadonlySet<string>>(new Set());

  protected readonly types = this.catalog.issueTypes;
  protected readonly canEdit = this.context.canEditNow;

  protected readonly dirty = computed(() => {
    const a = this.saved();
    const b = this.selected();
    return a.size !== b.size || [...a].some((id) => !b.has(id));
  });

  /** Ticked standard types: the last one cannot be switched off. */
  private readonly standardOn = computed(
    () =>
      this.types().filter(
        (item) => item.hierarchyLevel === HIERARCHY_STANDARD && this.selected().has(item.id),
      ).length,
  );

  constructor() {
    this.catalog.ensureLoaded();
    effect(() => {
      const id = this.context.projectId();
      if (id) untracked(() => this.load(id));
    });
  }

  protected isLocked(item: CatalogItem): boolean {
    if (item.hierarchyLevel !== HIERARCHY_STANDARD) return true;
    return this.selected().has(item.id) && this.standardOn() <= 1;
  }

  protected lockReason(item: CatalogItem): string | null {
    if (item.hierarchyLevel === 1) return 'La épica está siempre activa.';
    if (item.hierarchyLevel === -1) return 'La subtarea está siempre activa.';
    if (this.isLocked(item)) return 'El proyecto necesita al menos un tipo estándar.';
    return null;
  }

  protected toggle(item: CatalogItem): void {
    if (!this.canEdit() || this.isLocked(item)) return;
    this.selected.update((current) => {
      const next = new Set(current);
      if (next.has(item.id)) next.delete(item.id);
      else next.add(item.id);
      return next;
    });
  }

  protected reset(): void {
    this.selected.set(new Set(this.saved()));
    this.error.set(null);
  }

  protected retry(): void {
    const id = this.context.projectId();
    if (id) this.load(id);
    this.catalog.ensureLoaded();
  }

  protected save(): void {
    const id = this.context.projectId();
    if (!id || this.saving() || !this.dirty()) return;
    const ids = this.types()
      .filter((item) => this.selected().has(item.id))
      .map((item) => item.id);
    this.saving.set(true);
    this.error.set(null);
    this.catalog
      .saveProjectIssueTypes(id, ids)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (result) => {
          this.saving.set(false);
          this.apply(result.items, result.configured);
          this.notify.success('Tipos de incidencia del proyecto guardados');
        },
        error: (error: unknown) => {
          this.saving.set(false);
          this.error.set(catalogErrorMessage(error, 'No se pudieron guardar los tipos.'));
        },
      });
  }

  private load(projectId: string): void {
    this.status.set('loading');
    this.catalog
      .projectIssueTypes(projectId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (result) => {
          this.apply(result.items, result.configured);
          this.status.set('ready');
        },
        error: () => this.status.set('error'),
      });
  }

  private apply(items: readonly CatalogItem[], configured: boolean): void {
    const ids = new Set(items.map((item) => item.id));
    this.saved.set(ids);
    this.selected.set(new Set(ids));
    this.configured.set(configured);
  }
}

import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  Signal,
  computed,
  effect,
  inject,
  signal,
  untracked,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  NotificationService,
  PermissionHolder,
  UnsavedChangesAware,
  asApiError,
  warnBeforeUnload,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Lozenge } from '@corelog/ui/lozenge';
import { Section } from '@corelog/ui/section';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';

import { groupPermissions, permissionCopy } from '../permission-catalog';
import { ProjectSettingsContext } from '../project-settings-context';
import { settingsErrorMessage } from '../settings-copy';
import {
  GridChange,
  HOLDER_COLUMNS,
  HolderColumn,
  LEAD_LOCKED,
  SchemeGrid,
  defaultGrid,
  diffGrids,
  gridOf,
  grantsOf,
  hasHolder,
  holderLabel,
  schemeProblems,
  toggleHolder,
} from '../scheme-grid';

type Pending = 'save' | 'reset' | null;

/** Matrix of who holds each permission in the project, with a preview of the changes before saving. */
@Component({
  selector: 'app-project-permissions-matrix-page',
  imports: [Section, Lozenge, Button, Message, Dialog, LoadingSkeleton, ErrorState],
  templateUrl: './project-permissions-matrix-page.html',
  styleUrl: './project-permissions-matrix-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectPermissionsMatrixPage implements UnsavedChangesAware {
  private readonly context = inject(ProjectSettingsContext);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly columns = HOLDER_COLUMNS;
  protected readonly canEdit = this.context.canEditNow;
  protected readonly isDefault = this.context.permissions.isDefault;
  protected readonly scheme = this.context.permissions.scheme;

  protected readonly loading = computed(
    () => this.context.permissions.schemeLoading() && this.scheme() === null,
  );
  protected readonly failed = computed(
    () => this.context.permissions.schemeError() !== null && this.scheme() === null,
  );

  /** Permissions of the scheme in server order: the rows of the matrix and of the request. */
  private readonly order = computed(
    () => this.scheme()?.grants.map((grant) => grant.permission) ?? [],
  );
  protected readonly groups = computed(() => groupPermissions(this.order()));

  private readonly saved = computed<SchemeGrid>(() => gridOf(this.scheme()?.grants ?? []));
  protected readonly draft = signal<SchemeGrid>(new Map());

  protected readonly changes = computed(() => diffGrids(this.saved(), this.draft(), this.order()));
  readonly dirty: Signal<boolean> = computed(() => this.changes().length > 0);
  protected readonly problems = computed(() => schemeProblems(this.draft()));

  /** What restoring the standard permissions would change compared with what is saved now. */
  protected readonly resetChanges = computed(() =>
    diffGrids(this.saved(), defaultGrid(this.order()), this.order()),
  );
  protected readonly canReset = computed(() => !this.isDefault() || this.dirty());

  protected readonly pending = signal<Pending>(null);
  protected readonly busy = signal(false);
  protected readonly failure = signal<string | null>(null);

  protected readonly previewChanges = computed<GridChange[]>(() =>
    this.pending() === 'reset' ? this.resetChanges() : this.changes(),
  );

  constructor() {
    warnBeforeUnload(this.dirty);

    // A new or refreshed scheme replaces the working copy.
    effect(() => {
      const grid = this.saved();
      untracked(() => this.draft.set(grid));
    });
  }

  protected copyOf(permission: string): { label: string; description: string } {
    return permissionCopy(permission);
  }

  protected has(permission: string, holder: PermissionHolder): boolean {
    return hasHolder(this.draft(), permission, holder);
  }

  /** Reason a box cannot change, or null when it can. */
  protected lockReason(permission: string, holder: PermissionHolder): string | null {
    if (!this.canEdit()) return null;
    return holder === 'lead' ? (LEAD_LOCKED[permission] ?? null) : null;
  }

  protected toggle(permission: string, holder: PermissionHolder, checked: boolean): void {
    if (!this.canEdit() || this.lockReason(permission, holder)) return;
    this.draft.update((grid) => toggleHolder(grid, permission, holder, checked));
    this.failure.set(null);
  }

  protected checkboxLabel(column: HolderColumn, permission: string): string {
    return `${column.label}: ${permissionCopy(permission).label}`;
  }

  protected namesOf(holders: readonly PermissionHolder[]): string {
    return holders.map((holder) => holderLabel(holder)).join(', ');
  }

  protected discard(): void {
    this.draft.set(this.saved());
    this.failure.set(null);
  }

  protected askSave(): void {
    if (this.problems().length > 0 || !this.dirty()) return;
    this.failure.set(null);
    this.pending.set('save');
  }

  protected askReset(): void {
    this.failure.set(null);
    this.pending.set('reset');
  }

  protected cancelPending(): void {
    if (!this.busy()) this.pending.set(null);
  }

  protected confirm(): void {
    const id = this.context.projectId();
    const action = this.pending();
    if (!id || !action) return;
    this.busy.set(true);
    const request =
      action === 'save'
        ? this.context.permissions.saveScheme(id, grantsOf(this.draft(), this.order()))
        : this.context.permissions.resetScheme(id);
    request.pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: () => {
        this.busy.set(false);
        this.pending.set(null);
        this.notify.success(
          action === 'save' ? 'Permisos guardados' : 'Se restauraron los permisos estándar',
        );
        // The caller's own rights may have moved with the new scheme.
        this.context.refreshOwnPermissions();
      },
      error: (error: unknown) => {
        this.busy.set(false);
        this.pending.set(null);
        const apiError = asApiError(error);
        this.failure.set(
          apiError.code === 'authorization.invalid_scheme'
            ? `Los permisos no son válidos: ${apiError.message}`
            : settingsErrorMessage(error, 'No se pudieron guardar los permisos.'),
        );
      },
    });
  }

  protected retry(): void {
    this.context.reloadProjectData();
  }

  /** Arrow keys move between the boxes of the matrix; Tab still walks them one by one. */
  protected onKeydown(event: KeyboardEvent, row: number, col: number): void {
    const delta: Record<string, [number, number]> = {
      ArrowUp: [-1, 0],
      ArrowDown: [1, 0],
      ArrowLeft: [0, -1],
      ArrowRight: [0, 1],
    };
    const move = delta[event.key];
    if (!move) return;
    const table = (event.currentTarget as HTMLElement).closest('table');
    const next = table?.querySelector<HTMLInputElement>(
      `input[data-row="${row + move[0]}"][data-col="${col + move[1]}"]`,
    );
    if (next) {
      event.preventDefault();
      next.focus();
    }
  }

  /** Position of a permission among all the rows, for keyboard movement. */
  protected rowIndex(permission: string): number {
    return this.order().indexOf(permission);
  }
}

import {
  CdkDrag,
  CdkDragDrop,
  CdkDragPreview,
  CdkDropList,
  CdkDropListGroup,
} from '@angular/cdk/drag-drop';
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
import { ActivatedRoute } from '@angular/router';
import {
  AuthStore,
  BoardColumn,
  BoardCard,
  BoardMoveTarget,
  BoardStore,
  BreakpointService,
  ISSUE_PRIORITY_LABELS,
  OrganizationStore,
  ProjectStore,
  WorkItemPriority,
  isColumnFull,
  PROJECT_MOBILE_QUERY,
  projectAccessVariantOf,
} from '@corelog/core';
import { DropdownMenu } from '@corelog/ui/app-shell';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { Lozenge } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { SearchInput } from '@corelog/ui/search-input';
import { AppTooltip } from '@corelog/ui/tooltip';
import { InputNumber } from 'primeng/inputnumber';
import { Message } from 'primeng/message';
import { Select } from 'primeng/select';
import { Skeleton } from 'primeng/skeleton';
import { ToggleSwitch } from 'primeng/toggleswitch';
import { FormsModule } from '@angular/forms';

import { trackProjectScope } from '../shared/project-scope';
import { isReadOnlyRole, statusName } from '../shared/work-labels';
import { BoardCardView } from './board-card';

/** Priority filter options, most urgent first, as the board lists them. */
const PRIORITY_OPTIONS: { value: WorkItemPriority; label: string }[] = [
  { value: 'critical', label: ISSUE_PRIORITY_LABELS.critical },
  { value: 'high', label: ISSUE_PRIORITY_LABELS.high },
  { value: 'medium', label: ISSUE_PRIORITY_LABELS.medium },
  { value: 'low', label: ISSUE_PRIORITY_LABELS.low },
];

const LOAD_ERROR_MESSAGE =
  'No se pudo cargar el tablero. Verifica que tengas acceso al proyecto e intenta de nuevo.';

/**
 * Kanban board of the project. Cards move by drag and drop or by the "Mover a" menu of each card, which is the
 * keyboard and touch alternative. A move waits for the server answer and then replaces the board; nothing is
 * shown as moved before that. Filters run on the loaded cards and never call the API.
 */
@Component({
  selector: 'app-project-board-page',
  imports: [
    FormsModule,
    CdkDrag,
    CdkDragPreview,
    CdkDropList,
    CdkDropListGroup,
    InputNumber,
    Message,
    Select,
    Skeleton,
    ToggleSwitch,
    BoardCardView,
    AppTooltip,
    DropdownMenu,
    EmptyState,
    ErrorPage,
    ErrorState,
    Lozenge,
    PageContainer,
    PageHeader,
    SearchInput,
  ],
  templateUrl: './project-board-page.html',
  styleUrl: './project-board-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectBoardPage {
  protected readonly store = inject(BoardStore);
  private readonly auth = inject(AuthStore);
  private readonly orgs = inject(OrganizationStore);
  private readonly projects = inject(ProjectStore);
  private readonly breakpoints = inject(BreakpointService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly board = this.store.board.value;
  protected readonly columns = this.store.visibleColumns;
  protected readonly filter = this.store.filter;
  protected readonly priorityOptions = PRIORITY_OPTIONS;
  protected readonly isMobile = this.breakpoints.matches(PROJECT_MOBILE_QUERY);
  protected readonly skeletonColumns = [0, 1, 2, 3];

  /** Viewers and guests see the board but cannot move cards or change limits. */
  protected readonly readOnly = computed(() =>
    isReadOnlyRole(
      this.auth.user()?.role,
      this.orgs.currentRole(),
      this.auth.user()?.id,
      this.projects.members(),
    ),
  );
  protected readonly canManage = computed(
    () => (this.board()?.canManageBoard ?? false) && !this.readOnly(),
  );

  /** Column shown on phones, one at a time. Falls back to the first visible column. */
  private readonly mobileColumnState = signal<string | null>(null);
  protected readonly mobileColumn = computed<BoardColumn | null>(() => {
    const columns = this.columns();
    return columns.find((column) => column.id === this.mobileColumnState()) ?? columns[0] ?? null;
  });
  protected readonly shownColumns = computed(() => {
    const active = this.mobileColumn();
    return this.isMobile() && active ? [active] : this.columns();
  });

  /** "Mover a" targets of every card, named in Spanish by status code. */
  protected readonly targetsByCard = computed(() => {
    const board = this.board();
    const result = new Map<string, BoardMoveTarget[]>();
    for (const column of board?.columns ?? []) {
      for (const card of column.cards) {
        result.set(
          card.id,
          this.store.moveTargetsFor(card.id).map((target) => ({
            ...target,
            name: this.nameOfColumnId(target.id),
          })),
        );
      }
    }
    return result;
  });

  /** Spanish text for the live region, using the Spanish column name of the last move. */
  private readonly lastTarget = signal('');
  protected readonly announcement = computed(() =>
    this.store.announcement() ? `Incidencia movida a ${this.lastTarget()}` : '',
  );

  /** Column whose limit is being edited, and the typed value. */
  protected readonly wipEditing = signal<string | null>(null);
  protected readonly wipDraft = signal<number | null>(null);

  /** Column under a dragged card that can take it; null when none. */
  protected readonly overColumnId = signal<string | null>(null);
  /** Card being dragged; used for the dimmed original while the preview is carried. */
  protected readonly activeCardId = signal<string | null>(null);
  /**
   * Changes after a failed move or limit change. Card views are keyed with it, so the DOM is rebuilt
   * from the unchanged board data and no card stays where the failed drop left it.
   */
  private readonly renderEpoch = signal(0);

  /** Failure of the project read or of the board load. */
  private readonly failure = computed(() => this.scope.accessError() ?? this.store.board.error());
  protected readonly accessVariant = computed(() => projectAccessVariantOf(this.failure()));
  protected readonly failed = computed(() => this.failure() !== null);
  protected readonly ready = computed(() => this.board() !== undefined && !this.failed());
  protected readonly loadErrorMessage = LOAD_ERROR_MESSAGE;

  protected readonly scope = trackProjectScope(
    inject(ActivatedRoute),
    inject(ProjectStore),
    this.destroyRef,
    (projectId) => {
      this.store.load(projectId);
      this.projects.fetchMembers(projectId);
    },
  );

  constructor() {
    effect(() => {
      this.store.setCurrentUserId(this.auth.user()?.id);
    });
    effect(() => {
      if (this.store.moveError()) {
        untracked(() => this.renderEpoch.update((value) => value + 1));
      }
    });
  }

  protected isFull(column: BoardColumn): boolean {
    return isColumnFull(column);
  }

  protected countLabel(column: BoardColumn): string {
    return column.wipLimit !== undefined
      ? `${column.count} / ${column.wipLimit}`
      : String(column.count);
  }

  protected trackCard(card: BoardCard): string {
    return `${this.renderEpoch()}:${card.id}`;
  }

  /** Spanish name of a column: by status code, never the raw API text. */
  protected columnName(column: BoardColumn): string {
    return statusName(column.status, column.name);
  }

  private nameOfColumnId(columnId: string): string {
    const column = this.board()?.columns.find((candidate) => candidate.id === columnId);
    return column ? this.columnName(column) : 'otra columna';
  }

  protected targetsOf(cardId: string): readonly BoardMoveTarget[] {
    return this.targetsByCard().get(cardId) ?? [];
  }

  protected selectMobileColumn(columnId: string): void {
    this.mobileColumnState.set(columnId);
  }

  protected onEnter(columnId: string): void {
    this.overColumnId.set(columnId);
  }

  /** Drop on another column moves the card. Drops on the same column or outside any column do nothing. */
  protected onDrop(event: CdkDragDrop<string, string>): void {
    this.overColumnId.set(null);
    if (event.previousContainer === event.container) {
      return;
    }
    this.moveCard(event.item.data, event.container.data);
  }

  protected moveCard(cardId: string, targetColumnId: string): void {
    if (this.readOnly()) return;
    this.lastTarget.set(this.nameOfColumnId(targetColumnId));
    this.store.move(cardId, targetColumnId);
  }

  protected setSearch(search: string): void {
    this.store.setFilter({ search });
  }

  protected setPriority(priority: WorkItemPriority | null): void {
    this.store.setFilter({ priority });
  }

  protected setOnlyMine(onlyMine: boolean): void {
    this.store.setFilter({ onlyMine });
  }

  protected startWip(column: BoardColumn): void {
    this.wipDraft.set(column.wipLimit ?? null);
    this.wipEditing.set(column.id);
  }

  protected cancelWip(): void {
    this.wipEditing.set(null);
  }

  /** Saves the typed limit; an empty value removes it. */
  protected saveWip(column: BoardColumn): void {
    const draft = this.wipDraft();
    this.store.saveWipLimit(column.id, draft === null ? '' : String(draft));
    this.wipEditing.set(null);
  }

  protected removeWip(column: BoardColumn): void {
    this.store.saveWipLimit(column.id, '');
    this.wipEditing.set(null);
  }

  protected retry(): void {
    this.scope.reload();
  }
}

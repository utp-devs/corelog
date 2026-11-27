import {
  CdkDrag,
  CdkDragDrop,
  CdkDragHandle,
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
  BoardCard,
  BoardColumn,
  BoardStore,
  BreakpointService,
  ISSUE_PRIORITY_LABELS,
  ProjectApiService,
  WorkItemPriority,
  isColumnFull,
  PROJECT_MOBILE_QUERY,
  projectAccessVariantOf,
} from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { Lozenge } from '@corelog/ui/lozenge';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { SearchInput } from '@corelog/ui/search-input';
import { InputNumber } from 'primeng/inputnumber';
import { Message } from 'primeng/message';
import { Select } from 'primeng/select';
import { Skeleton } from 'primeng/skeleton';
import { ToggleSwitch } from 'primeng/toggleswitch';
import { FormsModule } from '@angular/forms';

import { trackProjectScope } from '../shared/project-scope';
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
 * Kanban board of the project. Cards move by drag and drop or by the "Mover a..." selector, which is the
 * keyboard alternative. A move waits for the server answer and then replaces the board; nothing is
 * shown as moved before that. Filters run on the loaded cards and never call the API.
 */
@Component({
  selector: 'app-project-board-page',
  imports: [
    FormsModule,
    CdkDrag,
    CdkDragHandle,
    CdkDragPreview,
    CdkDropList,
    CdkDropListGroup,
    InputNumber,
    Message,
    Select,
    Skeleton,
    ToggleSwitch,
    BoardCardView,
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
  private readonly breakpoints = inject(BreakpointService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly board = this.store.board.value;
  protected readonly columns = this.store.visibleColumns;
  protected readonly filter = this.store.filter;
  protected readonly priorityOptions = PRIORITY_OPTIONS;
  protected readonly isMobile = this.breakpoints.matches(PROJECT_MOBILE_QUERY);
  protected readonly skeletonColumns = [0, 1, 2, 3];

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
    inject(ProjectApiService),
    this.destroyRef,
    (projectId) => this.store.load(projectId),
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

  protected moveTargets(cardId: string) {
    return this.store.moveTargetsFor(cardId);
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
    this.store.move(event.item.data, event.container.data);
  }

  protected moveCard(cardId: string, targetColumnId: string): void {
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

  protected saveWip(column: BoardColumn, event: Event): void {
    this.store.saveWipLimit(column.id, (event.target as HTMLInputElement).value);
  }

  /** Enter saves the limit from the keyboard, like leaving the field. */
  protected onWipKey(column: BoardColumn, event: KeyboardEvent): void {
    if (event.key === 'Enter') {
      this.saveWip(column, event);
    }
  }

  protected retry(): void {
    this.scope.reload();
  }
}

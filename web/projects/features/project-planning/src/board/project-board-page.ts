import {
  CdkDrag,
  CdkDragDrop,
  CdkDragPlaceholder,
  CdkDragPreview,
  CdkDropList,
  CdkDropListGroup,
} from '@angular/cdk/drag-drop';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  DOCUMENT,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router, RouterLink } from '@angular/router';
import {
  BacklogV2ApiService,
  BoardSwimlaneMode,
  BoardViewCard,
  BoardViewStore,
  BreakpointService,
  CompleteSprintInput,
  CurrentProjectStore,
  IssueStore,
  NotificationService,
  PROJECT_MOBILE_QUERY,
  SprintPlanStore,
  UNMAPPED_COLUMN_ID,
  adjacentColumnId,
  asApiError,
  dropNeighbors,
  boardSettingsErrorMessage,
  projectAccessVariantOf,
  projectPath,
  sprintErrorMessage,
} from '@corelog/core';
import { ArchivedNotice } from '@corelog/ui/archived-notice';
import { BoardColumnHeader } from '@corelog/ui/board-column';
import { Dialog } from '@corelog/ui/dialog';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorPage } from '@corelog/ui/error-page';
import { ErrorState } from '@corelog/ui/error-state';
import { FieldRow, FieldControl } from '@corelog/ui/field-row';
import { KanbanCard, KanbanMoveTarget } from '@corelog/ui/kanban-card';
import { PageContainer } from '@corelog/ui/page-container';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { TransitionRunner } from '@corelog/ui/transition-dialog';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';
import { Skeleton } from 'primeng/skeleton';

import { daysLeftOf, daysLeftText } from '../shared/planning-format';
import { trackProjectScope } from '../shared/project-scope';
import { CompleteSprintDialog, CompleteSprintSummary } from '../sprints/complete-sprint-dialog';
import { BoardCell, boardHeaders, boardRows, estimateText } from './board-grid';
import { BoardSettingsDrawer } from './board-settings-drawer';
import { BoardSprintChip, BoardToolbar } from './board-toolbar';

const MIN_SUMMARY = 3;
const LOAD_ERROR_MESSAGE =
  'No se pudo cargar el tablero. Verifica que tengas acceso al proyecto e intenta de nuevo.';

const EMPTY_SUMMARY: CompleteSprintSummary = {
  doneIssues: 0,
  openIssues: 0,
  donePoints: 0,
  openPoints: 0,
};

/**
 * Board of the project: columns with sticky headers and WIP indicators, optional swimlanes, cards that move by
 * drag and drop, by their "Mover a" menu or with Alt+Arrow keys. A move shows at once and goes back if the
 * server refuses it. Settings, sprint completion and board creation live in drawers and dialogs from here.
 */
@Component({
  selector: 'app-project-board-page',
  imports: [
    ArchivedNotice,
    BoardColumnHeader,
    BoardSettingsDrawer,
    BoardToolbar,
    Button,
    CdkDrag,
    CdkDragPlaceholder,
    CdkDragPreview,
    CdkDropList,
    CdkDropListGroup,
    CompleteSprintDialog,
    ConfirmModal,
    Dialog,
    EmptyState,
    ErrorPage,
    ErrorState,
    FieldControl,
    FieldRow,
    FormsModule,
    InputText,
    KanbanCard,
    Message,
    PageContainer,
    RouterLink,
    Skeleton,
    TransitionRunner,
  ],
  templateUrl: './project-board-page.html',
  styleUrl: './project-board-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectBoardPage {
  protected readonly store = inject(BoardViewStore);
  private readonly current = inject(CurrentProjectStore);
  private readonly sprintPlan = inject(SprintPlanStore);
  private readonly backlogApi = inject(BacklogV2ApiService);
  private readonly issues = inject(IssueStore);
  private readonly notify = inject(NotificationService);
  private readonly router = inject(Router);
  private readonly breakpoints = inject(BreakpointService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly document = inject(DOCUMENT);
  private readonly injector = inject(Injector);

  protected readonly isMobile = this.breakpoints.matches(PROJECT_MOBILE_QUERY);
  protected readonly view = this.store.view.value;
  protected readonly skeletonColumns = [0, 1, 2, 3];
  protected readonly loadErrorMessage = LOAD_ERROR_MESSAGE;
  protected readonly unmappedId = UNMAPPED_COLUMN_ID;

  protected readonly scope = trackProjectScope(this.current, this.destroyRef, (projectId) => {
    this.store.open(projectId);
    this.sprintPlan.loadSprints(projectId);
  });

  private readonly failure = computed(
    () => this.scope.accessError() ?? this.store.boards.error() ?? this.store.view.error(),
  );
  protected readonly accessVariant = computed(() => projectAccessVariantOf(this.failure()));
  protected readonly failed = computed(() => this.failure() !== null);
  protected readonly ready = computed(() => this.view() !== undefined && !this.failed());

  protected readonly board = computed(() => this.view()?.board ?? null);
  protected readonly archived = this.current.isArchived;
  /** Viewers, guests and archived projects read the board but do not change it. */
  protected readonly readOnly = computed(() => this.archived() || !this.store.canMove());
  protected readonly canManage = computed(() => this.store.canManage() && !this.archived());
  protected readonly dragDisabled = computed(() => this.readOnly() || this.isMobile());
  protected readonly backlogPath = computed(() => {
    const project = this.current.project();
    return project ? projectPath(project, 'backlog') : '/';
  });

  /** Sprint chip and completion: scrum boards with an active sprint. */
  protected readonly sprintChip = computed<BoardSprintChip | null>(() => {
    const view = this.view();
    if (!view?.sprint || view.board.type !== 'scrum') return null;
    const detail = this.sprintPlan.sprints.value()?.find((item) => item.id === view.sprint?.id);
    const days = daysLeftOf(detail?.endDate);
    return {
      id: view.sprint.id,
      name: view.sprint.name,
      remaining: daysLeftText(days),
      overdue: days !== null && days < 0,
    };
  });
  protected readonly noActiveSprint = computed(() => {
    const view = this.view();
    return !!view && view.board.type === 'scrum' && !view.sprint;
  });

  protected readonly headers = computed(() => {
    const view = this.view();
    return view ? boardHeaders(view, this.store.estimation()) : [];
  });
  private readonly mobileColumnState = signal<string | null>(null);
  protected readonly mobileColumnId = computed(() => {
    const headers = this.headers();
    const chosen = this.mobileColumnState();
    return headers.find((header) => header.id === chosen)?.id ?? headers[0]?.id ?? null;
  });
  protected readonly shownHeaders = computed(() => {
    const headers = this.headers();
    const active = this.mobileColumnId();
    return this.isMobile() && active ? headers.filter((header) => header.id === active) : headers;
  });
  protected readonly rows = computed(() => {
    const view = this.view();
    if (!view) return [];
    const rows = boardRows(view);
    const active = this.mobileColumnId();
    if (!this.isMobile() || !active) return rows;
    return rows.map((row) => ({
      ...row,
      cells: row.cells.filter((cell) => cell.columnId === active),
    }));
  });
  protected readonly estimation = this.store.estimation;
  protected readonly firstColumnId = computed(() => this.headers()[0]?.id ?? null);
  protected readonly hasCards = computed(() => this.headers().some((header) => header.count > 0));

  /** Columns a card can move to, by the column it is in (the unmapped column cannot be a target). */
  private readonly targetsByColumn = computed(() => {
    const mapped = this.headers().filter((header) => !header.implicit);
    const result = new Map<string, KanbanMoveTarget[]>();
    for (const header of this.headers()) {
      result.set(
        header.id,
        mapped
          .filter((target) => target.id !== header.id)
          .map((target) => ({ id: target.id, name: target.name })),
      );
    }
    return result;
  });

  protected readonly settingsOpen = signal(false);
  protected readonly createBoardOpen = signal(false);
  protected readonly newBoardName = signal('');
  protected readonly createBoardError = signal<string | null>(null);
  protected readonly deleteBoardOpen = signal(false);
  protected readonly deleteBoardBusy = signal(false);

  protected readonly creating = signal(false);
  protected readonly draft = signal('');
  protected readonly createError = signal<string | null>(null);

  protected readonly completeOpen = signal(false);
  protected readonly completeBusy = signal(false);
  protected readonly completeError = signal<string | null>(null);
  protected readonly completeSummary = signal<CompleteSprintSummary>(EMPTY_SUMMARY);
  protected readonly plannedSprints = computed(
    () => this.sprintPlan.sprints.value()?.filter((sprint) => sprint.status === 'planned') ?? [],
  );
  protected readonly activeSprint = computed(() => {
    const id = this.view()?.sprint?.id;
    return this.sprintPlan.sprints.value()?.find((sprint) => sprint.id === id) ?? null;
  });

  protected readonly runner = viewChild(TransitionRunner);
  protected readonly inputCard = computed(() => {
    const request = this.store.inputRequest();
    return request ? (this.store.findCard(request.cardId) ?? null) : null;
  });

  constructor() {
    effect(() => {
      const request = this.store.inputRequest();
      if (request) untracked(() => this.openTransitionDialog(request.cardId, request.transitionId));
    });
  }

  protected targetsOf(columnId: string): readonly KanbanMoveTarget[] {
    return this.readOnly() ? [] : (this.targetsByColumn().get(columnId) ?? []);
  }

  protected estimateOf(estimate: number): string {
    return estimateText(estimate, this.estimation());
  }

  protected columnNameOf(columnId: string): string {
    return this.headers().find((header) => header.id === columnId)?.name ?? '';
  }

  protected selectMobileColumn(id: string): void {
    this.mobileColumnState.set(id);
  }

  /* Moves */

  protected onDrop(event: CdkDragDrop<BoardCell>): void {
    const card = event.item.data as BoardViewCard;
    const target = event.container.data;
    if (this.readOnly() || target.columnId === UNMAPPED_COLUMN_ID) return;
    const others = target.cards.map((item) => item.id).filter((id) => id !== card.id);
    const neighbors = dropNeighbors(others, event.currentIndex);
    const sameSpot =
      event.previousContainer === event.container && event.previousIndex === event.currentIndex;
    if (sameSpot) return;
    this.store.move(card.id, target.columnId, {
      ...neighbors,
      swimlaneId: target.laneId && target.laneId !== card.swimlaneId ? target.laneId : undefined,
    });
  }

  protected moveTo(card: BoardViewCard, columnId: string): void {
    this.store.move(card.id, columnId);
  }

  /** Alt+Arrow on a card: move it to the neighbor column. */
  protected shift(card: BoardViewCard, columnId: string, direction: -1 | 1): void {
    const mapped = this.headers().filter((header) => !header.implicit);
    const target = adjacentColumnId(mapped, columnId, direction);
    if (target) {
      this.store.move(card.id, target);
      this.keepFocusOn(card);
    }
  }

  /** The moved card is drawn again in its new column: focus goes back to it so the keyboard can keep moving it. */
  private keepFocusOn(card: BoardViewCard): void {
    afterNextRender(
      () =>
        this.document
          .querySelector<HTMLElement>(`[data-testid="kanban-card-${card.key}"] a`)
          ?.focus(),
      { injector: this.injector },
    );
  }

  /** The card dropped into the unmapped column is refused: it only holds what has no column. */
  protected readonly canEnter = (): boolean => false;
  protected readonly anyEnter = (): boolean => true;

  /* Header actions */

  protected onSwimlane(mode: BoardSwimlaneMode): void {
    if (mode === this.board()?.swimlaneMode) return;
    this.store.saveSettings({ swimlaneMode: mode }).subscribe({
      error: (error: unknown) => this.notify.error(boardSettingsErrorMessage(asApiError(error))),
    });
  }

  protected onBoardSelected(id: string): void {
    if (id !== this.store.boardId()) this.store.select(id);
  }

  /* Inline create */

  protected startCreate(): void {
    this.creating.set(true);
    this.draft.set('');
    this.createError.set(null);
    afterNextRender(
      () =>
        this.document
          .querySelector<HTMLInputElement>('[data-testid="board-create-input"]')
          ?.focus(),
      { injector: this.injector },
    );
  }

  protected cancelCreate(): void {
    this.creating.set(false);
    this.createError.set(null);
  }

  protected submitCreate(): void {
    const summary = this.draft().trim();
    if (summary.length < MIN_SUMMARY) {
      this.createError.set('Escribe al menos 3 caracteres.');
      return;
    }
    this.store.createCard(summary).subscribe({
      next: () => {
        this.draft.set('');
        this.createError.set(null);
      },
      error: () => this.createError.set('No se pudo crear la incidencia. Intenta de nuevo.'),
    });
  }

  /* Dialogs: choice of transition, required data */

  protected pickTransition(id: string): void {
    this.store.resolveChoice(id);
  }

  private openTransitionDialog(cardId: string, transitionId: string): void {
    this.issues.listTransitions(cardId).subscribe({
      next: (items) => {
        const match = items.find((item) => item.id === transitionId);
        if (match) {
          this.runner()?.run(match);
        } else {
          this.store.clearInputRequest();
          this.notify.error('El flujo no tiene una transición disponible para ese movimiento.');
        }
      },
      error: () => {
        this.store.clearInputRequest();
        this.notify.error('No se pudo mover la incidencia. Intenta de nuevo.');
      },
    });
  }

  protected onTransitionFinished(): void {
    this.store.clearInputRequest();
    this.store.refresh();
  }

  protected onTransitionClosed(): void {
    this.store.clearInputRequest();
  }

  /* Sprint completion */

  protected openComplete(): void {
    const projectId = this.current.projectId();
    if (!projectId) return;
    this.completeError.set(null);
    this.backlogApi.get(projectId).subscribe({
      next: (backlog) => {
        const active = backlog.sections.find((section) => section.kind === 'active');
        const totals = active?.totals;
        const done = totals?.byCategory.done;
        this.completeSummary.set({
          doneIssues: done?.issues ?? 0,
          donePoints: done?.points ?? 0,
          openIssues: (totals?.issues ?? 0) - (done?.issues ?? 0),
          openPoints: (totals?.points ?? 0) - (done?.points ?? 0),
        });
        this.completeOpen.set(true);
      },
      error: () => this.notify.error('No se pudo leer el sprint. Intenta de nuevo.'),
    });
  }

  protected confirmComplete(input: CompleteSprintInput): void {
    const sprint = this.view()?.sprint;
    const project = this.current.project();
    if (!sprint || !project) return;
    this.completeBusy.set(true);
    this.sprintPlan.complete(sprint.id, input).subscribe({
      next: (result) => {
        this.completeBusy.set(false);
        this.completeOpen.set(false);
        this.notify.success(`Sprint «${result.sprint.name}» completado`);
        this.sprintPlan.loadSprints(project.id);
        void this.router.navigateByUrl(projectPath(project, `sprints/${sprint.id}/report`));
      },
      error: (error: unknown) => {
        this.completeBusy.set(false);
        this.completeError.set(sprintErrorMessage(asApiError(error)));
      },
    });
  }

  /* Boards */

  protected openCreateBoard(): void {
    this.newBoardName.set('');
    this.createBoardError.set(null);
    this.createBoardOpen.set(true);
  }

  protected submitCreateBoard(): void {
    const name = this.newBoardName().trim();
    if (!name) {
      this.createBoardError.set('Escribe un nombre.');
      return;
    }
    this.store.createBoard({ name }).subscribe({
      next: () => this.createBoardOpen.set(false),
      error: (error: unknown) =>
        this.createBoardError.set(boardSettingsErrorMessage(asApiError(error))),
    });
  }

  protected confirmDeleteBoard(): void {
    this.deleteBoardBusy.set(true);
    this.store.deleteBoard().subscribe({
      next: () => {
        this.deleteBoardBusy.set(false);
        this.deleteBoardOpen.set(false);
        this.notify.success('Tablero eliminado');
      },
      error: (error: unknown) => {
        this.deleteBoardBusy.set(false);
        this.deleteBoardOpen.set(false);
        this.notify.error(boardSettingsErrorMessage(asApiError(error)));
      },
    });
  }

  protected retry(): void {
    this.scope.reload();
  }
}

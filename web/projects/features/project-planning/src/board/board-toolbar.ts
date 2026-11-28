import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { BoardDetail, BoardEstimation, BoardSummary, BoardSwimlaneMode } from '@corelog/core';
import { DropdownMenu } from '@corelog/ui/app-shell';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';

/** The active sprint as the board header shows it. */
export interface BoardSprintChip {
  readonly id: string;
  readonly name: string;
  /** "Quedan 8 días" and the like. */
  readonly remaining: string;
  /** The sprint is past its end date. */
  readonly overdue: boolean;
}

const SWIMLANE_OPTIONS: readonly SelectFieldOption[] = [
  { value: 'none', label: 'Sin carriles' },
  { value: 'assignee', label: 'Por responsable' },
  { value: 'epic', label: 'Por épica' },
  { value: 'story', label: 'Por historia' },
  { value: 'query', label: 'Por consulta' },
];

const ESTIMATIONS: readonly { value: BoardEstimation; label: string }[] = [
  { value: 'story_points', label: 'Puntos' },
  { value: 'issue_count', label: 'Incidencias' },
  { value: 'time', label: 'Horas' },
];

/**
 * Header of the board: board switcher, sprint chip with "Completar sprint", quick filters, swimlane choice,
 * estimation toggle and the more menu. It only shows state and emits what the person asks for.
 */
@Component({
  selector: 'app-board-toolbar',
  imports: [AppTooltip, Button, DropdownMenu, FormsModule, RouterLink, SelectField],
  templateUrl: './board-toolbar.html',
  styleUrl: './board-toolbar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BoardToolbar {
  readonly boards = input<readonly BoardSummary[]>([]);
  readonly board = input.required<BoardDetail>();
  readonly sprint = input<BoardSprintChip | null>(null);
  readonly activeFilterIds = input<readonly string[]>([]);
  readonly estimation = input<BoardEstimation>('story_points');
  readonly canManage = input(false);
  /** The sprint can be completed from here. */
  readonly canCompleteSprint = input(false);
  /** Project is archived or the person only reads. */
  readonly readOnly = input(false);
  readonly refreshing = input(false);
  /** Path of the backlog, the target of the sprint chip. */
  readonly backlogLink = input('/');

  readonly boardSelected = output<string>();
  readonly filterToggled = output<string>();
  readonly filtersCleared = output<void>();
  readonly swimlaneChanged = output<BoardSwimlaneMode>();
  readonly estimationChanged = output<BoardEstimation>();
  readonly completeSprint = output<void>();
  readonly settingsRequested = output<void>();
  readonly createBoardRequested = output<void>();
  readonly deleteBoardRequested = output<void>();

  protected readonly swimlaneOptions = SWIMLANE_OPTIONS;
  protected readonly estimations = ESTIMATIONS;
  protected readonly boardOptions = computed<SelectFieldOption[]>(() =>
    this.boards().map((item) => ({ value: item.id, label: item.name })),
  );
  protected readonly hasSeveralBoards = computed(() => this.boards().length > 1);
  protected readonly filters = computed(() => this.board().quickFilters);
  protected readonly canDelete = computed(() => this.canManage() && !this.board().isDefault);
}

import { CdkDrag, CdkDragDrop, CdkDragHandle, CdkDropList } from '@angular/cdk/drag-drop';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  BOARD_COLUMN_COLORS,
  BoardCardField,
  BoardDetail,
  BoardEstimation,
  BoardSwimlaneMode,
  BoardViewStore,
  NotificationService,
  boardSettingsErrorMessage,
} from '@corelog/core';
import { SelectField, SelectFieldOption } from '@corelog/ui/select-field';
import { SidePanel } from '@corelog/ui/side-panel';
import { AppTooltip } from '@corelog/ui/tooltip';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Message } from 'primeng/message';
import { MultiSelect } from 'primeng/multiselect';

type TabId = 'columns' | 'swimlanes' | 'filters' | 'cards';

interface ColumnDraft {
  readonly key: number;
  readonly id?: string;
  readonly name: string;
  readonly color: string;
  readonly min: number | null;
  readonly max: number | null;
  readonly statusIds: readonly string[];
}

interface QueryDraft {
  readonly key: number;
  readonly name: string;
  readonly query: string;
}

const TABS: readonly { id: TabId; label: string }[] = [
  { id: 'columns', label: 'Columnas' },
  { id: 'swimlanes', label: 'Carriles' },
  { id: 'filters', label: 'Filtros rápidos' },
  { id: 'cards', label: 'Tarjetas' },
];

const FIELD_LABELS: readonly { id: BoardCardField; label: string }[] = [
  { id: 'key', label: 'Clave' },
  { id: 'type', label: 'Tipo' },
  { id: 'priority', label: 'Prioridad' },
  { id: 'assignee', label: 'Responsable' },
  { id: 'labels', label: 'Etiquetas' },
  { id: 'points', label: 'Puntos' },
  { id: 'epic', label: 'Épica' },
  { id: 'subtasks', label: 'Subtareas' },
  { id: 'flag', label: 'Impedimento' },
];

const SWIMLANE_OPTIONS: readonly SelectFieldOption[] = [
  { value: 'none', label: 'Sin carriles' },
  { value: 'assignee', label: 'Por responsable' },
  { value: 'epic', label: 'Por épica' },
  { value: 'story', label: 'Por historia' },
  { value: 'query', label: 'Por consulta' },
];

const ESTIMATION_OPTIONS: readonly SelectFieldOption[] = [
  { value: 'story_points', label: 'Puntos de historia' },
  { value: 'issue_count', label: 'Número de incidencias' },
  { value: 'time', label: 'Tiempo restante' },
];

const COLOR_NAMES: Readonly<Record<string, string>> = {
  gray: 'Gris',
  red: 'Rojo',
  orange: 'Naranja',
  amber: 'Ámbar',
  green: 'Verde',
  teal: 'Turquesa',
  blue: 'Azul',
  indigo: 'Índigo',
  purple: 'Morado',
  pink: 'Rosa',
};

const MAX_COLUMNS = 20;
const MAX_QUERIES = 20;

let nextKey = 0;

/**
 * Settings of the board in a side drawer: columns (order, statuses, WIP limits, colors), swimlanes, quick
 * filters and card layout. Each tab saves on its own. Only people who manage the board reach it.
 */
@Component({
  selector: 'app-board-settings-drawer',
  imports: [
    AppTooltip,
    Button,
    CdkDrag,
    CdkDragHandle,
    CdkDropList,
    FormsModule,
    InputText,
    Message,
    MultiSelect,
    SelectField,
    SidePanel,
  ],
  templateUrl: './board-settings-drawer.html',
  styleUrl: './board-settings-drawer.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BoardSettingsDrawer {
  private readonly store = inject(BoardViewStore);
  private readonly notify = inject(NotificationService);

  readonly open = input(false);
  readonly board = input.required<BoardDetail>();
  readonly closeRequested = output<void>();

  protected readonly tabs = TABS;
  protected readonly fieldLabels = FIELD_LABELS;
  protected readonly swimlaneOptions = SWIMLANE_OPTIONS;
  protected readonly estimationOptions = ESTIMATION_OPTIONS;
  protected readonly colors = BOARD_COLUMN_COLORS;
  protected readonly colorNames = COLOR_NAMES;
  protected readonly busy = this.store.busy;

  protected readonly tab = signal<TabId>('columns');
  protected readonly error = signal<string | null>(null);

  protected readonly columns = signal<readonly ColumnDraft[]>([]);
  protected readonly swimlaneMode = signal<BoardSwimlaneMode>('none');
  protected readonly swimlanes = signal<readonly QueryDraft[]>([]);
  protected readonly filters = signal<readonly QueryDraft[]>([]);
  protected readonly name = signal('');
  protected readonly estimation = signal<BoardEstimation>('story_points');
  protected readonly fields = signal<readonly BoardCardField[]>([]);
  protected readonly showUnmapped = signal(true);
  protected readonly retention = signal<number | null>(null);

  protected readonly statusOptions = computed(() =>
    this.board().workflowStatuses.map((status) => ({
      label: status.name,
      value: status.id,
    })),
  );
  /** Statuses no column holds yet: they show up in the implicit "Sin asignar" column. */
  protected readonly unassignedNames = computed(() => {
    const used = new Set(this.columns().flatMap((column) => column.statusIds));
    return this.board()
      .workflowStatuses.filter((status) => !used.has(status.id))
      .map((status) => status.name);
  });
  protected readonly canAddColumn = computed(() => this.columns().length < MAX_COLUMNS);
  protected readonly canAddFilter = computed(() => this.filters().length < MAX_QUERIES);
  protected readonly canAddLane = computed(() => this.swimlanes().length < MAX_QUERIES);
  protected readonly columnsInvalid = computed(() =>
    this.columns().some(
      (column) =>
        !column.name.trim() ||
        (column.min !== null && column.max !== null && column.min > column.max),
    ),
  );
  protected readonly filtersInvalid = computed(() =>
    this.filters().some((filter) => !filter.name.trim() || !filter.query.trim()),
  );
  protected readonly lanesInvalid = computed(
    () =>
      this.swimlaneMode() === 'query' &&
      (this.swimlanes().length === 0 ||
        this.swimlanes().some((lane) => !lane.name.trim() || !lane.query.trim())),
  );

  constructor() {
    effect(() => {
      const board = this.board();
      if (!this.open()) return;
      untracked(() => this.load(board));
    });
  }

  protected selectTab(tab: TabId): void {
    this.tab.set(tab);
    this.error.set(null);
  }

  /** Arrow keys move between tabs, as a tab list does. */
  protected onTabKeydown(event: KeyboardEvent): void {
    const index = TABS.findIndex((item) => item.id === this.tab());
    const step = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0;
    if (step === 0) return;
    event.preventDefault();
    const next = TABS[(index + step + TABS.length) % TABS.length];
    this.selectTab(next.id);
    const target = (event.currentTarget as HTMLElement).querySelector<HTMLElement>(
      `#settings-tab-${next.id}`,
    );
    target?.focus();
  }

  /* Columns */

  protected addColumn(): void {
    this.columns.update((list) => [
      ...list,
      { key: nextKey++, name: '', color: '', min: null, max: null, statusIds: [] },
    ]);
  }

  protected patchColumn(key: number, patch: Partial<ColumnDraft>): void {
    this.columns.update((list) =>
      list.map((column) => (column.key === key ? { ...column, ...patch } : column)),
    );
  }

  /** A status belongs to one column: choosing it here takes it out of the others. */
  protected setStatuses(key: number, statusIds: readonly string[]): void {
    const chosen = new Set(statusIds);
    this.columns.update((list) =>
      list.map((column) =>
        column.key === key
          ? { ...column, statusIds }
          : { ...column, statusIds: column.statusIds.filter((id) => !chosen.has(id)) },
      ),
    );
  }

  protected removeColumn(key: number): void {
    if (this.columns().length <= 1) return;
    this.columns.update((list) => list.filter((column) => column.key !== key));
  }

  protected moveColumn(index: number, step: -1 | 1): void {
    const target = index + step;
    this.columns.update((list) => {
      if (target < 0 || target >= list.length) return list;
      const copy = [...list];
      [copy[index], copy[target]] = [copy[target], copy[index]];
      return copy;
    });
  }

  protected dropColumn(event: CdkDragDrop<readonly ColumnDraft[]>): void {
    if (event.previousIndex === event.currentIndex) return;
    this.columns.update((list) => {
      const copy = [...list];
      const [moved] = copy.splice(event.previousIndex, 1);
      copy.splice(event.currentIndex, 0, moved);
      return copy;
    });
  }

  protected saveColumns(): void {
    this.run(
      this.store.saveColumns(
        this.columns().map((column) => ({
          id: column.id,
          name: column.name.trim(),
          color: column.color,
          minIssues: column.min,
          maxIssues: column.max,
          statusIds: column.statusIds,
        })),
      ),
      'Columnas guardadas',
    );
  }

  /* Swimlanes and quick filters */

  protected addQuery(target: 'swimlanes' | 'filters'): void {
    const draft: QueryDraft = { key: nextKey++, name: '', query: '' };
    this[target].update((list) => [...list, draft]);
  }

  protected patchQuery(
    target: 'swimlanes' | 'filters',
    key: number,
    patch: Partial<QueryDraft>,
  ): void {
    this[target].update((list) =>
      list.map((item) => (item.key === key ? { ...item, ...patch } : item)),
    );
  }

  protected removeQuery(target: 'swimlanes' | 'filters', key: number): void {
    this[target].update((list) => list.filter((item) => item.key !== key));
  }

  protected saveSwimlanes(): void {
    const mode = this.swimlaneMode();
    const lanes = this.swimlanes().map((lane) => ({
      name: lane.name.trim(),
      query: lane.query.trim(),
    }));
    const save = this.store.saveSettings({ swimlaneMode: mode });
    this.run(save, 'Carriles guardados', () => {
      if (mode !== 'query') {
        this.notify.success('Carriles guardados');
        return;
      }
      this.run(this.store.saveSwimlanes(lanes), 'Carriles guardados');
    });
  }

  protected saveFilters(): void {
    this.run(
      this.store.saveQuickFilters(
        this.filters().map((filter) => ({ name: filter.name.trim(), query: filter.query.trim() })),
      ),
      'Filtros rápidos guardados',
    );
  }

  /* Cards */

  protected toggleField(field: BoardCardField, on: boolean): void {
    this.fields.update((list) =>
      on ? (list.includes(field) ? list : [...list, field]) : list.filter((item) => item !== field),
    );
  }

  protected saveCards(): void {
    this.run(
      this.store.saveSettings({
        name: this.name().trim(),
        estimation: this.estimation(),
        cardFields: this.fields(),
        showUnmappedLane: this.showUnmapped(),
        doneRetentionDays: this.retention(),
      }),
      'Tarjetas guardadas',
    );
  }

  private load(board: BoardDetail): void {
    this.error.set(null);
    this.columns.set(
      board.columns.map((column) => ({
        key: nextKey++,
        id: column.id,
        name: column.name,
        color: column.color,
        min: column.minIssues,
        max: column.maxIssues,
        statusIds: column.statusIds,
      })),
    );
    this.swimlaneMode.set(board.swimlaneMode);
    this.swimlanes.set(
      board.swimlanes.map((lane) => ({ key: nextKey++, name: lane.name, query: lane.query })),
    );
    this.filters.set(
      board.quickFilters.map((filter) => ({
        key: nextKey++,
        name: filter.name,
        query: filter.query,
      })),
    );
    this.name.set(board.name);
    this.estimation.set(board.estimation);
    this.fields.set(board.cardFields);
    this.showUnmapped.set(board.showUnmappedLane);
    this.retention.set(board.doneRetentionDays);
  }

  private run(
    source: ReturnType<BoardViewStore['saveColumns']>,
    done: string,
    then?: () => void,
  ): void {
    this.error.set(null);
    source.subscribe({
      next: () => {
        if (then) {
          then();
        } else {
          this.notify.success(done);
        }
      },
      error: (error: Parameters<typeof boardSettingsErrorMessage>[0]) =>
        this.error.set(boardSettingsErrorMessage(error)),
    });
  }
}

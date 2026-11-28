import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import type { ReportQuery } from '@corelog/core';
import { CoreqlInput } from '@corelog/ui/coreql-input';
import { DateField } from '@corelog/ui/date-field';
import { SelectField, type SelectFieldOption } from '@corelog/ui/select-field';

import { RANGE_PRESETS, activePreset, effectiveRange, rangeForDays } from '../report-query';

const GROUP_BY_OPTIONS: Readonly<Record<string, readonly SelectFieldOption[]>> = {
  burndown: [
    { value: 'points', label: 'Puntos de historia' },
    { value: 'issues', label: 'Número de incidencias' },
  ],
  burnup: [
    { value: 'points', label: 'Puntos de historia' },
    { value: 'issues', label: 'Número de incidencias' },
  ],
  'time-tracking': [
    { value: 'issue', label: 'Por incidencia' },
    { value: 'user', label: 'Por persona' },
  ],
  'resolution-time': [
    { value: 'type', label: 'Por tipo' },
    { value: 'priority', label: 'Por prioridad' },
  ],
};

const INTERVAL_OPTIONS: readonly SelectFieldOption[] = [
  { value: 'day', label: 'Por día' },
  { value: 'week', label: 'Por semana' },
  { value: 'month', label: 'Por mes' },
];

const LIMIT_OPTIONS: readonly SelectFieldOption[] = ['3', '6', '10', '20'].map((value) => ({
  value,
  label: `Últimos ${value} sprints`,
}));

const DEFAULT_LIMIT = '6';

let nextId = 0;

/**
 * Parameters of a report, built from what the report accepts: date range with presets, sprint, version,
 * grouping, interval, number of sprints and a CoreQL filter. Every change emits the whole new query; the
 * page keeps it in the URL. The filter is applied with Ctrl+Enter or its button.
 */
@Component({
  selector: 'app-report-params-bar',
  imports: [FormsModule, CoreqlInput, DateField, SelectField],
  templateUrl: './report-params-bar.html',
  styleUrl: './report-params-bar.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ReportParamsBar {
  private readonly idBase = `report-params-${nextId++}`;

  readonly kind = input.required<string>();
  /** Parameters the report accepts, from the catalog. */
  readonly params = input.required<readonly string[]>();
  readonly query = input.required<ReportQuery>();
  readonly sprints = input<readonly SelectFieldOption[]>([]);
  readonly versions = input<readonly SelectFieldOption[]>([]);
  /** Today as ISO date, injected so tests are deterministic. */
  readonly today = input.required<string>();

  readonly queryChange = output<ReportQuery>();

  protected readonly presets = RANGE_PRESETS;
  protected readonly intervalOptions = INTERVAL_OPTIONS;
  protected readonly limitOptions = LIMIT_OPTIONS;
  protected readonly filterDraft = signal('');

  protected readonly hasRange = computed(
    () => this.params().includes('from') && this.params().includes('to'),
  );
  protected readonly has = (name: string): boolean => this.params().includes(name);
  protected readonly range = computed(() => effectiveRange(this.query(), this.today()));
  protected readonly preset = computed(() => activePreset(this.query(), this.today()));
  protected readonly groupOptions = computed(() => GROUP_BY_OPTIONS[this.kind()] ?? []);
  protected readonly groupValue = computed(
    () => this.query().groupBy ?? this.groupOptions()[0]?.value ?? null,
  );
  protected readonly intervalValue = computed(() => this.query().interval ?? 'day');
  protected readonly limitValue = computed(() => String(this.query().limit ?? DEFAULT_LIMIT));
  /** Without a sprint the server uses the active one, or the last started; time tracking then covers the whole range. */
  protected readonly sprintPlaceholder = computed(() =>
    this.kind() === 'time-tracking' ? 'Todos los sprints' : 'Sprint activo o último',
  );
  protected readonly filterDirty = computed(
    () => this.filterDraft() !== (this.query().filter ?? ''),
  );

  protected id(name: string): string {
    return `${this.idBase}-${name}`;
  }

  constructor() {
    effect(() => {
      const filter = this.query().filter ?? '';
      untracked(() => this.filterDraft.set(filter));
    });
  }

  protected patch(change: Partial<Record<keyof ReportQuery, string | number | null>>): void {
    const next: Record<string, string | number> = { ...this.query() };
    for (const [key, value] of Object.entries(change)) {
      if (value === null || value === undefined || value === '') delete next[key];
      else next[key] = value;
    }
    this.queryChange.emit(next as ReportQuery);
  }

  protected applyPreset(days: number): void {
    this.patch(rangeForDays(days, this.today()));
  }

  protected setLimit(value: string | null): void {
    this.patch({ limit: value ? Number(value) : null });
  }

  protected applyFilter(): void {
    this.patch({ filter: this.filterDraft().trim() });
  }
}

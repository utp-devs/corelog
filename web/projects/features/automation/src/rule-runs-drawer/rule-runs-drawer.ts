import { DatePipe } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  AutomationApiService,
  issuePath,
  type AutomationRule,
  type AutomationRun,
  type AutomationRunStatus,
} from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { ErrorState } from '@corelog/ui/error-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { SidePanel } from '@corelog/ui/side-panel';
import { ToneTag, type Tone } from '@corelog/ui/tone-tag';
import { Button } from 'primeng/button';

type Filter = 'all' | AutomationRunStatus;
type LoadStatus = 'loading' | 'ready' | 'error';

const LABELS: Readonly<Record<AutomationRunStatus, string>> = {
  success: 'Correcta',
  skipped: 'Omitida',
  failed: 'Fallida',
};
const TONES: Readonly<Record<AutomationRunStatus, Tone>> = {
  success: 'green',
  skipped: 'neutral',
  failed: 'red',
};

/** Execution history of a rule: status, detail and a link to the issue each run touched. */
@Component({
  selector: 'app-rule-runs-drawer',
  imports: [
    Button,
    DatePipe,
    EmptyState,
    ErrorState,
    LoadingSkeleton,
    RouterLink,
    SidePanel,
    ToneTag,
  ],
  templateUrl: './rule-runs-drawer.html',
  styleUrl: './rule-runs-drawer.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RuleRunsDrawer {
  private readonly api = inject(AutomationApiService);

  readonly rule = input<AutomationRule | null>(null);
  readonly closed = output<void>();

  protected readonly status = signal<LoadStatus>('loading');
  protected readonly runs = signal<readonly AutomationRun[]>([]);
  protected readonly filter = signal<Filter>('all');
  protected readonly filters: readonly { value: Filter; label: string }[] = [
    { value: 'all', label: 'Todas' },
    { value: 'success', label: 'Correctas' },
    { value: 'failed', label: 'Fallidas' },
    { value: 'skipped', label: 'Omitidas' },
  ];
  protected readonly labels = LABELS;
  protected readonly tones = TONES;
  protected readonly issuePath = issuePath;

  constructor() {
    effect(() => {
      const rule = this.rule();
      const filter = this.filter();
      if (!rule) return;
      untracked(() => this.load(rule.id, filter));
    });
  }

  protected setFilter(filter: Filter): void {
    this.filter.set(filter);
  }

  protected reload(): void {
    const rule = this.rule();
    if (rule) this.load(rule.id, this.filter());
  }

  private load(id: string, filter: Filter): void {
    this.status.set('loading');
    this.api.runs(id, { status: filter === 'all' ? undefined : filter, limit: 100 }).subscribe({
      next: (runs) => {
        this.runs.set(runs);
        this.status.set('ready');
      },
      error: () => this.status.set('error'),
    });
  }
}

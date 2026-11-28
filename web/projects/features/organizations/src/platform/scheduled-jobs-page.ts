import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ScheduledJobStore, type ScheduledJob } from '@corelog/core';
import { DataTable, DataTableCell, type DataTableColumn } from '@corelog/ui/data-table';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader, PageHeaderActions } from '@corelog/ui/page-header';
import { ToneTag, type Tone } from '@corelog/ui/tone-tag';
import { Button } from 'primeng/button';
import { ToggleSwitch } from 'primeng/toggleswitch';

import {
  RUN_STATUS_LABELS,
  RUN_STATUS_TONES,
  durationLabel,
  exactTimeLabel,
  relativeTimeLabel,
  scheduleLabel,
} from './platform-format';

/** Seconds between automatic refreshes. */
export const JOBS_REFRESH_SECONDS = 10;

const COLUMNS: DataTableColumn[] = [
  { key: 'name', title: 'Trabajo', field: 'name', mobilePrimary: true },
  { key: 'status', title: 'Última ejecución', mobilePrimary: true },
  { key: 'schedule', title: 'Frecuencia' },
  { key: 'started', title: 'Empezó' },
  { key: 'duration', title: 'Duración', align: 'right' },
  { key: 'success', title: 'Último éxito' },
  { key: 'error', title: 'Error', ellipsis: true },
];

/** Scheduled background jobs of the platform and how their last run went. */
@Component({
  selector: 'app-scheduled-jobs-page',
  imports: [
    Button,
    DataTable,
    DataTableCell,
    FormsModule,
    PageContainer,
    PageHeader,
    PageHeaderActions,
    ToggleSwitch,
    ToneTag,
  ],
  templateUrl: './scheduled-jobs-page.html',
  styleUrl: './scheduled-jobs-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ScheduledJobsPage {
  protected readonly store = inject(ScheduledJobStore);

  protected readonly columns = COLUMNS;
  protected readonly refreshSeconds = JOBS_REFRESH_SECONDS;
  protected readonly autoRefresh = signal(true);
  protected readonly now = signal(new Date());

  protected readonly rows = computed(() => this.store.jobs().slice());
  protected readonly loadError = computed(() => {
    const failure = this.store.error();
    return failure && !this.store.loaded() ? failure.message : null;
  });
  protected readonly refreshFailed = computed(
    () => this.store.error() !== null && this.store.loaded(),
  );

  constructor() {
    void this.store.load();
    effect((onCleanup) => {
      if (!this.autoRefresh()) return;
      const timer = setInterval(() => {
        this.now.set(new Date());
        void this.store.load(true);
      }, JOBS_REFRESH_SECONDS * 1000);
      onCleanup(() => clearInterval(timer));
    });
  }

  protected refresh(): void {
    this.now.set(new Date());
    void this.store.load();
  }

  protected statusLabel(job: ScheduledJob): string {
    return job.lastRun ? RUN_STATUS_LABELS[job.lastRun.status] : 'Sin ejecuciones';
  }

  protected statusTone(job: ScheduledJob): Tone {
    return job.lastRun ? RUN_STATUS_TONES[job.lastRun.status] : 'neutral';
  }

  protected schedule(job: ScheduledJob): string {
    return scheduleLabel(job.schedule);
  }

  protected duration(job: ScheduledJob): string {
    return durationLabel(job.lastRun?.durationMs);
  }

  protected ago(iso: string | null | undefined): string {
    return relativeTimeLabel(iso, this.now());
  }

  protected exact(iso: string | null | undefined): string {
    return exactTimeLabel(iso);
  }
}

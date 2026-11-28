import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  computed,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import {
  CurrentProjectStore,
  ImportExportApiService,
  asApiError,
  downloadFile,
  type ImportJob,
  type ImportPreview,
} from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import {
  MappingTable,
  missingRequired,
  pruneMapping,
  type FieldMapping,
} from '@corelog/ui/mapping-table';
import { ProgressBar } from '@corelog/ui/progress-bar';
import { Section } from '@corelog/ui/section';
import { Stepper, type StepperStep } from '@corelog/ui/stepper';
import { Button } from 'primeng/button';
import { firstValueFrom } from 'rxjs';

import {
  MAX_IMPORT_BYTES,
  STATUS_TEXT,
  checkImportFile,
  formatBytes,
  importIssuesQuery,
  isFinished,
  pollDelay,
  progressPercent,
} from '../import-flow';

type Step = 0 | 1 | 2 | 3;

const STEPS: readonly StepperStep[] = [
  { label: 'Archivo' },
  { label: 'Columnas' },
  { label: 'Revisión' },
  { label: 'Importación' },
];

/** Consecutive failed progress reads tolerated before the person is told. */
const MAX_POLL_FAILURES = 3;

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Import of issues from a CSV file in four steps: upload, map the columns, review a dry run (counts and the rows
 * that would fail) and the real import with live progress. A file is validated by the server once: the dry run
 * and the real import are two jobs over the same file, so the wizard keeps the file until it ends.
 */
@Component({
  selector: 'app-import-page',
  imports: [
    Button,
    EmptyState,
    LoadingSkeleton,
    MappingTable,
    ProgressBar,
    RouterLink,
    Section,
    Stepper,
  ],
  templateUrl: './import-page.html',
  styleUrl: './import-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ImportPage {
  private readonly api = inject(ImportExportApiService);
  private readonly current = inject(CurrentProjectStore);
  private readonly destroyRef = inject(DestroyRef);
  private readonly picker = viewChild<ElementRef<HTMLInputElement>>('picker');
  private destroyed = false;

  protected readonly steps = STEPS;
  protected readonly maxText = formatBytes(MAX_IMPORT_BYTES);
  protected readonly statusText = STATUS_TEXT;
  protected readonly formatBytes = formatBytes;
  protected readonly percent = progressPercent;

  protected readonly step = signal<Step>(0);
  protected readonly file = signal<File | null>(null);
  protected readonly preview = signal<ImportPreview | null>(null);
  protected readonly mapping = signal<FieldMapping>({});
  protected readonly dryRun = signal<ImportJob | null>(null);
  protected readonly result = signal<ImportJob | null>(null);
  protected readonly busy = signal(false);
  protected readonly error = signal<string | null>(null);
  protected readonly dragging = signal(false);
  protected readonly downloading = signal(false);

  protected readonly canImport = computed(
    () => this.current.can('edit_project') && !this.current.isArchived(),
  );
  protected readonly archived = this.current.isArchived;
  protected readonly ready = computed(() => this.current.permissionsLoaded());
  protected readonly mappingReady = computed(() => missingRequired(this.mapping()).length === 0);
  protected readonly issuesLink = computed(() => {
    const project = this.current.project();
    const job = this.result();
    return project && job ? importIssuesQuery(project.key, job.createdAt) : null;
  });

  constructor() {
    this.destroyRef.onDestroy(() => (this.destroyed = true));
  }

  // Step 1.

  protected openPicker(): void {
    this.picker()?.nativeElement.click();
  }

  protected onPicked(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.item(0);
    input.value = '';
    if (file) void this.upload(file);
  }

  protected onDragOver(event: DragEvent): void {
    event.preventDefault();
    this.dragging.set(true);
  }

  protected onDrop(event: DragEvent): void {
    event.preventDefault();
    this.dragging.set(false);
    const file = event.dataTransfer?.files.item(0);
    if (file) void this.upload(file);
  }

  protected async upload(file: File): Promise<void> {
    const project = this.current.projectId();
    if (!project || this.busy()) return;
    const problem = checkImportFile(file);
    if (problem) {
      this.error.set(problem);
      return;
    }
    this.busy.set(true);
    this.error.set(null);
    try {
      const preview = await firstValueFrom(this.api.upload(project, file));
      this.file.set(file);
      this.preview.set(preview);
      this.mapping.set(pruneMapping(preview.suggestedMapping, preview.columns));
      this.step.set(1);
    } catch (failure) {
      this.error.set(asApiError(failure).message);
    } finally {
      this.busy.set(false);
    }
  }

  // Step 2 and 3.

  protected async review(): Promise<void> {
    const project = this.current.projectId();
    const preview = this.preview();
    if (!project || !preview || this.busy() || !this.mappingReady()) return;
    this.busy.set(true);
    this.error.set(null);
    try {
      const started = await firstValueFrom(
        this.api.start(project, preview.jobId, this.mapping(), true),
      );
      this.dryRun.set(started);
      const done = await this.waitFor(project, started.id, (job) => this.dryRun.set(job));
      this.dryRun.set(done);
      this.step.set(2);
    } catch (failure) {
      this.error.set(asApiError(failure).message);
    } finally {
      this.busy.set(false);
    }
  }

  protected backTo(step: number): void {
    if (this.busy() || step >= this.step() || this.step() === 3) return;
    this.error.set(null);
    this.step.set(step as Step);
  }

  // Step 4.

  protected async runImport(): Promise<void> {
    const project = this.current.projectId();
    const file = this.file();
    if (!project || !file || this.busy()) return;
    this.busy.set(true);
    this.error.set(null);
    this.result.set(null);
    this.step.set(3);
    try {
      // The dry run consumed the first job: the same file becomes a new one for the real import.
      const preview = await firstValueFrom(this.api.upload(project, file));
      const started = await firstValueFrom(
        this.api.start(
          project,
          preview.jobId,
          pruneMapping(this.mapping(), preview.columns),
          false,
        ),
      );
      this.result.set(started);
      const done = await this.waitFor(project, started.id, (job) => this.result.set(job));
      this.result.set(done);
    } catch (failure) {
      this.error.set(asApiError(failure).message);
    } finally {
      this.busy.set(false);
    }
  }

  private async waitFor(
    project: string,
    jobId: string,
    onUpdate: (job: ImportJob) => void,
  ): Promise<ImportJob> {
    let attempt = 0;
    let failures = 0;
    for (;;) {
      try {
        const job = await firstValueFrom(this.api.job(project, jobId));
        failures = 0;
        onUpdate(job);
        if (isFinished(job)) return job;
      } catch (failure) {
        failures += 1;
        if (failures >= MAX_POLL_FAILURES) throw failure;
      }
      await sleep(pollDelay(attempt));
      attempt += 1;
      if (this.destroyed) throw new Error('cancelled');
    }
  }

  protected async downloadErrors(job: ImportJob): Promise<void> {
    const project = this.current.projectId();
    if (!project || this.downloading()) return;
    this.downloading.set(true);
    try {
      const file = await firstValueFrom(this.api.errorsCsv(project, job.id));
      downloadFile(file.blob, file.fileName);
    } catch (failure) {
      this.error.set(asApiError(failure).message);
    } finally {
      this.downloading.set(false);
    }
  }

  protected restart(): void {
    this.file.set(null);
    this.preview.set(null);
    this.mapping.set({});
    this.dryRun.set(null);
    this.result.set(null);
    this.error.set(null);
    this.step.set(0);
  }
}

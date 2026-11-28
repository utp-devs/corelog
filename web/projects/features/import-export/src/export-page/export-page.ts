import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import {
  CurrentProjectStore,
  ImportExportApiService,
  asApiError,
  downloadFile,
  type DownloadedFile,
} from '@corelog/core';
import { CoreqlField } from '@corelog/ui/coreql-field';
import { EmptyState } from '@corelog/ui/empty-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { Section } from '@corelog/ui/section';
import { Button } from 'primeng/button';
import { firstValueFrom } from 'rxjs';

import { formatBytes } from '../import-flow';

type Kind = 'csv' | 'json';

/** Spanish text for the answers the export endpoints give when a size cap or a bad filter stops the download. */
export function exportErrorMessage(status: number, code: string, fallback: string): string {
  if (status === 422 && code.includes('too_large')) {
    return 'El proyecto es demasiado grande para exportarlo de una vez. Usa un filtro para reducir las incidencias.';
  }
  if (status === 422) return `${fallback} Revisa el filtro CoreQL.`;
  if (status === 403) return 'No tienes permiso para exportar este proyecto.';
  return fallback;
}

/**
 * Export of the project: the issues as CSV (optionally narrowed by a CoreQL filter) and a full JSON copy of the
 * project. Files are fetched with the session and saved from memory, so the size is shown once they arrive.
 */
@Component({
  selector: 'app-export-page',
  imports: [Button, CoreqlField, EmptyState, LoadingSkeleton, Section],
  templateUrl: './export-page.html',
  styleUrl: './export-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ExportPage {
  private readonly api = inject(ImportExportApiService);
  private readonly current = inject(CurrentProjectStore);

  protected readonly filter = signal('');
  protected readonly busy = signal<Kind | null>(null);
  protected readonly error = signal<{ readonly kind: Kind; readonly message: string } | null>(null);
  protected readonly done = signal<{
    readonly kind: Kind;
    readonly name: string;
    readonly size: string;
  } | null>(null);

  protected readonly ready = computed(() => this.current.permissionsLoaded());
  protected readonly canCsv = computed(() => this.current.can('export_issues'));
  protected readonly canJson = computed(() => this.current.can('edit_project'));
  protected readonly allowed = computed(() => this.canCsv() || this.canJson());

  protected async export(kind: Kind): Promise<void> {
    const project = this.current.projectId();
    if (!project || this.busy()) return;
    this.busy.set(kind);
    this.error.set(null);
    this.done.set(null);
    try {
      const file: DownloadedFile = await firstValueFrom(
        kind === 'csv'
          ? this.api.exportIssues(project, this.filter())
          : this.api.exportProject(project),
      );
      downloadFile(file.blob, file.fileName);
      this.done.set({ kind, name: file.fileName, size: formatBytes(file.blob.size) });
    } catch (failure) {
      const failed = asApiError(failure);
      this.error.set({
        kind,
        message: exportErrorMessage(failed.status, failed.code, failed.message),
      });
    } finally {
      this.busy.set(null);
    }
  }
}

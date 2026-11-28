import type { ImportJob, ImportJobStatus } from '@corelog/core';

/** Largest file the server accepts. */
export const MAX_IMPORT_BYTES = 10 * 1024 * 1024;

/** Message for a file that cannot be imported, or null when it can be uploaded. */
export function checkImportFile(file: Pick<File, 'name' | 'size'>): string | null {
  if (!/\.(csv|txt)$/i.test(file.name)) return 'El archivo debe ser un CSV (extensión .csv).';
  if (file.size === 0) return 'El archivo está vacío.';
  if (file.size > MAX_IMPORT_BYTES) return 'El archivo supera el máximo de 10 MiB.';
  return null;
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Number((bytes / 1024).toFixed(1))} KiB`;
  return `${Number((bytes / (1024 * 1024)).toFixed(1))} MiB`;
}

const FINISHED: readonly ImportJobStatus[] = ['done', 'failed'];

export function isFinished(job: Pick<ImportJob, 'status'>): boolean {
  return FINISHED.includes(job.status);
}

/** Wait before asking for the progress again: every 2 s at first, slowing down after a while up to 8 s. */
export function pollDelay(attempt: number): number {
  return attempt < 5 ? 2000 : Math.min(2000 + (attempt - 4) * 500, 8000);
}

/** Share of rows handled, 0 to 100. */
export function progressPercent(
  job: Pick<ImportJob, 'processedRows' | 'totalRows' | 'status'>,
): number {
  if (job.status === 'done') return 100;
  if (job.totalRows <= 0) return 0;
  return Math.min(100, Math.round((job.processedRows / job.totalRows) * 100));
}

export const STATUS_TEXT: Readonly<Record<ImportJobStatus, string>> = {
  queued: 'En cola',
  validating: 'Preparando',
  running: 'Importando',
  done: 'Terminada',
  failed: 'Con errores',
};

/**
 * Search that lists what an import created: the project's issues created since the job started. The window is
 * relative (minutes back from now), so it does not depend on the time zone of the viewer.
 */
export function importIssuesQuery(
  projectKey: string,
  startedAt: string,
  now: Date = new Date(),
): string {
  const minutes = Math.max(
    1,
    Math.ceil((now.getTime() - new Date(startedAt).getTime()) / 60000) + 1,
  );
  return `project = ${projectKey} AND created >= -${minutes}m ORDER BY created DESC`;
}

export type ImportJobStatus = 'queued' | 'validating' | 'running' | 'done' | 'failed';

export interface ImportRowError {
  readonly row: number;
  readonly errors: readonly string[];
}

/** First look at an uploaded CSV: its columns, a few rows and the mapping the server suggests. */
export interface ImportPreview {
  readonly jobId: string;
  readonly columns: readonly string[];
  readonly sampleRows: readonly (readonly string[])[];
  /** CoreLog field -> CSV column. */
  readonly suggestedMapping: Readonly<Record<string, string>>;
  readonly totalRows: number;
  readonly delimiter: string;
}

export interface ImportJob {
  readonly id: string;
  readonly status: ImportJobStatus;
  readonly totalRows: number;
  readonly processedRows: number;
  readonly createdRows: number;
  readonly errorRows: number;
  readonly dryRun: boolean;
  readonly fileName: string;
  readonly errors: readonly ImportRowError[];
  readonly createdAt: string;
  readonly finishedAt: string | null;
}

/** A downloaded file: its content and the name the server proposes. */
export interface DownloadedFile {
  readonly blob: Blob;
  readonly fileName: string;
}

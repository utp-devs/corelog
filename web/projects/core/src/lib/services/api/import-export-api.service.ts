import { HttpClient, HttpErrorResponse, HttpParams, HttpResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, catchError, from, map, switchMap, throwError } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { SKIP_LOADING } from '../../config/http-context';
import type {
  DownloadedFile,
  ImportJob,
  ImportJobStatus,
  ImportPreview,
  ImportRowError,
} from '../../models';
import { backgroundRequest, silentRequest } from '../../utils/silent-request';

interface JobDto {
  id: string;
  status: ImportJobStatus;
  total_rows?: number;
  processed_rows?: number;
  created_rows?: number;
  error_rows?: number;
  dry_run?: boolean;
  file_name?: string;
  errors?: { row: number; errors?: string[] }[] | null;
  created_at: string;
  finished_at?: string | null;
}

interface PreviewDto {
  job_id: string;
  columns?: string[];
  sample_rows?: string[][];
  suggested_mapping?: Record<string, string>;
  total_rows?: number;
  delimiter?: string;
}

function toJob(dto: JobDto): ImportJob {
  const errors: ImportRowError[] = (dto.errors ?? []).map((e) => ({
    row: e.row,
    errors: e.errors ?? [],
  }));
  return {
    id: dto.id,
    status: dto.status,
    totalRows: dto.total_rows ?? 0,
    processedRows: dto.processed_rows ?? 0,
    createdRows: dto.created_rows ?? 0,
    errorRows: dto.error_rows ?? 0,
    dryRun: dto.dry_run ?? false,
    fileName: dto.file_name ?? '',
    errors,
    createdAt: dto.created_at,
    finishedAt: dto.finished_at ?? null,
  };
}

/** Name proposed by a Content-Disposition header, or the fallback. */
export function fileNameOf(response: HttpResponse<unknown>, fallback: string): string {
  const header = response.headers.get('Content-Disposition') ?? '';
  const match = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(header);
  return match ? decodeURIComponent(match[1]) : fallback;
}

/** A failed download carries its error body as a Blob: read it so the normal error mapping can use it. */
function readBlobError(error: unknown): Observable<never> {
  if (error instanceof HttpErrorResponse && error.error instanceof Blob) {
    return from(error.error.text()).pipe(
      switchMap((text) => {
        let body: unknown = null;
        try {
          body = JSON.parse(text);
        } catch {
          body = null;
        }
        return throwError(
          () =>
            new HttpErrorResponse({
              error: body,
              status: error.status,
              statusText: error.statusText,
              url: error.url ?? undefined,
              headers: error.headers,
            }),
        );
      }),
    );
  }
  return throwError(() => error);
}

/** CSV import of issues and exports of a project. Downloads travel through the authenticated client. */
@Injectable({ providedIn: 'root' })
export class ImportExportApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  private path(project: string): string {
    return `${this.baseUrl}/projects/${encodeURIComponent(project)}`;
  }

  upload(project: string, file: File): Observable<ImportPreview> {
    const form = new FormData();
    form.append('file', file, file.name);
    return this.http
      .post<{ data: PreviewDto }>(`${this.path(project)}/import/csv`, form, {
        context: silentRequest(),
      })
      .pipe(
        map(({ data }) => ({
          jobId: data.job_id,
          columns: data.columns ?? [],
          sampleRows: data.sample_rows ?? [],
          suggestedMapping: data.suggested_mapping ?? {},
          totalRows: data.total_rows ?? 0,
          delimiter: data.delimiter ?? ',',
        })),
      );
  }

  start(
    project: string,
    jobId: string,
    mapping: Readonly<Record<string, string>>,
    dryRun: boolean,
  ): Observable<ImportJob> {
    const url = `${this.path(project)}/import/csv/${encodeURIComponent(jobId)}/start`;
    return this.http
      .post<{ data: JobDto }>(url, { mapping, dry_run: dryRun }, { context: silentRequest() })
      .pipe(map(({ data }) => toJob(data)));
  }

  /** Progress read; it does not move the global loading bar because it is polled. */
  job(project: string, jobId: string): Observable<ImportJob> {
    const url = `${this.path(project)}/import/jobs/${encodeURIComponent(jobId)}`;
    return this.http
      .get<{ data: JobDto }>(url, { context: backgroundRequest() })
      .pipe(map(({ data }) => toJob(data)));
  }

  jobs(project: string): Observable<readonly ImportJob[]> {
    return this.http
      .get<{ data: JobDto[] | null }>(`${this.path(project)}/import/jobs`, {
        context: silentRequest(),
      })
      .pipe(map((body) => (body.data ?? []).map(toJob)));
  }

  errorsCsv(project: string, jobId: string): Observable<DownloadedFile> {
    return this.download(
      `${this.path(project)}/import/jobs/${encodeURIComponent(jobId)}/errors.csv`,
      'errores-importacion.csv',
    );
  }

  exportIssues(project: string, filter: string): Observable<DownloadedFile> {
    let params = new HttpParams();
    if (filter.trim()) params = params.set('filter', filter.trim());
    return this.download(`${this.path(project)}/export/issues.csv`, 'incidencias.csv', params);
  }

  exportProject(project: string): Observable<DownloadedFile> {
    return this.download(`${this.path(project)}/export/project.json`, 'proyecto.json');
  }

  private download(url: string, fallback: string, params?: HttpParams): Observable<DownloadedFile> {
    return this.http
      .get(url, {
        params,
        responseType: 'blob',
        observe: 'response',
        context: silentRequest().set(SKIP_LOADING, true),
      })
      .pipe(
        catchError(readBlobError),
        map((response) => ({
          blob: response.body ?? new Blob([]),
          fileName: fileNameOf(response, fallback),
        })),
      );
  }
}

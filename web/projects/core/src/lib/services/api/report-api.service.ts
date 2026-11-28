import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, catchError, from, map, switchMap, throwError } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type { Report, ReportDescriptor, ReportQuery, ReportTable } from '../../models/report.model';
import { silentRequest } from '../../utils/silent-request';

interface ReportDto {
  kind: string;
  title: string;
  generated_at: string;
  params?: Record<string, unknown> | null;
  series?: Report['series'] | null;
  table?: Partial<ReportTable> | null;
  summary?: Record<string, unknown> | null;
}

/** Query string parameters of a report; empty values are left out. */
export function reportParams(query: ReportQuery): Record<string, string> {
  const entries: [string, string | number | undefined][] = [
    ['from', query.from],
    ['to', query.to],
    ['group_by', query.groupBy],
    ['filter', query.filter],
    ['sprint', query.sprint],
    ['version', query.version],
    ['interval', query.interval],
    ['limit', query.limit],
  ];
  const params: Record<string, string> = {};
  for (const [key, value] of entries) {
    if (value !== undefined && value !== '') params[key] = String(value);
  }
  return params;
}

export function toReport(dto: ReportDto): Report {
  return {
    kind: dto.kind,
    title: dto.title,
    generatedAt: dto.generated_at,
    params: dto.params ?? {},
    series: dto.series ?? [],
    table: { columns: dto.table?.columns ?? [], rows: dto.table?.rows ?? [] },
    summary: dto.summary ?? {},
  };
}

function parseJson(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

/** Project reports: the catalog, one report and its CSV export. */
@Injectable({ providedIn: 'root' })
export class ReportApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(projectId: string): Observable<ReportDescriptor[]> {
    return this.http
      .get<{ data: ReportDescriptor[] }>(`${this.baseUrl}/projects/${projectId}/reports`, {
        context: silentRequest(),
      })
      .pipe(map((body) => body.data));
  }

  get(projectId: string, kind: string, query: ReportQuery = {}): Observable<Report> {
    return this.http
      .get<{ data: ReportDto }>(`${this.baseUrl}/projects/${projectId}/reports/${kind}`, {
        params: reportParams(query),
        context: silentRequest(),
      })
      .pipe(map((body) => toReport(body.data)));
  }

  /** The table of a report as CSV (UTF-8 with BOM). A failure keeps the JSON error body of the server. */
  csv(projectId: string, kind: string, query: ReportQuery = {}): Observable<Blob> {
    return this.http
      .get(`${this.baseUrl}/projects/${projectId}/reports/${kind}.csv`, {
        params: reportParams(query),
        responseType: 'blob',
        context: silentRequest(),
      })
      .pipe(
        catchError((error: unknown) => {
          if (error instanceof HttpErrorResponse && error.error instanceof Blob) {
            return from(error.error.text()).pipe(
              switchMap((text) =>
                throwError(
                  () =>
                    new HttpErrorResponse({
                      status: error.status,
                      statusText: error.statusText,
                      url: error.url ?? undefined,
                      error: parseJson(text),
                    }),
                ),
              ),
            );
          }
          return throwError(() => error);
        }),
      );
  }
}

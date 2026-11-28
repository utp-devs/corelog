import { HttpClient, HttpContext } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { SKIP_ERROR_NOTIFICATION } from '../../config/http-context';
import type {
  MoveUnresolvedTo,
  ReleaseVersion,
  ReleaseNotes,
  ReleaseResult,
  VersionPayload,
  VersionStatus,
} from '../../models/version.model';

interface VersionDto {
  id: string;
  project_id: string;
  name: string;
  description?: string;
  start_date?: string | null;
  release_date?: string | null;
  status: VersionStatus;
  released_at?: string | null;
  issues_total?: number;
  issues_done?: number;
  progress?: {
    issues?: {
      total: number;
      done: number;
      by_category?: { todo?: number; in_progress?: number; done?: number };
    };
    points?: { total: number; done: number };
  };
}

export function toReleaseVersion(dto: VersionDto): ReleaseVersion {
  const issues = dto.progress?.issues;
  return {
    id: dto.id,
    projectId: dto.project_id,
    name: dto.name,
    description: dto.description ?? '',
    startDate: dto.start_date ?? null,
    releaseDate: dto.release_date ?? null,
    status: dto.status,
    releasedAt: dto.released_at ?? null,
    progress: {
      total: issues?.total ?? dto.issues_total ?? 0,
      done: issues?.done ?? dto.issues_done ?? 0,
      inProgress: issues?.by_category?.in_progress ?? 0,
      todo: issues?.by_category?.todo ?? 0,
      pointsTotal: dto.progress?.points?.total ?? 0,
      pointsDone: dto.progress?.points?.done ?? 0,
    },
  };
}

function body(payload: Partial<VersionPayload>): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  if (payload.name !== undefined) result['name'] = payload.name;
  if (payload.description !== undefined) result['description'] = payload.description;
  if (payload.startDate !== undefined) result['start_date'] = payload.startDate;
  if (payload.releaseDate !== undefined) result['release_date'] = payload.releaseDate;
  return result;
}

/** Versions (releases) of a project: list, create, edit, release, archive and release notes. */
@Injectable({ providedIn: 'root' })
export class VersionApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);
  private readonly quiet = (): HttpContext => new HttpContext().set(SKIP_ERROR_NOTIFICATION, true);

  list(projectId: string): Observable<ReleaseVersion[]> {
    return this.http
      .get<{ data: VersionDto[] }>(`${this.baseUrl}/projects/${projectId}/versions`, {
        context: this.quiet(),
      })
      .pipe(map(({ data }) => data.map(toReleaseVersion)));
  }

  create(projectId: string, payload: VersionPayload): Observable<ReleaseVersion> {
    return this.http
      .post<{ data: VersionDto }>(`${this.baseUrl}/projects/${projectId}/versions`, body(payload), {
        context: this.quiet(),
      })
      .pipe(map(({ data }) => toReleaseVersion(data)));
  }

  update(
    id: string,
    payload: Partial<VersionPayload> & { reopen?: boolean },
  ): Observable<ReleaseVersion> {
    const { reopen, ...rest } = payload;
    return this.http
      .patch<{ data: VersionDto }>(
        `${this.baseUrl}/versions/${id}`,
        { ...body(rest), ...(reopen ? { status: 'unreleased' } : {}) },
        { context: this.quiet() },
      )
      .pipe(map(({ data }) => toReleaseVersion(data)));
  }

  delete(id: string, moveTo?: string): Observable<void> {
    const suffix = moveTo ? `?move_to=${encodeURIComponent(moveTo)}` : '';
    return this.http.delete<void>(`${this.baseUrl}/versions/${id}${suffix}`, {
      context: this.quiet(),
    });
  }

  /** Releases the version. `moveUnresolvedTo`: null keeps the open issues, `new` creates the next version. */
  release(
    id: string,
    moveUnresolvedTo: MoveUnresolvedTo,
    releaseDate?: string,
  ): Observable<ReleaseResult> {
    return this.http
      .post<{
        data: {
          version: VersionDto;
          moved_issues: number;
          unresolved_issues: number;
          next_version: VersionDto | null;
        };
      }>(
        `${this.baseUrl}/versions/${id}/release`,
        {
          move_unresolved_to: moveUnresolvedTo,
          ...(releaseDate ? { release_date: releaseDate } : {}),
        },
        { context: this.quiet() },
      )
      .pipe(
        map(({ data }) => ({
          version: toReleaseVersion(data.version),
          movedIssues: data.moved_issues ?? 0,
          unresolvedIssues: data.unresolved_issues ?? 0,
          nextVersion: data.next_version ? toReleaseVersion(data.next_version) : null,
        })),
      );
  }

  archive(id: string): Observable<ReleaseVersion> {
    return this.http
      .post<{ data: VersionDto }>(`${this.baseUrl}/versions/${id}/archive`, null, {
        context: this.quiet(),
      })
      .pipe(map(({ data }) => toReleaseVersion(data)));
  }

  notes(id: string): Observable<ReleaseNotes> {
    return this.http
      .get<{
        data: {
          version_name: string;
          status: VersionStatus;
          release_date: string | null;
          issue_count: number;
          markdown: string;
          groups: ReleaseNotes['groups'] | null;
        };
      }>(`${this.baseUrl}/versions/${id}/notes`, { context: this.quiet() })
      .pipe(
        map(({ data }) => ({
          versionName: data.version_name,
          status: data.status,
          releaseDate: data.release_date,
          issueCount: data.issue_count,
          markdown: data.markdown,
          groups: data.groups ?? [],
        })),
      );
  }
}

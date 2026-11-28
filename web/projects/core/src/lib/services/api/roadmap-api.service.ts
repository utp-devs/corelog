import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type {
  Roadmap,
  RoadmapBar,
  RoadmapDependency,
  RoadmapGroupBy,
  RoadmapQuery,
} from '../../models/roadmap.model';
import { silentRequest } from '../../utils/silent-request';

interface BarDto {
  id: string;
  kind: RoadmapBar['kind'];
  key?: string;
  title: string;
  status: string;
  status_category?: RoadmapBar['statusCategory'];
  assignee?: string;
  start: string | null;
  end: string | null;
  start_source: RoadmapBar['startSource'];
  end_source: RoadmapBar['endSource'];
  dated: boolean;
  complete: boolean;
  progress: {
    total: number;
    todo: number;
    in_progress: number;
    done: number;
    percent: number;
    points_total: number;
    points_done: number;
  };
}

interface RoadmapDto {
  from: string | null;
  to: string | null;
  group_by: RoadmapGroupBy;
  bars: BarDto[] | null;
  versions: BarDto[] | null;
  sprints: BarDto[] | null;
  dependencies: RoadmapDependency[] | null;
}

export function toRoadmapBar(dto: BarDto): RoadmapBar {
  return {
    id: dto.id,
    kind: dto.kind,
    key: dto.key ?? '',
    title: dto.title,
    status: dto.status,
    statusCategory: dto.status_category ?? 'todo',
    assignee: dto.assignee ?? '',
    start: dto.start,
    end: dto.end,
    startSource: dto.start_source,
    endSource: dto.end_source,
    dated: dto.dated,
    complete: dto.complete,
    progress: {
      total: dto.progress.total,
      todo: dto.progress.todo,
      inProgress: dto.progress.in_progress,
      done: dto.progress.done,
      percent: dto.progress.percent,
      pointsTotal: dto.progress.points_total,
      pointsDone: dto.progress.points_done,
    },
  };
}

/** Roadmap of a project: epics, versions or sprints on a timeline, with dependencies and manual order. */
@Injectable({ providedIn: 'root' })
export class RoadmapApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  /** `projectId` must be the project UUID. */
  get(projectId: string, query: RoadmapQuery = {}): Observable<Roadmap> {
    let params = new HttpParams();
    if (query.from) params = params.set('from', query.from);
    if (query.to) params = params.set('to', query.to);
    if (query.groupBy) params = params.set('group_by', query.groupBy);
    if (query.filter && query.filter !== 'all') params = params.set('filter', query.filter);
    if (query.q) params = params.set('q', query.q);
    return this.http
      .get<{ data: RoadmapDto }>(`${this.baseUrl}/projects/${projectId}/roadmap`, {
        params,
        context: silentRequest(),
      })
      .pipe(
        map(({ data }) => ({
          from: data.from,
          to: data.to,
          groupBy: data.group_by,
          bars: (data.bars ?? []).map(toRoadmapBar),
          versions: (data.versions ?? []).map(toRoadmapBar),
          sprints: (data.sprints ?? []).map(toRoadmapBar),
          dependencies: data.dependencies ?? [],
        })),
      );
  }

  /** Saves the manual order of the epics: the listed epics go first, in that order. */
  rank(projectId: string, epicIds: readonly string[]): Observable<void> {
    return this.http
      .put(`${this.baseUrl}/projects/${projectId}/roadmap/rank`, { epic_ids: epicIds })
      .pipe(map(() => undefined));
  }
}

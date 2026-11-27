import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type { BacklogItem } from '../../models/backlog.model';
import type { WorkItemPriority, WorkItemStatus } from '../../models/board.model';
import { silentRequest } from '../../utils/silent-request';

interface BacklogItemDto {
  readonly id: string;
  readonly project_id: string;
  readonly summary: string;
  readonly type: string;
  readonly status: WorkItemStatus;
  readonly priority: WorkItemPriority;
  readonly rank: string;
  readonly assignee_id?: string | null;
  readonly story_points?: number | null;
}

interface DataEnvelope<T> {
  readonly data: T;
}

function toBacklogItem(item: BacklogItemDto): BacklogItem {
  return {
    id: item.id,
    projectId: item.project_id,
    summary: item.summary,
    type: item.type,
    status: item.status,
    priority: item.priority,
    rank: item.rank,
    assigneeId: item.assignee_id ?? undefined,
    storyPoints: item.story_points ?? undefined,
  };
}

/** Backlog of a project, ordered by rank. */
@Injectable({ providedIn: 'root' })
export class BacklogApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(projectId: string): Observable<BacklogItem[]> {
    return this.http
      .get<DataEnvelope<BacklogItemDto[]>>(`${this.baseUrl}/backlog`, {
        params: { project_id: projectId },
        context: silentRequest(),
      })
      .pipe(map((response) => response.data.map(toBacklogItem)));
  }

  /** Places an issue at `position` (0 is the first item). Returns the renumbered backlog. */
  move(projectId: string, issueId: string, position: number): Observable<BacklogItem[]> {
    return this.http
      .put<DataEnvelope<BacklogItemDto[]>>(
        `${this.baseUrl}/backlog/${issueId}`,
        { position },
        { params: { project_id: projectId }, context: silentRequest() },
      )
      .pipe(map((response) => response.data.map(toBacklogItem)));
  }
}

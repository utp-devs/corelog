import { HttpClient, HttpContext } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { SKIP_ERROR_NOTIFICATION } from '../../config/http-context';

/** Who an issue is assigned to by default when it gets the component. */
export type ComponentAssigneePolicy = 'none' | 'component_lead' | 'project_lead';

/** Component of a project with its lead and its default assignee policy. */
export interface ReleaseComponent {
  readonly id: string;
  readonly projectId: string;
  readonly name: string;
  readonly description: string;
  readonly leadId: string | null;
  readonly leadName: string | null;
  readonly policy: ComponentAssigneePolicy;
  readonly issuesCount: number;
}

export interface ReleaseComponentPayload {
  readonly name: string;
  readonly description: string;
  readonly leadId: string | null;
  readonly policy: ComponentAssigneePolicy;
}

interface ComponentDto {
  id: string;
  project_id: string;
  name: string;
  description?: string;
  lead_id: string | null;
  lead_name: string | null;
  default_assignee_policy: ComponentAssigneePolicy;
  issues_count?: number;
}

function toComponent(dto: ComponentDto): ReleaseComponent {
  return {
    id: dto.id,
    projectId: dto.project_id,
    name: dto.name,
    description: dto.description ?? '',
    leadId: dto.lead_id,
    leadName: dto.lead_name,
    policy: dto.default_assignee_policy,
    issuesCount: dto.issues_count ?? 0,
  };
}

function toBody(payload: ReleaseComponentPayload): Record<string, unknown> {
  return {
    name: payload.name,
    description: payload.description,
    lead_id: payload.leadId,
    default_assignee_policy: payload.policy,
  };
}

/** Components of a project, with lead and default assignee policy (administration pages). */
@Injectable({ providedIn: 'root' })
export class ReleaseComponentsApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);
  private readonly quiet = (): HttpContext => new HttpContext().set(SKIP_ERROR_NOTIFICATION, true);

  list(projectId: string): Observable<ReleaseComponent[]> {
    return this.http
      .get<{ data: ComponentDto[] }>(`${this.baseUrl}/projects/${projectId}/components`, {
        context: this.quiet(),
      })
      .pipe(map(({ data }) => data.map(toComponent)));
  }

  create(projectId: string, payload: ReleaseComponentPayload): Observable<ReleaseComponent> {
    return this.http
      .post<{ data: ComponentDto }>(
        `${this.baseUrl}/projects/${projectId}/components`,
        toBody(payload),
        {
          context: this.quiet(),
        },
      )
      .pipe(map(({ data }) => toComponent(data)));
  }

  update(
    projectId: string,
    id: string,
    payload: ReleaseComponentPayload,
  ): Observable<ReleaseComponent> {
    return this.http
      .patch<{ data: ComponentDto }>(
        `${this.baseUrl}/projects/${projectId}/components/${id}`,
        toBody(payload),
        { context: this.quiet() },
      )
      .pipe(map(({ data }) => toComponent(data)));
  }

  /** Deletes the component; with `moveTo` its issues pass to that component. */
  delete(projectId: string, id: string, moveTo?: string): Observable<void> {
    const suffix = moveTo ? `?move_to=${encodeURIComponent(moveTo)}` : '';
    return this.http.delete<void>(
      `${this.baseUrl}/projects/${projectId}/components/${id}${suffix}`,
      {
        context: this.quiet(),
      },
    );
  }
}

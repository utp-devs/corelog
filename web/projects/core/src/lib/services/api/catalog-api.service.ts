import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  CatalogCreateInput,
  CatalogItem,
  CatalogKind,
  CatalogPatchInput,
  HierarchyLevel,
  ProjectIssueTypes,
} from '../../models/catalog.model';
import { silentRequest } from '../../utils/silent-request';

interface CatalogItemDto {
  id: string;
  name: string;
  description?: string;
  icon?: string;
  color?: string;
  hierarchy_level?: HierarchyLevel;
  position: number;
  is_default: boolean;
  usage_count: number;
  created_at: string;
  updated_at: string;
}

interface Envelope<T> {
  data: T;
}

export function toCatalogItem(dto: CatalogItemDto): CatalogItem {
  return {
    id: dto.id,
    name: dto.name,
    description: dto.description ?? '',
    icon: dto.icon || null,
    color: dto.color || null,
    hierarchyLevel: dto.hierarchy_level ?? null,
    position: dto.position,
    isDefault: dto.is_default,
    usageCount: dto.usage_count,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

/** Catalogs of an organization (issue types, priorities, resolutions) and the types of a project. */
@Injectable({ providedIn: 'root' })
export class CatalogApiService {
  private readonly http = inject(HttpClient);
  private readonly apiUrl = inject(API_URL);

  private url(orgId: string, kind: CatalogKind): string {
    return `${this.apiUrl}/organizations/${orgId}/${kind}`;
  }

  list(orgId: string, kind: CatalogKind): Observable<CatalogItem[]> {
    return this.http
      .get<Envelope<CatalogItemDto[]>>(this.url(orgId, kind), { context: silentRequest() })
      .pipe(map((body) => body.data.map(toCatalogItem)));
  }

  create(orgId: string, kind: CatalogKind, input: CatalogCreateInput): Observable<CatalogItem> {
    return this.http
      .post<Envelope<CatalogItemDto>>(
        this.url(orgId, kind),
        {
          name: input.name,
          description: input.description,
          icon: input.icon,
          color: input.color,
          hierarchy_level: input.hierarchyLevel,
          is_default: input.isDefault,
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toCatalogItem(body.data)));
  }

  update(
    orgId: string,
    kind: CatalogKind,
    id: string,
    input: CatalogPatchInput,
  ): Observable<CatalogItem> {
    return this.http
      .patch<Envelope<CatalogItemDto>>(
        `${this.url(orgId, kind)}/${id}`,
        {
          name: input.name,
          description: input.description,
          icon: input.icon,
          color: input.color,
          is_default: input.isDefault,
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toCatalogItem(body.data)));
  }

  /** Deletes an item. When issues use it, `reassignTo` names the item that receives them. */
  remove(orgId: string, kind: CatalogKind, id: string, reassignTo?: string): Observable<void> {
    let params = new HttpParams();
    if (reassignTo) {
      params = params.set('reassign_to', reassignTo);
    }
    return this.http.delete<void>(`${this.url(orgId, kind)}/${id}`, {
      params,
      context: silentRequest(),
    });
  }

  /** Sets the order of the whole catalog. `ids` must list every item once. */
  reorder(orgId: string, kind: CatalogKind, ids: readonly string[]): Observable<CatalogItem[]> {
    return this.http
      .put<Envelope<CatalogItemDto[]>>(
        `${this.url(orgId, kind)}/reorder`,
        { ids },
        { context: silentRequest() },
      )
      .pipe(map((body) => body.data.map(toCatalogItem)));
  }

  getProjectIssueTypes(projectId: string): Observable<ProjectIssueTypes> {
    return this.http
      .get<Envelope<{ items: CatalogItemDto[]; configured: boolean }>>(
        `${this.apiUrl}/projects/${projectId}/issue-types`,
        { context: silentRequest() },
      )
      .pipe(
        map((body) => ({
          items: body.data.items.map(toCatalogItem),
          configured: body.data.configured,
        })),
      );
  }

  replaceProjectIssueTypes(
    projectId: string,
    issueTypeIds: readonly string[],
  ): Observable<ProjectIssueTypes> {
    return this.http
      .put<Envelope<{ items: CatalogItemDto[]; configured: boolean }>>(
        `${this.apiUrl}/projects/${projectId}/issue-types`,
        { issue_type_ids: issueTypeIds },
        { context: silentRequest() },
      )
      .pipe(
        map((body) => ({
          items: body.data.items.map(toCatalogItem),
          configured: body.data.configured,
        })),
      );
  }
}

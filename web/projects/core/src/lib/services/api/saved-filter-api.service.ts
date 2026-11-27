import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  IssuePriority,
  IssueStatus,
  IssueType,
  SavedFilter,
  SavedFilterCriteria,
} from '../../models/issue.model';

interface SavedFilterDto {
  id: string;
  name: string;
  filters: {
    status?: IssueStatus;
    priority?: IssuePriority;
    query?: string;
    issue_type?: IssueType;
    due_from?: string;
    due_to?: string;
  };
  created_at: string;
  updated_at: string;
}

interface Envelope<T> {
  data: T;
}

function toSavedFilter(dto: SavedFilterDto): SavedFilter {
  return {
    id: dto.id,
    name: dto.name,
    criteria: {
      status: dto.filters.status,
      priority: dto.filters.priority,
      query: dto.filters.query,
      issueType: dto.filters.issue_type,
      dueFrom: dto.filters.due_from,
      dueTo: dto.filters.due_to,
    },
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

function toFiltersDto(criteria: SavedFilterCriteria): SavedFilterDto['filters'] {
  return {
    status: criteria.status,
    priority: criteria.priority,
    query: criteria.query,
    issue_type: criteria.issueType,
    due_from: criteria.dueFrom,
    due_to: criteria.dueTo,
  };
}

@Injectable({ providedIn: 'root' })
export class SavedFilterApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = `${inject(API_URL)}/saved-filters`;

  list(): Observable<SavedFilter[]> {
    return this.http
      .get<Envelope<SavedFilterDto[]>>(this.baseUrl)
      .pipe(map((body) => body.data.map(toSavedFilter)));
  }

  create(name: string, criteria: SavedFilterCriteria): Observable<SavedFilter> {
    return this.http
      .post<Envelope<SavedFilterDto>>(this.baseUrl, { name, filters: toFiltersDto(criteria) })
      .pipe(map((body) => toSavedFilter(body.data)));
  }

  delete(id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${id}`);
  }
}

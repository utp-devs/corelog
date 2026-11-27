import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  CreateNoteInput,
  Note,
  NoteColor,
  NoteFilter,
  NotePage,
  NoteTagCount,
  UpdateNoteInput,
} from '../../models';

interface NoteDto {
  id: string;
  title: string;
  body: string;
  color: NoteColor;
  pinned: boolean;
  tags: string[] | null;
  issue_id: string | null;
  project_id: string | null;
  created_at: string;
  updated_at: string;
}

interface NoteListDto {
  data: NoteDto[];
  meta: { limit: number; next_cursor: string | null };
}

export function toNote(dto: NoteDto): Note {
  return {
    id: dto.id,
    title: dto.title ?? '',
    body: dto.body ?? '',
    color: dto.color,
    pinned: dto.pinned,
    tags: dto.tags ?? [],
    issueId: dto.issue_id ?? null,
    projectId: dto.project_id ?? null,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

/** Private notes of the signed-in user. The server never returns notes of other users. */
@Injectable({ providedIn: 'root' })
export class NoteApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = `${inject(API_URL)}/notes`;

  list(filter: NoteFilter = {}): Observable<NotePage> {
    let params = new HttpParams();
    const set = (name: string, value: string | number | boolean | null | undefined) => {
      if (value !== undefined && value !== null && value !== '') {
        params = params.set(name, String(value));
      }
    };
    set('q', filter.query?.trim());
    set('tag', filter.tag);
    set('pinned', filter.pinned);
    set('issue', filter.issueId);
    set('project', filter.projectId);
    set('cursor', filter.cursor);
    set('limit', filter.limit);
    return this.http.get<NoteListDto>(this.baseUrl, { params }).pipe(
      map((body) => ({
        items: body.data.map(toNote),
        nextCursor: body.meta.next_cursor ?? null,
      })),
    );
  }

  get(id: string): Observable<Note> {
    return this.http
      .get<{ data: NoteDto }>(`${this.baseUrl}/${id}`)
      .pipe(map((response) => toNote(response.data)));
  }

  tags(): Observable<NoteTagCount[]> {
    return this.http
      .get<{ data: NoteTagCount[] }>(`${this.baseUrl}/tags`)
      .pipe(map((body) => body.data));
  }

  create(input: CreateNoteInput): Observable<Note> {
    const body: Record<string, unknown> = {};
    if (input.title !== undefined) body['title'] = input.title;
    if (input.body !== undefined) body['body'] = input.body;
    if (input.color !== undefined) body['color'] = input.color;
    if (input.tags !== undefined) body['tags'] = input.tags;
    if (input.pinned !== undefined) body['pinned'] = input.pinned;
    if (input.issueId !== undefined) body['issue_id'] = input.issueId;
    if (input.projectId !== undefined) body['project_id'] = input.projectId;
    return this.http
      .post<{ data: NoteDto }>(this.baseUrl, body)
      .pipe(map((response) => toNote(response.data)));
  }

  update(id: string, input: UpdateNoteInput): Observable<Note> {
    const body: Record<string, unknown> = {};
    if (input.title !== undefined) body['title'] = input.title;
    if (input.body !== undefined) body['body'] = input.body;
    if (input.color !== undefined) body['color'] = input.color;
    if (input.tags !== undefined) body['tags'] = input.tags;
    if (input.issueId !== undefined) body['issue_id'] = input.issueId;
    if (input.projectId !== undefined) body['project_id'] = input.projectId;
    return this.http
      .patch<{ data: NoteDto }>(`${this.baseUrl}/${id}`, body)
      .pipe(map((response) => toNote(response.data)));
  }

  delete(id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${id}`);
  }

  pin(id: string): Observable<Note> {
    return this.http
      .post<{ data: NoteDto }>(`${this.baseUrl}/${id}/pin`, {})
      .pipe(map((response) => toNote(response.data)));
  }

  unpin(id: string): Observable<Note> {
    return this.http
      .post<{ data: NoteDto }>(`${this.baseUrl}/${id}/unpin`, {})
      .pipe(map((response) => toNote(response.data)));
  }
}

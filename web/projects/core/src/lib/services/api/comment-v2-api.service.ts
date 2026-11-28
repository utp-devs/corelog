import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  ActivityEntry,
  ActivityFilter,
  ActivityPage,
  CommentInternalRole,
  CommentReaction,
  CommentReactionKey,
  CommentRevision,
  CommentThread,
  CommentThreadPage,
  CommentV2,
  CommentVisibility,
  CommentWrite,
  CreateCommentInput,
} from '../../models/comment-v2.model';
import { silentRequest } from '../../utils/silent-request';

type Dto = Record<string, unknown>;

interface ActivityPageBody {
  data: Dto[];
  meta?: { next_cursor: string | null };
}

interface ActivityEnvelope {
  data: Dto[] | ActivityPageBody;
  meta?: { next_cursor: string | null };
}

interface Envelope<T> {
  data: T;
}

interface CursorEnvelope<T> {
  data: T;
  meta: { limit: number; next_cursor: string | null };
}

export function toCommentV2(dto: Dto): CommentV2 {
  const author = (dto['author'] ?? {}) as Dto;
  return {
    id: String(dto['id']),
    issueId: String(dto['issue_id']),
    parentId: (dto['parent_id'] as string | null) ?? null,
    authorId: String(dto['author_id']),
    authorName: String(author['name'] ?? ''),
    body: String(dto['body'] ?? ''),
    visibility: (dto['visibility'] as CommentVisibility) ?? 'public',
    internalRole: (dto['internal_role'] as CommentInternalRole | null) ?? null,
    mentions: (dto['mentions'] as string[] | undefined) ?? [],
    reactions: toReactions(dto['reactions']),
    edited: dto['edited'] === true,
    editedAt: (dto['edited_at'] as string | null) ?? null,
    deleted: dto['deleted'] === true,
    createdAt: String(dto['created_at']),
    updatedAt: String(dto['updated_at']),
  };
}

function toReactions(value: unknown): CommentReaction[] {
  return Array.isArray(value)
    ? value.map((item) => ({
        emoji: (item as Dto)['emoji'] as CommentReactionKey,
        count: Number((item as Dto)['count']),
        viewerReacted: (item as Dto)['viewer_reacted'] === true,
      }))
    : [];
}

function toThread(dto: Dto): CommentThread {
  const replies = (dto['replies'] as Dto[] | undefined) ?? [];
  return {
    ...toCommentV2(dto),
    replyCount: Number(dto['reply_count'] ?? replies.length),
    replies: replies.map(toCommentV2),
  };
}

function toWrite(dto: Dto): CommentWrite {
  const warnings = (dto['warnings'] as unknown[] | undefined) ?? [];
  return {
    comment: toCommentV2(dto),
    warnings: warnings.map((item) =>
      typeof item === 'string' ? item : String((item as Dto)['message'] ?? ''),
    ),
  };
}

/** Threaded comments, reactions, revisions and the merged activity feed of an issue. */
@Injectable({ providedIn: 'root' })
export class CommentV2ApiService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(API_URL);

  listThreads(
    issueId: string,
    options: { cursor?: string | null; limit?: number; sort?: 'asc' | 'desc' } = {},
  ): Observable<CommentThreadPage> {
    let params = new HttpParams();
    if (options.cursor) params = params.set('cursor', options.cursor);
    if (options.limit) params = params.set('limit', options.limit);
    if (options.sort) params = params.set('sort', options.sort);
    return this.http
      .get<CursorEnvelope<Dto[]>>(`${this.api}/issues/${issueId}/comments`, {
        params,
        context: silentRequest(),
      })
      .pipe(
        map((body) => ({
          threads: body.data.map(toThread),
          nextCursor: body.meta.next_cursor,
        })),
      );
  }

  create(issueId: string, input: CreateCommentInput): Observable<CommentWrite> {
    return this.http
      .post<Envelope<Dto>>(
        `${this.api}/issues/${issueId}/comments`,
        {
          body: input.body,
          parent_id: input.parentId ?? undefined,
          visibility: input.visibility,
          internal_role: input.visibility === 'internal' ? input.internalRole : undefined,
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toWrite(body.data)));
  }

  update(commentId: string, body: string): Observable<CommentWrite> {
    return this.http
      .patch<Envelope<Dto>>(
        `${this.api}/comments/${commentId}`,
        { body },
        { context: silentRequest() },
      )
      .pipe(map((response) => toWrite(response.data)));
  }

  remove(commentId: string): Observable<void> {
    return this.http.delete<void>(`${this.api}/comments/${commentId}`, {
      context: silentRequest(),
    });
  }

  revisions(commentId: string): Observable<CommentRevision[]> {
    return this.http
      .get<Envelope<Dto[]>>(`${this.api}/comments/${commentId}/revisions`, {
        context: silentRequest(),
      })
      .pipe(
        map((body) =>
          body.data.map((item) => ({
            id: String(item['id']),
            body: String(item['body']),
            editedBy: String(item['edited_by']),
            editedAt: String(item['edited_at']),
          })),
        ),
      );
  }

  /** Adds (or removes) the reaction of the caller and returns the reactions of the comment. */
  react(commentId: string, emoji: CommentReactionKey, on: boolean): Observable<CommentReaction[]> {
    const url = `${this.api}/comments/${commentId}/reactions/${emoji}`;
    const request = on
      ? this.http.put<Envelope<Dto>>(url, null, { context: silentRequest() })
      : this.http.delete<Envelope<Dto>>(url, { context: silentRequest() });
    return request.pipe(map((body) => toReactions(body.data['reactions'])));
  }

  activity(
    issueId: string,
    filter: ActivityFilter,
    cursor?: string | null,
    limit = 25,
  ): Observable<ActivityPage> {
    let params = new HttpParams().set('filter', filter).set('limit', limit);
    if (cursor) params = params.set('cursor', cursor);
    return this.http
      .get<ActivityEnvelope>(`${this.api}/issues/${issueId}/activity`, {
        params,
        context: silentRequest(),
      })
      .pipe(
        map((body) => {
          // The server wraps the page in a second data object that also carries the cursor.
          const page = (Array.isArray(body.data) ? body : body.data) as ActivityPageBody;
          return {
            entries: page.data.map(toEntry),
            nextCursor: page.meta?.next_cursor ?? null,
          };
        }),
      );
  }
}

function toEntry(dto: Dto): ActivityEntry {
  const actor = (dto['actor'] ?? {}) as Dto;
  const comment = dto['comment'] as Dto | undefined;
  const changes = (dto['changes'] as Dto[] | undefined) ?? [];
  return {
    id: String(dto['id']),
    type: dto['type'] === 'comment' ? 'comment' : 'history',
    createdAt: String(dto['created_at']),
    actor: { id: String(actor['id']), name: String(actor['name'] ?? '') },
    comment: comment
      ? {
          id: String(comment['id']),
          body: String(comment['body']),
          edited: comment['edited'] === true,
        }
      : null,
    changes: changes.map((change) => ({
      action: String(change['action']),
      field: String(change['field']),
      label: String(change['label'] ?? change['field']),
      from: (change['from'] as string | null) ?? null,
      to: (change['to'] as string | null) ?? null,
    })),
  };
}

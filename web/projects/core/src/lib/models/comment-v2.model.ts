/** The six reactions the API accepts, with the glyph and the Spanish name shown in the UI. */
export const COMMENT_REACTIONS = [
  { key: 'thumbs_up', glyph: '\u{1F44D}', label: 'Me gusta' },
  { key: 'heart', glyph: '❤️', label: 'Me encanta' },
  { key: 'tada', glyph: '\u{1F389}', label: 'Celebrar' },
  { key: 'eyes', glyph: '\u{1F440}', label: 'Lo estoy mirando' },
  { key: 'rocket', glyph: '\u{1F680}', label: 'Genial' },
  { key: 'check', glyph: '✅', label: 'Hecho' },
] as const;

export type CommentReactionKey = (typeof COMMENT_REACTIONS)[number]['key'];

export interface CommentReaction {
  readonly emoji: CommentReactionKey;
  readonly count: number;
  readonly viewerReacted: boolean;
}

export type CommentVisibility = 'public' | 'internal';
export type CommentInternalRole = 'contributor' | 'lead';

export interface CommentV2 {
  readonly id: string;
  readonly issueId: string;
  readonly parentId: string | null;
  readonly authorId: string;
  readonly authorName: string;
  readonly body: string;
  readonly visibility: CommentVisibility;
  readonly internalRole: CommentInternalRole | null;
  readonly mentions: readonly string[];
  readonly reactions: readonly CommentReaction[];
  readonly edited: boolean;
  readonly editedAt: string | null;
  readonly deleted: boolean;
  readonly createdAt: string;
  readonly updatedAt: string;
}

export interface CommentThread extends CommentV2 {
  readonly replyCount: number;
  readonly replies: readonly CommentV2[];
}

export interface CommentThreadPage {
  readonly threads: readonly CommentThread[];
  readonly nextCursor: string | null;
}

export interface CommentWrite {
  readonly comment: CommentV2;
  /** Mentions kept as plain text because the person cannot see the comment. */
  readonly warnings: readonly string[];
}

export interface CreateCommentInput {
  readonly body: string;
  readonly parentId?: string | null;
  readonly visibility?: CommentVisibility;
  readonly internalRole?: CommentInternalRole;
}

export interface CommentRevision {
  readonly id: string;
  readonly body: string;
  readonly editedBy: string;
  readonly editedAt: string;
}

export type ActivityFilter = 'all' | 'comments' | 'history';

export interface ActivityChange {
  /** field_changed, assignee_changed, status_changed, link_added or link_removed. */
  readonly action: string;
  readonly field: string;
  readonly label: string;
  readonly from: string | null;
  readonly to: string | null;
}

export interface ActivityEntry {
  /** Comment id, or the batch id shared by every change of one update. */
  readonly id: string;
  readonly type: 'comment' | 'history';
  readonly createdAt: string;
  readonly actor: { readonly id: string; readonly name: string };
  readonly comment: { readonly id: string; readonly body: string; readonly edited: boolean } | null;
  readonly changes: readonly ActivityChange[];
}

export interface ActivityPage {
  readonly entries: readonly ActivityEntry[];
  readonly nextCursor: string | null;
}

/** User shape used by the mention picker (the @ menu of the rich text editor). */
export interface MentionUser {
  readonly id: string;
  readonly name: string;
  readonly email: string;
}

/** Delay before a mention search is sent, in milliseconds. */
export const MENTION_SEARCH_DEBOUNCE_MS = 150;

/** Keeps only the fields the mention picker needs. */
export function toMentionUsers(
  users: readonly { readonly id: string; readonly name: string; readonly email: string }[],
): MentionUser[] {
  return users.map((user) => ({ id: user.id, name: user.name, email: user.email }));
}

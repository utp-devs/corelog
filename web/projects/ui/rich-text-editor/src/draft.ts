/** Draft autosave in localStorage under `corelog.draft.<draftKey>`. Storage errors are ignored. */

export function draftStorageKey(draftKey: string): string {
  return `corelog.draft.${draftKey}`;
}

export function readDraft(draftKey: string): string | null {
  try {
    return window.localStorage.getItem(draftStorageKey(draftKey));
  } catch {
    return null;
  }
}

export function writeDraft(draftKey: string, value: string): void {
  try {
    window.localStorage.setItem(draftStorageKey(draftKey), value);
  } catch {
    // Storage full or disabled: the draft is best effort.
  }
}

export function clearDraft(draftKey: string): void {
  try {
    window.localStorage.removeItem(draftStorageKey(draftKey));
  } catch {
    // Best effort.
  }
}

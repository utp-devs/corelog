/** Draft autosave in sessionStorage under `corelog.draft.<draftKey>`. Storage errors are ignored. */

export function draftStorageKey(draftKey: string): string {
  return `corelog.draft.${draftKey}`
}

export function readDraft(draftKey: string): string | null {
  try {
    return window.sessionStorage.getItem(draftStorageKey(draftKey))
  } catch {
    return null
  }
}

export function writeDraft(draftKey: string, value: string): void {
  try {
    window.sessionStorage.setItem(draftStorageKey(draftKey), value)
  } catch {
    // Storage full or disabled: the draft is best effort.
  }
}

export function clearDraft(draftKey: string): void {
  try {
    window.sessionStorage.removeItem(draftStorageKey(draftKey))
  } catch {
    // Best effort.
  }
}

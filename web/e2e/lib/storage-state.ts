import { readFileSync } from 'node:fs';

/** localStorage key used by the UI (ui/src/adapters/out/storage/token-storage.ts). */
export const TOKEN_STORAGE_KEY = 'corelog.auth.token';

/** localStorage key of the theme preference (the web app stores "light" or "dark" under it). */
export const THEME_STORAGE_KEY = 'corelog.theme';

/** localStorage key of the active organization id (ui/src/adapters/in/state/org.store.ts). */
export const CURRENT_ORG_STORAGE_KEY = 'corelog.currentOrg';

export interface StorageStateFile {
  cookies: unknown[];
  origins: Array<{ origin: string; localStorage: Array<{ name: string; value: string }> }>;
}

/**
 * Storage state that restores a logged-in UI session: the JWT lives in localStorage for the app origin.
 * When `currentOrgId` is given, the active organization is persisted too.
 */
export function buildStorageState(
  baseURL: string,
  token: string,
  currentOrgId?: string,
): StorageStateFile {
  const localStorage = [{ name: TOKEN_STORAGE_KEY, value: token }];
  if (currentOrgId) localStorage.push({ name: CURRENT_ORG_STORAGE_KEY, value: currentOrgId });
  return { cookies: [], origins: [{ origin: new URL(baseURL).origin, localStorage }] };
}

/** Reads the saved JWT of a role from its storage state file. Throws when the role was not set up. */
export function readStoredToken(storageFile: string): string {
  const parsed = JSON.parse(readFileSync(storageFile, 'utf8')) as StorageStateFile;
  for (const origin of parsed.origins) {
    const entry = origin.localStorage.find((item) => item.name === TOKEN_STORAGE_KEY);
    if (entry && entry.value !== '') return entry.value;
  }
  throw new Error(`No ${TOKEN_STORAGE_KEY} in ${storageFile}; re-run the e2e global setup`);
}

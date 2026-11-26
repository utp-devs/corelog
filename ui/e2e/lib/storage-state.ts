import { readFileSync } from 'node:fs'

/** localStorage key used by the UI (ui/src/adapters/out/storage/token-storage.ts). */
export const TOKEN_STORAGE_KEY = 'corelog.auth.token'

export interface StorageStateFile {
  cookies: unknown[]
  origins: Array<{ origin: string; localStorage: Array<{ name: string; value: string }> }>
}

/** Storage state that restores a logged-in UI session: the JWT lives in localStorage for the app origin. */
export function buildStorageState(baseURL: string, token: string): StorageStateFile {
  return {
    cookies: [],
    origins: [{ origin: new URL(baseURL).origin, localStorage: [{ name: TOKEN_STORAGE_KEY, value: token }] }],
  }
}

/** Reads the saved JWT of a role from its storage state file. Throws when the role was not set up. */
export function readStoredToken(storageFile: string): string {
  const parsed = JSON.parse(readFileSync(storageFile, 'utf8')) as StorageStateFile
  for (const origin of parsed.origins) {
    const entry = origin.localStorage.find((item) => item.name === TOKEN_STORAGE_KEY)
    if (entry && entry.value !== '') return entry.value
  }
  throw new Error(`No ${TOKEN_STORAGE_KEY} in ${storageFile}; re-run the e2e global setup`)
}

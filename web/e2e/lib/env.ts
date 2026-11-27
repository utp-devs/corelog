import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const DEFAULT_BASE_URL = 'http://localhost:4300';
// Same prefix the web app uses (openapi.yaml servers).
const DEFAULT_API_URL = 'http://localhost:8080/api/v1';

export interface E2eEnv {
  /** Origin and path of the web app under test. */
  readonly baseURL: string;
  /** API prefix, without trailing slash (e.g. http://localhost:8080/api/v1). */
  readonly apiURL: string;
  /** Liveness endpoint; mounted at the server root, outside the API prefix. */
  readonly healthURL: string;
  /** Shared password of the seeded accounts. Never logged. */
  readonly password: string | undefined;
  /** Directory holding one storage state per role (git-ignored). */
  readonly authDir: string;
}

/** Parses KEY=VALUE lines. Comments (#) and blank lines are ignored; matching quotes are stripped. */
export function parseEnvFile(text: string): Record<string, string> {
  const values: Record<string, string> = {};
  for (const rawLine of text.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (line === '' || line.startsWith('#')) continue;
    const separator = line.indexOf('=');
    if (separator <= 0) continue;
    const key = line.slice(0, separator).trim();
    let value = line.slice(separator + 1).trim();
    const quoted =
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"));
    if (quoted && value.length >= 2) value = value.slice(1, -1);
    values[key] = value;
  }
  return values;
}

/**
 * Git-ignored env files, first match wins for each key: E2E_ENV_FILE when set, then web/.env.e2e.
 */
function readLocalEnvFile(): Record<string, string> {
  const candidates = [
    process.env.E2E_ENV_FILE,
    fileURLToPath(new URL('../../.env.e2e', import.meta.url)),
  ].filter((file): file is string => Boolean(file));
  const merged: Record<string, string> = {};
  for (const file of candidates) {
    if (!existsSync(file)) continue;
    for (const [key, value] of Object.entries(parseEnvFile(readFileSync(file, 'utf8')))) {
      if (!(key in merged)) merged[key] = value;
    }
  }
  return merged;
}

/** Resolves the e2e settings: process environment first, then web/.env.e2e, then defaults. */
export function readE2eEnv(): E2eEnv {
  const fromFile = readLocalEnvFile();
  const pick = (key: string): string | undefined => process.env[key] || fromFile[key] || undefined;

  const baseURL = (pick('E2E_BASE_URL') ?? DEFAULT_BASE_URL).replace(/\/+$/, '');
  const apiURL = (pick('E2E_API_URL') ?? DEFAULT_API_URL).replace(/\/+$/, '');

  return {
    baseURL,
    apiURL,
    healthURL: `${new URL(apiURL).origin}/health`,
    password: pick('E2E_USER_PASSWORD'),
    authDir: fileURLToPath(new URL('../../playwright/.auth/', import.meta.url)),
  };
}

/** Path of the saved storage state for a role key (see fixtures/roles.ts). */
export function storageStatePath(roleKey: string): string {
  return path.join(readE2eEnv().authDir, `${roleKey}.json`);
}

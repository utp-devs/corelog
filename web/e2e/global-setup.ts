import { mkdirSync, writeFileSync } from 'node:fs';

import { expect, request, test as setup } from '@playwright/test';

import { readE2eEnv, storageStatePath } from './lib/env.ts';
import { buildStorageState } from './lib/storage-state.ts';
import { ROLE_KEYS, roles, type OrgSlug } from './fixtures/roles.ts';

// Runs as the `setup` project: every viewport project depends on it, the layout-audit self tests do not.
// It waits for the API, logs every role in through the API and stores one storage state per role,
// including the persisted current organization (acme roles -> Acme, legacy roles -> CoreLog, newbie -> none).
setup.setTimeout(120_000);

interface OrgSummary {
  readonly id: string;
  readonly slug: string;
}

setup('API is healthy and every role has a saved session', async () => {
  const env = readE2eEnv();
  if (!env.password) {
    throw new Error('E2E_USER_PASSWORD is not set: define it in the environment or in ui/.env.e2e');
  }

  const api = await request.newContext({ baseURL: `${env.apiURL}/` });
  try {
    await expect
      .poll(async () => (await api.get(env.healthURL)).status(), {
        message: `API health at ${env.healthURL} did not return 200 within 60 s`,
        timeout: 60_000,
        intervals: [1_000],
      })
      .toBe(200);

    mkdirSync(env.authDir, { recursive: true });
    for (const key of ROLE_KEYS) {
      const login = await api.post('auth/login', {
        data: { email: roles[key].email, password: env.password },
      });
      expect(login.status(), `login for role "${key}" must return 200`).toBe(200);
      const { data } = (await login.json()) as { data: { token: string } };

      const orgs = await api.get('organizations', {
        headers: { Authorization: `Bearer ${data.token}` },
      });
      expect(orgs.status(), `GET organizations for role "${key}" must return 200`).toBe(200);
      const listed = ((await orgs.json()) as { data: OrgSummary[] }).data;

      const wanted: OrgSlug | null = roles[key].org;
      let currentOrgId: string | undefined;
      if (wanted) {
        const match = listed.find((org) => org.slug === wanted);
        expect(
          match,
          `role "${key}" must belong to organization "${wanted}" (run the e2e seed)`,
        ).toBeTruthy();
        currentOrgId = match?.id;
      }

      writeFileSync(
        storageStatePath(key),
        JSON.stringify(buildStorageState(env.baseURL, data.token, currentOrgId), null, 2),
      );
    }
  } finally {
    await api.dispose();
  }
});

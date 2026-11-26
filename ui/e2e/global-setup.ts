import { mkdirSync, writeFileSync } from 'node:fs'

import { expect, request, test as setup } from '@playwright/test'

import { readE2eEnv, storageStatePath } from './lib/env.ts'
import { buildStorageState } from './lib/storage-state.ts'
import { ROLE_KEYS, roles } from './fixtures/roles.ts'

// Runs as the `setup` project: every viewport project depends on it, the layout-audit self tests do not.
// It waits for the API, logs every role in through the API and stores one storage state per role.
setup.setTimeout(120_000)

setup('API is healthy and every role has a saved session', async () => {
  const env = readE2eEnv()
  if (!env.password) {
    throw new Error('E2E_USER_PASSWORD is not set: define it in the environment or in ui/.env.e2e')
  }

  const api = await request.newContext({ baseURL: `${env.apiURL}/` })
  try {
    await expect
      .poll(async () => (await api.get(env.healthURL)).status(), {
        message: `API health at ${env.healthURL} did not return 200 within 60 s`,
        timeout: 60_000,
        intervals: [1_000],
      })
      .toBe(200)

    mkdirSync(env.authDir, { recursive: true })
    for (const key of ROLE_KEYS) {
      const response = await api.post('auth/login', {
        data: { email: roles[key].email, password: env.password },
      })
      expect(response.status(), `login for role "${key}" must return 200`).toBe(200)
      const body = (await response.json()) as { data: { token: string } }
      writeFileSync(storageStatePath(key), JSON.stringify(buildStorageState(env.baseURL, body.data.token), null, 2))
    }
  } finally {
    await api.dispose()
  }
})

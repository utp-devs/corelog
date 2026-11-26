import { orgIdBySlug } from './lib/api.ts'
import { test } from './fixtures/test.ts'

// TEMPORARY probe2 (delete before finishing)
test('probe2', async ({ api }) => {
  const client = await api('member')
  const orgId = await orgIdBySlug(client, 'acme')
  const res = await client.get(`organizations/${orgId}/members`)
  const body = (await res.json()) as { data?: Array<{ user_id?: string; userId?: string; name?: string; display_name?: string; email?: string }> }
  console.log('PROBE2 ' + JSON.stringify((body.data ?? []).map((m) => ({ name: m.name ?? m.display_name, email: m.email }))))
})

/** Platform roles as stored in users.role (migration 000024): admin or user. */
export type AppRole = 'admin' | 'user'

export interface RoleAccount {
  readonly email: string
  readonly appRole: AppRole
}

/**
 * Role key -> seeded account (testing strategy 3.1, `cmd/seed --scenario e2e`).
 * The key names the storage state file (ui/playwright/.auth/<key>.json) and is what
 * specs pass to `as()` and `api()`. Organization roles: owner, orgadmin, member, guest
 * belong to "Acme" (slug acme); admin, agent, requester and requesterAna are legacy
 * accounts kept in "corelog"; newbie belongs to no organization (onboarding).
 * Extend this map together with the seed when new accounts are added.
 */
export const roles = {
  admin: { email: 'admin@corelog.local', appRole: 'admin' },
  agent: { email: 'agent@corelog.local', appRole: 'user' },
  requester: { email: 'requester@corelog.local', appRole: 'user' },
  requesterAna: { email: 'ana.torres@corelog.test', appRole: 'user' },
  owner: { email: 'owner@corelog.test', appRole: 'user' },
  orgadmin: { email: 'orgadmin@corelog.test', appRole: 'user' },
  lead: { email: 'lead@corelog.test', appRole: 'user' },
  member: { email: 'member@corelog.test', appRole: 'user' },
  viewer: { email: 'viewer@corelog.test', appRole: 'user' },
  guest: { email: 'guest@corelog.test', appRole: 'user' },
  newbie: { email: 'newbie@corelog.test', appRole: 'user' },
} as const satisfies Record<string, RoleAccount>

export type RoleKey = keyof typeof roles

export const ROLE_KEYS = Object.keys(roles) as RoleKey[]

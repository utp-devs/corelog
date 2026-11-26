/** Platform roles as stored in users.role (migration 000024): admin or user. */
export type AppRole = 'admin' | 'user'

/** Slug of the organization a role works in by default (null: no organization, onboarding). */
export type OrgSlug = 'acme' | 'corelog'

export interface RoleAccount {
  readonly email: string
  readonly appRole: AppRole
  /** Organization made current by global setup (localStorage corelog.currentOrg). */
  readonly org: OrgSlug | null
}

/**
 * Role key -> seeded account (testing strategy 3.1, `cmd/seed --scenario e2e`).
 * The key names the storage state file (ui/playwright/.auth/<key>.json) and is what
 * specs pass to `as()` and `api()`. Acme roles (owner, orgadmin, lead, member, viewer, guest)
 * belong to "Acme" (slug acme); legacy accounts (admin, agent, requester, requesterAna) stay in
 * "corelog"; newbie belongs to no organization (onboarding).
 */
export const roles = {
  admin: { email: 'admin@corelog.local', appRole: 'admin', org: 'corelog' },
  agent: { email: 'agent@corelog.local', appRole: 'user', org: 'corelog' },
  requester: { email: 'requester@corelog.local', appRole: 'user', org: 'corelog' },
  requesterAna: { email: 'ana.torres@corelog.test', appRole: 'user', org: 'corelog' },
  owner: { email: 'owner@corelog.test', appRole: 'user', org: 'acme' },
  orgadmin: { email: 'orgadmin@corelog.test', appRole: 'user', org: 'acme' },
  lead: { email: 'lead@corelog.test', appRole: 'user', org: 'acme' },
  member: { email: 'member@corelog.test', appRole: 'user', org: 'acme' },
  viewer: { email: 'viewer@corelog.test', appRole: 'user', org: 'acme' },
  guest: { email: 'guest@corelog.test', appRole: 'user', org: 'acme' },
  newbie: { email: 'newbie@corelog.test', appRole: 'user', org: null },
} as const satisfies Record<string, RoleAccount>

export type RoleKey = keyof typeof roles

export const ROLE_KEYS = Object.keys(roles) as RoleKey[]

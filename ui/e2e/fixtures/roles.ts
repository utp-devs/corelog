/** Application roles as the API returns them (openapi.yaml Role enum). */
export type AppRole = 'admin' | 'agent' | 'requester'

export interface RoleAccount {
  readonly email: string
  readonly appRole: AppRole
}

/**
 * Role key -> seeded account. The key names the storage state file (ui/playwright/.auth/<key>.json)
 * and is what specs pass to `as()` and `api()`. Extend this map when new accounts are seeded.
 */
export const roles = {
  admin: { email: 'admin@corelog.local', appRole: 'admin' },
  agent: { email: 'agent@corelog.local', appRole: 'agent' },
  requester: { email: 'requester@corelog.local', appRole: 'requester' },
  requesterAna: { email: 'ana.torres@corelog.test', appRole: 'requester' },
} as const satisfies Record<string, RoleAccount>

export type RoleKey = keyof typeof roles

export const ROLE_KEYS = Object.keys(roles) as RoleKey[]

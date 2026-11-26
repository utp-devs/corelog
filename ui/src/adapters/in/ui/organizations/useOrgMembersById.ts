import { useEffect, useState } from 'react'

import { OrganizationHttpRepository } from '@/adapters/out/http/organization-http-repository'

export type OrgMemberEntry = { name: string }
export type OrgMembersById = ReadonlyMap<string, OrgMemberEntry>
export type OrgMembersByIdStatus = 'idle' | 'loading' | 'ready' | 'error'

/** Loads the members of one organization. Injected in tests; the default calls the API. */
export type OrgMembersLoader = (organizationId: string) => Promise<OrgMembersById>

const EMPTY: OrgMembersById = new Map()
const repository = new OrganizationHttpRepository()

// One request per organization for the whole session. Failed loads are evicted so a retry can run.
const cache = new Map<string, Promise<OrgMembersById>>()

const defaultLoader: OrgMembersLoader = async (organizationId) => {
  const members = await repository.listMembers(organizationId)
  return new Map(members.map((member) => [member.userId, { name: member.name }]))
}

/** Returns the cached member map for an organization, requesting it once. */
export function loadOrgMembersById(organizationId: string, load: OrgMembersLoader = defaultLoader): Promise<OrgMembersById> {
  const cached = cache.get(organizationId)
  if (cached) return cached
  const pending = load(organizationId)
  cache.set(organizationId, pending)
  pending.catch(() => {
    if (cache.get(organizationId) === pending) cache.delete(organizationId)
  })
  return pending
}

/** Clears the member cache. For tests only. */
export function resetOrgMembersByIdCache(): void {
  cache.clear()
}

/**
 * Map from user id to `{ name }` for the members of an organization, loaded once per organization.
 * Pass null or undefined while the organization is unknown.
 */
export function useOrgMembersById(
  organizationId: string | null | undefined,
  load: OrgMembersLoader = defaultLoader,
): { members: OrgMembersById; status: OrgMembersByIdStatus } {
  const [settled, setSettled] = useState<{ organizationId: string; status: 'ready' | 'error'; members: OrgMembersById } | null>(null)

  useEffect(() => {
    if (!organizationId) return
    let active = true
    loadOrgMembersById(organizationId, load).then(
      (members) => { if (active) setSettled({ organizationId, status: 'ready', members }) },
      () => { if (active) setSettled({ organizationId, status: 'error', members: EMPTY }) },
    )
    return () => { active = false }
    // The loader is expected to be stable (module function or test double created once per test).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [organizationId])

  if (!organizationId) return { members: EMPTY, status: 'idle' }
  if (settled?.organizationId !== organizationId) return { members: EMPTY, status: 'loading' }
  return { members: settled.members, status: settled.status }
}

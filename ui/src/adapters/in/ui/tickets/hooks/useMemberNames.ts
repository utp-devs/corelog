import { useCallback, useEffect, useState } from 'react'

import type { PeopleRepositoryPort } from '@/core/people/application/ports/people-repository.port'
import { PeopleHttpRepository } from '@/adapters/out/http/people-http-repository'
import { useOrgStore } from '@/adapters/in/state/org.store'

/** Name shown for an id that is not (or no longer) a member of the active organization. */
export const UNKNOWN_MEMBER_NAME = 'Usuario desconocido'
/** Placeholder shown while the member list of the organization is still loading. */
export const LOADING_MEMBER_NAME = 'Cargando…'

export type MemberSource = Pick<PeopleRepositoryPort, 'listMembers'>
export type MemberNames = ReadonlyMap<string, string>

const defaultSource: MemberSource = new PeopleHttpRepository()

// One request per organization for the whole session; a failed request is dropped so the next call retries.
const cache = new Map<string, Promise<MemberNames>>()

/** Loads (once per organization) the map from user id to member name. */
export function loadMemberNames(organizationId: string, source: MemberSource = defaultSource): Promise<MemberNames> {
  const cached = cache.get(organizationId)
  if (cached) return cached
  const request = source
    .listMembers(organizationId)
    .then((members) => new Map(members.map((member) => [member.userId, member.name] as const)))
  cache.set(organizationId, request)
  request.catch(() => {
    if (cache.get(organizationId) === request) cache.delete(organizationId)
  })
  return request
}

/** Clears the cache of member names. Used by tests and when the session changes. */
export function resetMemberNamesCache(): void {
  cache.clear()
}

type LoadResult = { organizationId: string; names?: MemberNames; failed?: boolean }

/**
 * Resolves user ids to names using the members of the active organization.
 * `nameOf` returns a loading placeholder while the list loads and UNKNOWN_MEMBER_NAME for ids that are not members.
 */
export function useMemberNames(source: MemberSource = defaultSource) {
  const organizationId = useOrgStore((state) => state.currentOrgId)
  const [result, setResult] = useState<LoadResult | null>(null)

  useEffect(() => {
    if (!organizationId) return
    let active = true
    loadMemberNames(organizationId, source).then(
      (names) => {
        if (active) setResult({ organizationId, names })
      },
      () => {
        if (active) setResult({ organizationId, failed: true })
      },
    )
    return () => {
      active = false
    }
  }, [organizationId, source])

  const current = result && result.organizationId === organizationId ? result : null
  const names = current?.names
  const failed = current?.failed === true

  const nameOf = useCallback(
    (userId: string): string => {
      if (names) return names.get(userId) ?? UNKNOWN_MEMBER_NAME
      if (failed || !organizationId) return UNKNOWN_MEMBER_NAME
      return LOADING_MEMBER_NAME
    },
    [names, failed, organizationId],
  )

  return { nameOf, isLoading: !names && !failed && organizationId !== null }
}

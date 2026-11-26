import { beforeEach, describe, expect, it } from 'vitest'

import { useOrgStore } from '@/adapters/in/state/org.store'
import { AppError } from '@/core/shared/app-error'
import { NO_CURRENT_ORG_CODE, withOrg } from './with-org'

describe('withOrg', () => {
  beforeEach(() => {
    useOrgStore.setState({ currentOrgId: null })
  })

  it('adds the active organization id and keeps the given params', () => {
    useOrgStore.setState({ currentOrgId: 'org-1' })
    expect(withOrg({ status: 'open' })).toEqual({ status: 'open', organization_id: 'org-1' })
  })

  it('works without params', () => {
    useOrgStore.setState({ currentOrgId: 'org-1' })
    expect(withOrg()).toEqual({ organization_id: 'org-1' })
  })

  it('throws a typed org.no_current error when no organization is active', () => {
    let caught: unknown
    try {
      withOrg()
    } catch (error) {
      caught = error
    }
    expect(caught).toBeInstanceOf(AppError)
    expect((caught as AppError).code).toBe(NO_CURRENT_ORG_CODE)
    expect(NO_CURRENT_ORG_CODE).toBe('org.no_current')
  })
})

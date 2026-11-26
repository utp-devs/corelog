import { create, type StoreApi, type UseBoundStore } from 'zustand'

import { OrganizationService } from '@/core/organizations/application/organization.service'
import type {
  CreateOrganizationInput,
  InvitationItemInput,
  OrganizationRepositoryPort,
} from '@/core/organizations/application/ports/organization-repository.port'
import type {
  InvitationCreated,
  InvitationPreview,
  Organization,
  OrganizationMembership,
  OrgRole,
} from '@/core/organizations/domain/organization'
import { OrganizationHttpRepository } from '@/adapters/out/http/organization-http-repository'
import { AppError, isAppError } from '@/core/shared/app-error'

/** Persisted id of the active organization. */
export const CURRENT_ORG_STORAGE_KEY = 'corelog.currentOrg'

function readPersistedOrgId(): string | null {
  try {
    return window.localStorage.getItem(CURRENT_ORG_STORAGE_KEY)
  } catch {
    return null
  }
}

function writePersistedOrgId(id: string | null): void {
  try {
    if (id) window.localStorage.setItem(CURRENT_ORG_STORAGE_KEY, id)
    else window.localStorage.removeItem(CURRENT_ORG_STORAGE_KEY)
  } catch {
    // Storage unavailable: the choice lasts for the session only.
  }
}

function toAppError(error: unknown, fallback: string): AppError {
  if (isAppError(error)) return error
  return new AppError({ status: 0, message: fallback, cause: error })
}

export interface OrgState {
  /** Organizations of the caller, each with the caller's role. */
  orgs: Organization[]
  /** Active organization id. Sent as organization_id by repositories through withOrg(). */
  currentOrgId: string | null
  /** True while a list request is in flight. */
  loading: boolean
  /** True once the list has been fetched successfully at least once. */
  loaded: boolean
  error: AppError | null
  fetchOrgs: () => Promise<void>
  setCurrentOrg: (id: string | null) => void
  createOrg: (input: CreateOrganizationInput) => Promise<Organization>
  leaveOrg: (id: string) => Promise<void>
  createInvitations: (organizationId: string, items: InvitationItemInput[]) => Promise<InvitationCreated[]>
  previewInvitation: (token: string) => Promise<InvitationPreview>
  acceptInvitation: (token: string) => Promise<OrganizationMembership>
  reset: () => void
}

type OrgStore = UseBoundStore<StoreApi<OrgState>>

const initialState = {
  orgs: [] as Organization[],
  loading: false,
  loaded: false,
  error: null as AppError | null,
}

/** Store factory: tests pass a fake repository; the app uses `useOrgStore` below. */
export function createOrgStore(repository: OrganizationRepositoryPort): OrgStore {
  const service = new OrganizationService(repository)

  return create<OrgState>((set, get) => {
    const select = (id: string | null) => {
      writePersistedOrgId(id)
      set({ currentOrgId: id })
    }

    return {
      ...initialState,
      currentOrgId: readPersistedOrgId(),

      fetchOrgs: async () => {
        set({ loading: true, error: null })
        try {
          const orgs = await service.list()
          const current = get().currentOrgId
          const keep = orgs.some((org) => org.id === current) ? current : (orgs[0]?.id ?? null)
          set({ orgs, loaded: true, loading: false })
          select(keep)
        } catch (error) {
          set({ loading: false, error: toAppError(error, 'No se pudieron cargar tus organizaciones.') })
        }
      },

      setCurrentOrg: (id) => {
        select(id)
      },

      createOrg: async (input) => {
        const org = await service.create(input)
        set({ orgs: [...get().orgs.filter((item) => item.id !== org.id), org], loaded: true })
        select(org.id)
        return org
      },

      leaveOrg: async (id) => {
        await service.leave(id)
        const orgs = get().orgs.filter((org) => org.id !== id)
        set({ orgs })
        if (get().currentOrgId === id) select(orgs[0]?.id ?? null)
      },

      createInvitations: (organizationId, items) => service.createInvitations(organizationId, items),

      previewInvitation: (token) => service.previewInvitation(token),

      acceptInvitation: (token) => service.acceptInvitation(token),

      reset: () => {
        writePersistedOrgId(null)
        set({ ...initialState, currentOrgId: null })
      },
    }
  })
}

export const useOrgStore = createOrgStore(new OrganizationHttpRepository())

/** Non-hook read of the active organization id, for repositories and helpers. */
export function getCurrentOrgId(): string | null {
  return useOrgStore.getState().currentOrgId
}

export interface CurrentOrg {
  org: Organization | null
  role: OrgRole | null
}

/** Active organization and the caller's role in it. */
export function useCurrentOrg(): CurrentOrg {
  const org = useOrgStore((s) => s.orgs.find((item) => item.id === s.currentOrgId) ?? null)
  return { org, role: org?.role ?? null }
}

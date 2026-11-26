import { create, type StoreApi, type UseBoundStore } from 'zustand'

import type {
  InvitationScope,
  OrganizationAdminRepositoryPort,
  OrganizationUpdate,
} from '@/core/organizations/application/ports/organization-admin-repository.port'
import type { InvitationItemInput } from '@/core/organizations/application/ports/organization-repository.port'
import type {
  Invitation,
  InvitationCreated,
  Organization,
  OrganizationMember,
  OrgRole,
} from '@/core/organizations/domain/organization'
import { OrganizationHttpRepository } from '@/adapters/out/http/organization-http-repository'
import { AppError, isAppError } from '@/core/shared/app-error'
import { useOrgStore } from '@/adapters/in/state/org.store'

export type LoadStatus = 'idle' | 'loading' | 'ready' | 'error'

function toAppError(error: unknown, fallback: string): AppError {
  if (isAppError(error)) return error
  return new AppError({ status: 0, message: fallback, cause: error })
}

export interface OrgAdminState {
  members: OrganizationMember[]
  membersStatus: LoadStatus
  membersError: AppError | null

  invitations: Invitation[]
  invitationScope: InvitationScope
  invitationsStatus: LoadStatus
  invitationsError: AppError | null
  /** Invitations created or resent in this session. Plain tokens exist only here, shown once. */
  createdInvitations: InvitationCreated[]

  /** Object URL of the organization logo, or null when there is none. Revoked when replaced. */
  logoUrl: string | null

  /** Last failure of a write action (the action also rejects with the same AppError). */
  error: AppError | null

  fetchMembers: (organizationId: string) => Promise<void>
  changeMemberRole: (organizationId: string, userId: string, role: OrgRole) => Promise<void>
  removeMember: (organizationId: string, userId: string) => Promise<void>
  transferOwnership: (organizationId: string, userId: string) => Promise<void>

  fetchInvitations: (organizationId: string, scope?: InvitationScope) => Promise<void>
  createInvitations: (organizationId: string, items: InvitationItemInput[]) => Promise<InvitationCreated[]>
  resendInvitation: (organizationId: string, invitationId: string) => Promise<InvitationCreated>
  revokeInvitation: (organizationId: string, invitationId: string) => Promise<void>
  clearCreatedInvitations: () => void

  updateOrganization: (organizationId: string, update: OrganizationUpdate) => Promise<Organization>
  /** hasLogo false skips the request and clears the preview; undefined asks the API. */
  loadLogo: (organizationId: string, hasLogo?: boolean) => Promise<void>
  uploadLogo: (organizationId: string, file: File) => Promise<void>
  removeLogo: (organizationId: string) => Promise<void>
  deleteOrganization: (organizationId: string, confirmSlug: string) => Promise<void>
  leaveOrganization: (organizationId: string) => Promise<void>

  reset: () => void
}

export type OrgAdminStore = UseBoundStore<StoreApi<OrgAdminState>>

const initialState = {
  members: [] as OrganizationMember[],
  membersStatus: 'idle' as LoadStatus,
  membersError: null as AppError | null,
  invitations: [] as Invitation[],
  invitationScope: 'pending' as InvitationScope,
  invitationsStatus: 'idle' as LoadStatus,
  invitationsError: null as AppError | null,
  createdInvitations: [] as InvitationCreated[],
  logoUrl: null as string | null,
  error: null as AppError | null,
}

export interface OrgAdminStoreOptions {
  /** Called after a logo upload or removal so the organization list can reflect hasLogo. */
  onLogoChanged?: (organizationId: string, hasLogo: boolean) => void
}

/** Store factory: tests pass a fake repository; pages use `useOrgAdminStore` below. */
export function createOrgAdminStore(
  repository: OrganizationAdminRepositoryPort,
  options: OrgAdminStoreOptions = {},
): OrgAdminStore {
  return create<OrgAdminState>((set, get) => {
    let objectUrl: string | null = null

    const replaceObjectUrl = (next: string | null) => {
      if (objectUrl && typeof URL.revokeObjectURL === 'function') URL.revokeObjectURL(objectUrl)
      objectUrl = next
      set({ logoUrl: next })
    }

    /** Records the failure in the store and returns it so callers can `throw fail(...)`. */
    const fail = (error: unknown, fallback: string): AppError => {
      const appError = toAppError(error, fallback)
      set({ error: appError })
      return appError
    }

    const refreshInvitations = (organizationId: string) => get().fetchInvitations(organizationId, get().invitationScope)

    return {
      ...initialState,

      fetchMembers: async (organizationId) => {
        set({ membersStatus: 'loading', membersError: null })
        try {
          const members = await repository.listMembers(organizationId)
          set({ members, membersStatus: 'ready' })
        } catch (error) {
          set({ membersStatus: 'error', membersError: toAppError(error, 'No se pudo cargar el directorio.') })
        }
      },

      changeMemberRole: async (organizationId, userId, role) => {
        try {
          await repository.changeMemberRole(organizationId, userId, role)
        } catch (error) {
          throw fail(error, 'No se pudo cambiar el rol.')
        }
        set({ members: get().members.map((m) => (m.userId === userId ? { ...m, role } : m)) })
      },

      removeMember: async (organizationId, userId) => {
        try {
          await repository.removeMember(organizationId, userId)
        } catch (error) {
          throw fail(error, 'No se pudo quitar al miembro.')
        }
        set({ members: get().members.filter((m) => m.userId !== userId) })
      },

      transferOwnership: async (organizationId, userId) => {
        try {
          await repository.transferOwnership(organizationId, userId)
        } catch (error) {
          throw fail(error, 'No se pudo transferir la propiedad.')
        }
      },

      fetchInvitations: async (organizationId, scope) => {
        const nextScope = scope ?? get().invitationScope
        set({ invitationScope: nextScope, invitationsStatus: 'loading', invitationsError: null })
        try {
          const invitations = await repository.listInvitations(organizationId, nextScope)
          set({ invitations, invitationsStatus: 'ready' })
        } catch (error) {
          set({
            invitationsStatus: 'error',
            invitationsError: toAppError(error, 'No se pudieron cargar las invitaciones.'),
          })
        }
      },

      createInvitations: async (organizationId, items) => {
        let created: InvitationCreated[]
        try {
          created = await repository.createInvitations(organizationId, items)
        } catch (error) {
          throw fail(error, 'No se pudieron enviar las invitaciones.')
        }
        set({ createdInvitations: created })
        await refreshInvitations(organizationId)
        return created
      },

      resendInvitation: async (organizationId, invitationId) => {
        let created: InvitationCreated
        try {
          created = await repository.resendInvitation(organizationId, invitationId)
        } catch (error) {
          throw fail(error, 'No se pudo reenviar la invitación.')
        }
        set({ createdInvitations: [created] })
        await refreshInvitations(organizationId)
        return created
      },

      revokeInvitation: async (organizationId, invitationId) => {
        try {
          await repository.revokeInvitation(organizationId, invitationId)
        } catch (error) {
          throw fail(error, 'No se pudo revocar la invitación.')
        }
        await refreshInvitations(organizationId)
      },

      clearCreatedInvitations: () => set({ createdInvitations: [] }),

      updateOrganization: async (organizationId, update) => {
        try {
          return await repository.updateOrganization(organizationId, update)
        } catch (error) {
          throw fail(error, 'No se pudo guardar la configuración.')
        }
      },

      loadLogo: async (organizationId, hasLogo) => {
        if (hasLogo === false) {
          replaceObjectUrl(null)
          return
        }
        try {
          const blob = await repository.getLogo(organizationId)
          replaceObjectUrl(blob && typeof URL.createObjectURL === 'function' ? URL.createObjectURL(blob) : null)
        } catch {
          replaceObjectUrl(null)
        }
      },

      uploadLogo: async (organizationId, file) => {
        try {
          await repository.uploadLogo(organizationId, file)
        } catch (error) {
          throw fail(error, 'No se pudo subir el logo.')
        }
        options.onLogoChanged?.(organizationId, true)
        await get().loadLogo(organizationId)
      },

      removeLogo: async (organizationId) => {
        try {
          await repository.deleteLogo(organizationId)
        } catch (error) {
          throw fail(error, 'No se pudo quitar el logo.')
        }
        replaceObjectUrl(null)
        options.onLogoChanged?.(organizationId, false)
      },

      deleteOrganization: async (organizationId, confirmSlug) => {
        try {
          await repository.deleteOrganization(organizationId, confirmSlug)
        } catch (error) {
          throw fail(error, 'No se pudo eliminar la organización.')
        }
      },

      leaveOrganization: async (organizationId) => {
        try {
          await repository.leave(organizationId)
        } catch (error) {
          throw fail(error, 'No se pudo salir de la organización.')
        }
      },

      reset: () => {
        replaceObjectUrl(null)
        set({ ...initialState })
      },
    }
  })
}

/** Application singleton. Logo changes are mirrored into the organization list of org.store. */
export const useOrgAdminStore = createOrgAdminStore(new OrganizationHttpRepository(), {
  onLogoChanged: (organizationId, hasLogo) => useOrgStore.getState().setOrgLogo(organizationId, hasLogo),
})

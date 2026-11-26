import { create } from 'zustand'

import { AppError, isAppError } from '@/core/shared/app-error'
import { TeamService } from '@/core/teams/application/team.service'
import type { CreateTeamInput, UpdateTeamInput } from '@/core/teams/application/ports/team-repository.port'
import type { Team, TeamMember, TeamMemberRole } from '@/core/teams/domain/team'
import { UserDirectoryService } from '@/core/users/application/user-directory.service'
import { TeamHttpRepository } from '@/adapters/out/http/team-http-repository'
import { UserDirectoryHttpRepository } from '@/adapters/out/http/user-directory-http-repository'
import { getCurrentOrgId } from '@/adapters/in/state/org.store'

const teamService = new TeamService(new TeamHttpRepository())
const directoryService = new UserDirectoryService(new UserDirectoryHttpRepository())

type Status = 'idle' | 'loading' | 'error'

/** Team membership joined with the display data of the user (the membership endpoint only returns ids). */
export interface TeamMemberView extends TeamMember {
  name: string
  email: string
}

interface TeamsState {
  teams: Team[]
  currentTeam: Team | null
  status: Status
  error: AppError | null
  members: TeamMemberView[]
  membersStatus: Status
  fetchTeams: () => Promise<void>
  fetchTeam: (id: string) => Promise<void>
  createTeam: (input: Omit<CreateTeamInput, 'organizationId'>) => Promise<Team>
  updateTeam: (id: string, input: UpdateTeamInput) => Promise<Team>
  removeTeam: (id: string) => Promise<void>
  fetchMembers: (teamId: string) => Promise<void>
  addMember: (teamId: string, userId: string, role?: TeamMemberRole) => Promise<void>
  updateMemberRole: (teamId: string, userId: string, role: TeamMemberRole) => Promise<void>
  removeMember: (teamId: string, userId: string) => Promise<void>
  reset: () => void
}

/** Wraps unexpected (non-HTTP) failures so the store always holds an AppError. */
function toAppError(error: unknown, fallback: string): AppError {
  if (isAppError(error)) return error
  return new AppError({ status: 0, message: fallback, cause: error })
}

const initialState = {
  teams: [] as Team[],
  currentTeam: null as Team | null,
  status: 'idle' as Status,
  error: null as AppError | null,
  members: [] as TeamMemberView[],
  membersStatus: 'idle' as Status,
}

export const useTeamsStore = create<TeamsState>((set, get) => ({
  ...initialState,

  fetchTeams: async () => {
    set({ status: 'loading', error: null })
    try {
      const teams = await teamService.list()
      set({ teams, status: 'idle' })
    } catch (error) {
      set({ status: 'error', error: toAppError(error, 'No se pudieron cargar los equipos') })
    }
  },

  fetchTeam: async (id) => {
    try {
      set({ currentTeam: await teamService.get(id), error: null })
    } catch (error) {
      set({ error: toAppError(error, 'No se pudo cargar el equipo') })
    }
  },

  createTeam: async (input) => {
    const organizationId = getCurrentOrgId()
    if (!organizationId) {
      const error = new AppError({
        status: 422,
        code: 'team.organization_required',
        message: 'Selecciona una organización antes de crear un equipo.',
      })
      set({ error })
      throw error
    }
    try {
      const team = await teamService.create({ ...input, organizationId })
      set({ teams: [...get().teams, team].sort((a, b) => a.name.localeCompare(b.name)), error: null })
      return team
    } catch (error) {
      const appError = toAppError(error, 'No se pudo crear el equipo')
      set({ error: appError })
      throw appError
    }
  },

  updateTeam: async (id, input) => {
    try {
      const updated = await teamService.update(id, input)
      set({
        teams: get().teams.map((t) => (t.id === id ? updated : t)),
        currentTeam: get().currentTeam?.id === id ? updated : get().currentTeam,
        error: null,
      })
      return updated
    } catch (error) {
      const appError = toAppError(error, 'No se pudo actualizar el equipo')
      set({ error: appError })
      throw appError
    }
  },

  removeTeam: async (id) => {
    try {
      await teamService.remove(id)
      set({
        teams: get().teams.filter((t) => t.id !== id),
        currentTeam: get().currentTeam?.id === id ? null : get().currentTeam,
        error: null,
      })
    } catch (error) {
      const appError = toAppError(error, 'No se pudo eliminar el equipo')
      set({ error: appError })
      throw appError
    }
  },

  fetchMembers: async (teamId) => {
    set({ membersStatus: 'loading' })
    try {
      const [memberships, users] = await Promise.all([teamService.listMembers(teamId), directoryService.search({ query: '' })])
      const byId = new Map(users.map((u) => [u.id, u]))
      const members: TeamMemberView[] = memberships.map((m) => ({
        ...m,
        name: byId.get(m.userId)?.name ?? m.userId,
        email: byId.get(m.userId)?.email ?? '',
      }))
      set({ members, membersStatus: 'idle' })
    } catch (error) {
      set({ membersStatus: 'error', error: toAppError(error, 'No se pudieron cargar los miembros') })
    }
  },

  addMember: async (teamId, userId, role) => {
    try {
      await teamService.addMember(teamId, userId, role)
    } catch (error) {
      const appError = toAppError(error, 'No se pudo agregar el miembro')
      set({ error: appError })
      throw appError
    }
    await get().fetchMembers(teamId)
  },

  updateMemberRole: async (teamId, userId, role) => {
    try {
      const updated = await teamService.updateMemberRole(teamId, userId, role)
      set({
        members: get().members.map((m) => (m.userId === userId ? { ...m, role: updated.role } : m)),
        error: null,
      })
    } catch (error) {
      const appError = toAppError(error, 'No se pudo cambiar el rol del miembro')
      set({ error: appError })
      throw appError
    }
  },

  removeMember: async (teamId, userId) => {
    try {
      await teamService.removeMember(teamId, userId)
      set({ members: get().members.filter((m) => m.userId !== userId), error: null })
    } catch (error) {
      const appError = toAppError(error, 'No se pudo eliminar el miembro')
      set({ error: appError })
      throw appError
    }
  },

  reset: () => set({ ...initialState }),
}))

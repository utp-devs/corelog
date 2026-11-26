import { create } from 'zustand'

import { AppError, isAppError } from '@/core/shared/app-error'
import { ProjectService } from '@/core/projects/application/project.service'
import type { CreateProjectInput, UpdateProjectInput } from '@/core/projects/application/ports/project-repository.port'
import type { Project, ProjectMember, ProjectMemberRole, ProjectLabel, ProjectComponent } from '@/core/projects/domain/project'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import { getCurrentOrgId } from '@/adapters/in/state/org.store'

const projectRepository = new ProjectHttpRepository()
const projectService = new ProjectService(projectRepository)

interface ProjectsState {
  projects: Project[]
  members: ProjectMember[]
  labels: ProjectLabel[]
  components: ProjectComponent[]
  status: 'idle' | 'loading' | 'error'
  /** Last failure with its API code (for example project.user_not_in_org). Null when the last call succeeded. */
  error: AppError | null
  fetchProjects: () => Promise<void>
  createProject: (input: Omit<CreateProjectInput, 'organizationId'>) => Promise<Project>
  updateProject: (id: string, input: UpdateProjectInput) => Promise<Project>
  fetchMembers: (projectId: string) => Promise<void>
  addMember: (projectId: string, userId: string, role: ProjectMemberRole) => Promise<void>
  updateMember: (projectId: string, userId: string, role: ProjectMemberRole) => Promise<void>
  removeMember: (projectId: string, userId: string) => Promise<void>
  fetchLabels: (projectId: string) => Promise<void>
  createLabel: (projectId: string, name: string, color: string) => Promise<void>
  deleteLabel: (projectId: string, labelId: string) => Promise<void>
  fetchComponents: (projectId: string) => Promise<void>
  createComponent: (projectId: string, name: string, description: string) => Promise<void>
  deleteComponent: (projectId: string, componentId: string) => Promise<void>
  reset: () => void
}

/** Wraps unexpected (non-HTTP) failures so the store always holds an AppError. */
function toAppError(error: unknown, fallback: string): AppError {
  if (isAppError(error)) return error
  return new AppError({ status: 0, message: fallback, cause: error })
}

const initialState = {
  projects: [] as Project[],
  members: [] as ProjectMember[],
  labels: [] as ProjectLabel[],
  components: [] as ProjectComponent[],
  status: 'idle' as ProjectsState['status'],
  error: null as AppError | null,
}

export const useProjectsStore = create<ProjectsState>((set, get) => ({
  ...initialState,

  fetchProjects: async () => {
    set({ status: 'loading', error: null })
    try {
      set({ projects: await projectService.list(), status: 'idle' })
    } catch (error) {
      set({ status: 'error', error: toAppError(error, 'No se pudieron cargar los proyectos') })
    }
  },

  createProject: async (input) => {
    const organizationId = getCurrentOrgId()
    if (!organizationId) {
      const error = new AppError({
        status: 422,
        code: 'project.organization_required',
        message: 'Selecciona una organización antes de crear un proyecto.',
      })
      set({ error })
      throw error
    }
    try {
      const project = await projectService.create({ ...input, organizationId })
      set({ projects: [...get().projects, project].sort((a, b) => a.name.localeCompare(b.name)), error: null })
      return project
    } catch (error) {
      const appError = toAppError(error, 'No se pudo crear el proyecto')
      set({ error: appError })
      throw appError
    }
  },

  updateProject: async (id, input) => {
    try {
      const project = await projectService.update(id, input)
      set({ projects: get().projects.map((p) => (p.id === id ? project : p)), error: null })
      return project
    } catch (error) {
      const appError = toAppError(error, 'No se pudo actualizar el proyecto')
      set({ error: appError })
      throw appError
    }
  },

  fetchMembers: async (projectId) => {
    set({ status: 'loading', error: null })
    try {
      set({ members: await projectRepository.listMembers(projectId), status: 'idle' })
    } catch (error) {
      set({ status: 'error', error: toAppError(error, 'No se pudieron cargar los miembros') })
    }
  },

  addMember: async (projectId, userId, role) => {
    try {
      const member = await projectRepository.addMember(projectId, userId, role)
      set({ members: [...get().members.filter((item) => item.userId !== member.userId), member], error: null })
    } catch (error) {
      // Codes such as project.user_not_in_org and project.user_not_found reach the UI through error.code.
      const appError = toAppError(error, 'No se pudo agregar el miembro')
      set({ error: appError })
      throw appError
    }
  },

  updateMember: async (projectId, userId, role) => {
    const member = await projectRepository.updateMember(projectId, userId, role)
    set({ members: get().members.map((item) => (item.userId === userId ? member : item)) })
  },

  removeMember: async (projectId, userId) => {
    await projectRepository.removeMember(projectId, userId)
    set({ members: get().members.filter((item) => item.userId !== userId) })
  },

  fetchLabels: async (projectId) => {
    set({ labels: await projectRepository.listLabels(projectId) })
  },
  createLabel: async (projectId, name, color) => {
    const label = await projectRepository.createLabel(projectId, name, color)
    set({ labels: [...get().labels.filter((item) => item.id !== label.id), label] })
  },
  deleteLabel: async (projectId, labelId) => {
    await projectRepository.deleteLabel(projectId, labelId)
    set({ labels: get().labels.filter((item) => item.id !== labelId) })
  },
  fetchComponents: async (projectId) => {
    set({ components: await projectRepository.listComponents(projectId) })
  },
  createComponent: async (projectId, name, description) => {
    const component = await projectRepository.createComponent(projectId, name, description)
    set({ components: [...get().components.filter((item) => item.id !== component.id), component] })
  },
  deleteComponent: async (projectId, componentId) => {
    await projectRepository.deleteComponent(projectId, componentId)
    set({ components: get().components.filter((item) => item.id !== componentId) })
  },

  reset: () => set({ ...initialState }),
}))

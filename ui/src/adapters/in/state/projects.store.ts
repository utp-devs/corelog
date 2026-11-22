import { create } from 'zustand'

import { ProjectService } from '@/core/projects/application/project.service'
import type { CreateProjectInput } from '@/core/projects/application/ports/project-repository.port'
import type { Project, ProjectMember, ProjectMemberRole, ProjectLabel, ProjectComponent } from '@/core/projects/domain/project'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import { extractErrorMessage } from '@/adapters/out/http/api-client'

const projectService = new ProjectService(new ProjectHttpRepository())

interface ProjectsState {
  projects: Project[]
  members: ProjectMember[]
  labels: ProjectLabel[]
  components: ProjectComponent[]
  status: 'idle' | 'loading' | 'error'
  error: string | null
  fetchProjects: () => Promise<void>
  createProject: (input: CreateProjectInput) => Promise<Project>
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
}

export const useProjectsStore = create<ProjectsState>((set, get) => ({
  projects: [],
  members: [],
  labels: [],
  components: [],
  status: 'idle',
  error: null,

  fetchProjects: async () => {
    set({ status: 'loading', error: null })
    try {
      set({ projects: await projectService.list(), status: 'idle' })
    } catch (error) {
      set({ status: 'error', error: extractErrorMessage(error, 'No se pudieron cargar los proyectos') })
    }
  },

  createProject: async (input) => {
    const project = await projectService.create(input)
    set({ projects: [...get().projects, project].sort((a, b) => a.name.localeCompare(b.name)) })
    return project
  },

  fetchMembers: async (projectId) => {
    set({ status: 'loading', error: null })
    try {
      set({ members: await new ProjectHttpRepository().listMembers(projectId), status: 'idle' })
    } catch (error) {
      set({ status: 'error', error: extractErrorMessage(error, 'No se pudieron cargar los miembros') })
    }
  },

  addMember: async (projectId, userId, role) => {
    const member = await new ProjectHttpRepository().addMember(projectId, userId, role)
    set({ members: [...get().members.filter((item) => item.userId !== member.userId), member] })
  },

  updateMember: async (projectId, userId, role) => {
    const member = await new ProjectHttpRepository().updateMember(projectId, userId, role)
    set({ members: get().members.map((item) => item.userId === userId ? member : item) })
  },

  removeMember: async (projectId, userId) => {
    await new ProjectHttpRepository().removeMember(projectId, userId)
    set({ members: get().members.filter((item) => item.userId !== userId) })
  },

  fetchLabels: async (projectId) => {
    set({ labels: await new ProjectHttpRepository().listLabels(projectId) })
  },
  createLabel: async (projectId, name, color) => {
    const label = await new ProjectHttpRepository().createLabel(projectId, name, color)
    set({ labels: [...get().labels.filter((item) => item.id !== label.id), label] })
  },
  deleteLabel: async (projectId, labelId) => {
    await new ProjectHttpRepository().deleteLabel(projectId, labelId)
    set({ labels: get().labels.filter((item) => item.id !== labelId) })
  },
  fetchComponents: async (projectId) => {
    set({ components: await new ProjectHttpRepository().listComponents(projectId) })
  },
  createComponent: async (projectId, name, description) => {
    const component = await new ProjectHttpRepository().createComponent(projectId, name, description)
    set({ components: [...get().components.filter((item) => item.id !== component.id), component] })
  },
  deleteComponent: async (projectId, componentId) => {
    await new ProjectHttpRepository().deleteComponent(projectId, componentId)
    set({ components: get().components.filter((item) => item.id !== componentId) })
  },
}))

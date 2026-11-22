import type { CreateProjectInput, ProjectRepositoryPort } from '@/core/projects/application/ports/project-repository.port'
import type { Project, ProjectMember, ProjectMemberRole, ProjectLabel, ProjectComponent } from '@/core/projects/domain/project'
import { apiClient } from './api-client'

interface ProjectDto {
  id: string
  key: string
  name: string
  description: string
  type: Project['type']
  lead_id: string
  visibility: Project['visibility']
  archived_at: string | null
  created_at: string
  updated_at: string
}

interface ProjectMemberDto {
  project_id: string
  user_id: string
  role: ProjectMemberRole
  created_at: string
}

interface LabelDto {
  id: string
  project_id: string
  name: string
  color: string
  created_at: string
  updated_at: string
}

interface ComponentDto {
  id: string
  project_id: string
  name: string
  description: string
  created_at: string
  updated_at: string
}

function toProjectMember(dto: ProjectMemberDto): ProjectMember {
  return {
    projectId: dto.project_id,
    userId: dto.user_id,
    role: dto.role,
    createdAt: dto.created_at,
  }
}

function toLabel(dto: LabelDto): ProjectLabel {
  return {
    id: dto.id,
    projectId: dto.project_id,
    name: dto.name,
    color: dto.color,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  }
}

function toComponent(dto: ComponentDto): ProjectComponent {
  return {
    id: dto.id,
    projectId: dto.project_id,
    name: dto.name,
    description: dto.description,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  }
}

function toProject(dto: ProjectDto): Project {
  return {
    id: dto.id,
    key: dto.key,
    name: dto.name,
    description: dto.description,
    type: dto.type,
    leadId: dto.lead_id,
    visibility: dto.visibility,
    archivedAt: dto.archived_at,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  }
}

export class ProjectHttpRepository implements ProjectRepositoryPort {
  async list(): Promise<Project[]> {
    const { data } = await apiClient.get<{ data: ProjectDto[] }>('/projects')
    return data.data.map(toProject)
  }

  async get(id: string): Promise<Project> {
    const { data } = await apiClient.get<{ data: ProjectDto }>(`/projects/${id}`)
    return toProject(data.data)
  }

  async create(input: CreateProjectInput): Promise<Project> {
    const { data } = await apiClient.post<{ data: ProjectDto }>('/projects', {
      key: input.key,
      name: input.name,
      description: input.description,
      type: input.type,
      lead_id: input.leadId,
      visibility: input.visibility,
    })
    return toProject(data.data)
  }

  async update(id: string, input: Omit<CreateProjectInput, 'key'>): Promise<Project> {
    const { data } = await apiClient.patch<{ data: ProjectDto }>(`/projects/${id}`, {
      name: input.name,
      description: input.description,
      type: input.type,
      lead_id: input.leadId,
      visibility: input.visibility,
    })
    return toProject(data.data)
  }

  async remove(id: string): Promise<void> {
    await apiClient.delete(`/projects/${id}`)
  }

  async listMembers(projectId: string): Promise<ProjectMember[]> {
    const { data } = await apiClient.get<{ data: ProjectMemberDto[] }>(`/projects/${projectId}/members`)
    return data.data.map(toProjectMember)
  }

  async addMember(projectId: string, userId: string, role: ProjectMemberRole): Promise<ProjectMember> {
    const { data } = await apiClient.post<{ data: ProjectMemberDto }>(`/projects/${projectId}/members`, { user_id: userId, role })
    return toProjectMember(data.data)
  }

  async updateMember(projectId: string, userId: string, role: ProjectMemberRole): Promise<ProjectMember> {
    const { data } = await apiClient.patch<{ data: ProjectMemberDto }>(`/projects/${projectId}/members/${userId}`, { role })
    return toProjectMember(data.data)
  }

  async removeMember(projectId: string, userId: string): Promise<void> {
    await apiClient.delete(`/projects/${projectId}/members/${userId}`)
  }

  async listLabels(projectId: string): Promise<ProjectLabel[]> {
    const { data } = await apiClient.get<{ data: LabelDto[] }>(`/projects/${projectId}/labels`)
    return data.data.map(toLabel)
  }

  async createLabel(projectId: string, name: string, color: string): Promise<ProjectLabel> {
    const { data } = await apiClient.post<{ data: LabelDto }>(`/projects/${projectId}/labels`, { name, color })
    return toLabel(data.data)
  }

  async deleteLabel(projectId: string, labelId: string): Promise<void> {
    await apiClient.delete(`/projects/${projectId}/labels/${labelId}`)
  }

  async listComponents(projectId: string): Promise<ProjectComponent[]> {
    const { data } = await apiClient.get<{ data: ComponentDto[] }>(`/projects/${projectId}/components`)
    return data.data.map(toComponent)
  }

  async createComponent(projectId: string, name: string, description: string): Promise<ProjectComponent> {
    const { data } = await apiClient.post<{ data: ComponentDto }>(`/projects/${projectId}/components`, { name, description })
    return toComponent(data.data)
  }

  async deleteComponent(projectId: string, componentId: string): Promise<void> {
    await apiClient.delete(`/projects/${projectId}/components/${componentId}`)
  }
}

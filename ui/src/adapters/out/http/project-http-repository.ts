import type { CreateProjectInput, ProjectRepositoryPort } from '@/core/projects/application/ports/project-repository.port'
import type { Project, ProjectMember, ProjectMemberRole } from '@/core/projects/domain/project'
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

function toProjectMember(dto: ProjectMemberDto): ProjectMember {
  return {
    projectId: dto.project_id,
    userId: dto.user_id,
    role: dto.role,
    createdAt: dto.created_at,
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
}

import type { Project, ProjectMember, ProjectMemberRole, ProjectLabel, ProjectComponent, ProjectType, ProjectVisibility } from '@/core/projects/domain/project'

export interface CreateProjectInput {
  key: string
  name: string
  description: string
  type: ProjectType
  leadId: string
  visibility: ProjectVisibility
}

export interface ProjectRepositoryPort {
  list(): Promise<Project[]>
  get(id: string): Promise<Project>
  create(input: CreateProjectInput): Promise<Project>
  update(id: string, input: Omit<CreateProjectInput, 'key'>): Promise<Project>
  remove(id: string): Promise<void>
  listMembers(projectId: string): Promise<ProjectMember[]>
  addMember(projectId: string, userId: string, role: ProjectMemberRole): Promise<ProjectMember>
  updateMember(projectId: string, userId: string, role: ProjectMemberRole): Promise<ProjectMember>
  removeMember(projectId: string, userId: string): Promise<void>
  listLabels(projectId: string): Promise<ProjectLabel[]>
  createLabel(projectId: string, name: string, color: string): Promise<ProjectLabel>
  deleteLabel(projectId: string, labelId: string): Promise<void>
  listComponents(projectId: string): Promise<ProjectComponent[]>
  createComponent(projectId: string, name: string, description: string): Promise<ProjectComponent>
  deleteComponent(projectId: string, componentId: string): Promise<void>
}

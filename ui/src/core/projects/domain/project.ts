export type ProjectType = 'kanban' | 'scrum'
export type ProjectVisibility = 'private' | 'open'

export interface Project {
  id: string
  organizationId: string
  key: string
  name: string
  description: string
  type: ProjectType
  leadId: string
  visibility: ProjectVisibility
  archivedAt: string | null
  createdAt: string
  updatedAt: string
}

export type ProjectMemberRole = 'lead' | 'contributor' | 'viewer'

export interface ProjectMember {
  projectId: string
  userId: string
  role: ProjectMemberRole
  createdAt: string
}

export interface ProjectLabel {
  id: string
  projectId: string
  name: string
  color: string
  createdAt: string
  updatedAt: string
}

export interface ProjectComponent {
  id: string
  projectId: string
  name: string
  description: string
  createdAt: string
  updatedAt: string
}

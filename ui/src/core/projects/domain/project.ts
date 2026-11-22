export type ProjectType = 'kanban' | 'scrum'
export type ProjectVisibility = 'private' | 'team'

export interface Project {
  id: string
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

export type ProjectType = 'kanban' | 'scrum';

/** Open projects are visible to every member of the organization; private ones only to their members. */
export type ProjectVisibility = 'private' | 'open';

/** Role of a person inside a project. The global organization admin is not a member role. */
export type ProjectMemberRole = 'lead' | 'contributor' | 'viewer';

export interface Project {
  id: string;
  organizationId: string;
  key: string;
  name: string;
  description: string;
  type: ProjectType;
  leadId: string;
  visibility: ProjectVisibility;
  archivedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface ProjectMember {
  projectId: string;
  userId: string;
  role: ProjectMemberRole;
  createdAt: string;
}

export interface ProjectLabel {
  id: string;
  projectId: string;
  name: string;
  color: string;
  createdAt: string;
  updatedAt: string;
}

export interface ProjectComponent {
  id: string;
  projectId: string;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateProjectInput {
  organizationId: string;
  key: string;
  name: string;
  description: string;
  type: ProjectType;
  /** Optional. When omitted the organization default applies. The creator becomes the lead. */
  visibility?: ProjectVisibility;
}

export interface UpdateProjectInput {
  name: string;
  description: string;
  type: ProjectType;
  leadId: string;
  visibility: ProjectVisibility;
}

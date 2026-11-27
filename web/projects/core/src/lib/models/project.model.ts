export type ProjectType = 'kanban' | 'scrum';

/** Template a project was created from. It defines the board, the statuses and whether sprints exist (scrum only). */
export type ProjectTemplate = 'scrum' | 'kanban' | 'task_tracking' | 'bug_tracking';

/** Who receives an issue created without an assignee. */
export type DefaultAssigneePolicy = 'unassigned' | 'lead';

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
  /**
   * Template, color, icon and assignee policy are always set by the API mapper. They are optional in
   * the type only so that hand-built fixtures of older screens keep compiling.
   */
  template?: ProjectTemplate;
  color?: string;
  icon?: string;
  defaultAssigneePolicy?: DefaultAssigneePolicy;
  archivedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface ProjectMember {
  projectId: string;
  userId: string;
  role: ProjectMemberRole;
  /** Best role across the direct membership and every team grant. Absent on older responses. */
  effectiveRole?: ProjectMemberRole;
  /** True when the person reaches the project only through a team. */
  viaTeam?: boolean;
  teamId?: string;
  teamName?: string;
  /** Absent for people reached only through a team. */
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
  /** Optional. When set it wins over the legacy type field. */
  template?: ProjectTemplate;
  color?: string;
  icon?: string;
  defaultAssigneePolicy?: DefaultAssigneePolicy;
  /** People added as contributors. */
  memberIds?: string[];
  /** Teams that receive the contributor role. */
  teamIds?: string[];
}

export interface UpdateProjectInput {
  name: string;
  description: string;
  type: ProjectType;
  leadId: string;
  visibility: ProjectVisibility;
  color?: string;
  icon?: string;
  defaultAssigneePolicy?: DefaultAssigneePolicy;
}

export type ProjectArchiveFilter = 'exclude' | 'include' | 'only';
export type ProjectSortColumn = 'name' | 'key' | 'created_at' | 'updated_at';

/** Filters of the paginated project list. Every field is optional. */
export interface ProjectListFilters {
  q?: string;
  template?: ProjectTemplate;
  /** Legacy alias: scrum equals template scrum, kanban does not filter by itself. */
  type?: ProjectType;
  leadId?: string;
  starred?: boolean;
  archived?: ProjectArchiveFilter;
  sort?: ProjectSortColumn;
  desc?: boolean;
  page?: number;
  pageSize?: number;
}

export interface ProjectListMeta {
  total: number;
  page: number;
  pageSize: number;
}

export interface ProjectPage {
  items: Project[];
  meta: ProjectListMeta;
}

export interface ProjectKeyAvailability {
  available: boolean;
  /** Normalized key (trimmed, upper case). */
  key: string;
}

/** Who holds a permission in a project scheme. reporter, assignee and watcher only apply on an issue. */
export type PermissionHolder =
  'lead' | 'contributor' | 'viewer' | 'org_admin' | 'reporter' | 'assignee' | 'watcher';

export interface PermissionGrant {
  permission: string;
  holders: PermissionHolder[];
}

export interface PermissionScheme {
  projectId: string;
  /** True when the project has no rules of its own and uses the default scheme. */
  isDefault: boolean;
  grants: PermissionGrant[];
}

/** Effective role of the caller in a project (none when it has no role). */
export type EffectiveRole = ProjectMemberRole | 'none';

export interface EffectivePermissions {
  role: EffectiveRole;
  permissions: string[];
}

export interface ProjectTeamRole {
  projectId: string;
  teamId: string;
  teamName: string;
  role: ProjectMemberRole;
  createdAt: string;
}

/** Starred element. For projects it carries name, key and color. */
export interface FavoriteItem {
  type: string;
  id: string;
  name: string;
  key: string;
  color: string;
  /** When the star was set. */
  createdAt: string;
}

/** Recently visited element. createdAt is the creation date of the element itself. */
export interface RecentProjectItem {
  type: string;
  id: string;
  name: string;
  key: string;
  color: string;
  createdAt: string;
  viewedAt: string;
}

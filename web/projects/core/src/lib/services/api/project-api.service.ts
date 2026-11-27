import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map, throwError } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  CreateProjectInput,
  DefaultAssigneePolicy,
  Project,
  ProjectComponent,
  ProjectKeyAvailability,
  ProjectLabel,
  ProjectListFilters,
  ProjectMember,
  ProjectMemberRole,
  ProjectPage,
  ProjectTemplate,
  ProjectType,
  ProjectVisibility,
  UpdateProjectInput,
} from '../../models';
import { ORGANIZATION_ID_PROVIDER, noCurrentOrganizationError } from '../../utils/org-id-provider';
import { silentRequest } from '../../utils/silent-request';

interface ProjectDto {
  id: string;
  organization_id: string;
  key: string;
  name: string;
  description: string;
  type: ProjectType;
  lead_id: string;
  visibility: ProjectVisibility;
  template?: ProjectTemplate;
  color?: string;
  icon?: string;
  default_assignee_policy?: DefaultAssigneePolicy;
  archived_at: string | null;
  created_at: string;
  updated_at: string;
}

interface ProjectMemberDto {
  project_id: string;
  user_id: string;
  role: ProjectMemberRole;
  effective_role?: ProjectMemberRole;
  via_team?: boolean;
  team_id?: string;
  team_name?: string;
  created_at?: string;
}

interface ProjectListDto {
  data: ProjectDto[];
  meta?: { total: number; page: number; page_size: number };
}

interface LabelDto {
  id: string;
  project_id: string;
  name: string;
  color: string;
  created_at: string;
  updated_at: string;
}

interface ComponentDto {
  id: string;
  project_id: string;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
}

function toProject(dto: ProjectDto): Project {
  return {
    id: dto.id,
    organizationId: dto.organization_id,
    key: dto.key,
    name: dto.name,
    description: dto.description,
    type: dto.type,
    leadId: dto.lead_id,
    visibility: dto.visibility,
    template: dto.template ?? dto.type,
    color: dto.color ?? '',
    icon: dto.icon ?? '',
    defaultAssigneePolicy: dto.default_assignee_policy ?? 'unassigned',
    archivedAt: dto.archived_at,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

function toProjectMember(dto: ProjectMemberDto): ProjectMember {
  const member: ProjectMember = {
    projectId: dto.project_id,
    userId: dto.user_id,
    role: dto.role,
    createdAt: dto.created_at ?? '',
  };
  if (dto.effective_role !== undefined) member.effectiveRole = dto.effective_role;
  if (dto.via_team !== undefined) member.viaTeam = dto.via_team;
  if (dto.team_id !== undefined) member.teamId = dto.team_id;
  if (dto.team_name !== undefined) member.teamName = dto.team_name;
  return member;
}

const PROJECT_KEY_PATTERN = /^[A-Za-z][A-Za-z0-9]{1,9}$/;
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Whether the identifier is a UUID (otherwise it is a project key such as CORE). */
export function isProjectUuid(identifier: string): boolean {
  return UUID_PATTERN.test(identifier);
}

function toProjectLabel(dto: LabelDto): ProjectLabel {
  return {
    id: dto.id,
    projectId: dto.project_id,
    name: dto.name,
    color: dto.color,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

function toProjectComponent(dto: ComponentDto): ProjectComponent {
  return {
    id: dto.id,
    projectId: dto.project_id,
    name: dto.name,
    description: dto.description,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

@Injectable({ providedIn: 'root' })
export class ProjectApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);
  private readonly organizationId = inject(ORGANIZATION_ID_PROVIDER);

  /** Query scope for a key: the active organization disambiguates keys shared across organizations. */
  private keyScope(identifier: string): HttpParams {
    const organizationId = this.organizationId();
    return PROJECT_KEY_PATTERN.test(identifier) && organizationId
      ? new HttpParams().set('organization_id', organizationId)
      : new HttpParams();
  }

  /**
   * Projects of the active organization (first 100, active only). Fails with org.no_current when no
   * organization is active. Use listPage for filters and pagination.
   */
  list(): Observable<Project[]> {
    return this.listPage({ pageSize: 100 }).pipe(map((page) => page.items));
  }

  /** One page of the projects of the active organization with filters and sorting. */
  listPage(filters: ProjectListFilters = {}): Observable<ProjectPage> {
    const organizationId = this.organizationId();
    if (!organizationId) return throwError(() => noCurrentOrganizationError());
    let params = new HttpParams().set('organization_id', organizationId);
    const text = filters.q?.trim();
    if (text) params = params.set('q', text);
    if (filters.template) params = params.set('template', filters.template);
    if (filters.type) params = params.set('type', filters.type);
    if (filters.leadId) params = params.set('lead_id', filters.leadId);
    if (filters.starred !== undefined) params = params.set('starred', String(filters.starred));
    if (filters.archived) params = params.set('archived', filters.archived);
    if (filters.sort) params = params.set('sort', filters.sort);
    if (filters.desc !== undefined) params = params.set('desc', String(filters.desc));
    if (filters.page !== undefined) params = params.set('page', String(filters.page));
    if (filters.pageSize !== undefined) params = params.set('page_size', String(filters.pageSize));
    return this.http.get<ProjectListDto>(`${this.baseUrl}/projects`, { params }).pipe(
      map((response) => ({
        items: response.data.map(toProject),
        meta: {
          total: response.meta?.total ?? response.data.length,
          page: response.meta?.page ?? filters.page ?? 1,
          pageSize: response.meta?.page_size ?? filters.pageSize ?? response.data.length,
        },
      })),
    );
  }

  /**
   * One project by UUID or key. A key is sent with the active organization so that a key used in
   * several organizations of the caller resolves to the right one.
   */
  get(identifier: string, options: { readonly silent?: boolean } = {}): Observable<Project> {
    return this.http
      .get<{ data: ProjectDto }>(`${this.baseUrl}/projects/${encodeURIComponent(identifier)}`, {
        params: this.keyScope(identifier),
        context: options.silent ? silentRequest() : undefined,
      })
      .pipe(map((response) => toProject(response.data)));
  }

  /** Whether a key is valid and free among the active projects of the active organization. */
  keyAvailable(key: string): Observable<ProjectKeyAvailability> {
    const organizationId = this.organizationId();
    if (!organizationId) return throwError(() => noCurrentOrganizationError());
    const params = new HttpParams().set('organization_id', organizationId).set('key', key);
    return this.http
      .get<{ data: ProjectKeyAvailability }>(`${this.baseUrl}/projects/key-available`, { params })
      .pipe(map((response) => ({ available: response.data.available, key: response.data.key })));
  }

  /** The project becomes read-only. */
  archive(id: string): Observable<Project> {
    return this.http
      .post<{ data: ProjectDto }>(`${this.baseUrl}/projects/${id}/archive`, null, {
        params: this.keyScope(id),
      })
      .pipe(map((response) => toProject(response.data)));
  }

  restore(id: string): Observable<Project> {
    return this.http
      .post<{ data: ProjectDto }>(`${this.baseUrl}/projects/${id}/restore`, null, {
        params: this.keyScope(id),
      })
      .pipe(map((response) => toProject(response.data)));
  }

  /** The lead is never sent: the creator becomes the lead. Optional fields are omitted when not given. */
  create(input: CreateProjectInput): Observable<Project> {
    const body: Record<string, unknown> = {
      organization_id: input.organizationId,
      key: input.key,
      name: input.name,
      description: input.description,
      type: input.type,
    };
    if (input.visibility !== undefined) body['visibility'] = input.visibility;
    if (input.template !== undefined) body['template'] = input.template;
    if (input.color !== undefined) body['color'] = input.color;
    if (input.icon !== undefined) body['icon'] = input.icon;
    if (input.defaultAssigneePolicy !== undefined) {
      body['default_assignee_policy'] = input.defaultAssigneePolicy;
    }
    if (input.memberIds !== undefined) body['member_ids'] = input.memberIds;
    if (input.teamIds !== undefined) body['team_ids'] = input.teamIds;
    return this.http
      .post<{ data: ProjectDto }>(`${this.baseUrl}/projects`, body)
      .pipe(map((response) => toProject(response.data)));
  }

  update(id: string, input: UpdateProjectInput): Observable<Project> {
    const body: Record<string, unknown> = {
      name: input.name,
      description: input.description,
      type: input.type,
      lead_id: input.leadId,
      visibility: input.visibility,
    };
    if (input.color !== undefined) body['color'] = input.color;
    if (input.icon !== undefined) body['icon'] = input.icon;
    if (input.defaultAssigneePolicy !== undefined) {
      body['default_assignee_policy'] = input.defaultAssigneePolicy;
    }
    return this.http
      .patch<{ data: ProjectDto }>(`${this.baseUrl}/projects/${id}`, body, {
        params: this.keyScope(id),
      })
      .pipe(map((response) => toProject(response.data)));
  }

  /** Soft delete. `confirmation` must repeat the exact project name (project.confirmation_mismatch otherwise). */
  remove(id: string, confirmation: string): Observable<void> {
    return this.http
      .delete(`${this.baseUrl}/projects/${id}`, {
        body: { confirmation },
        params: this.keyScope(id),
      })
      .pipe(map(() => undefined));
  }

  /** With `silent`, a failure is left to the caller (a page that renders its own error state), not a toast. */
  listMembers(
    projectId: string,
    options: { readonly silent?: boolean } = {},
  ): Observable<ProjectMember[]> {
    return this.http
      .get<{ data: ProjectMemberDto[] }>(`${this.baseUrl}/projects/${projectId}/members`, {
        context: options.silent ? silentRequest() : undefined,
      })
      .pipe(map((response) => response.data.map(toProjectMember)));
  }

  addMember(projectId: string, userId: string, role: ProjectMemberRole): Observable<ProjectMember> {
    return this.http
      .post<{ data: ProjectMemberDto }>(`${this.baseUrl}/projects/${projectId}/members`, {
        user_id: userId,
        role,
      })
      .pipe(map((response) => toProjectMember(response.data)));
  }

  updateMember(
    projectId: string,
    userId: string,
    role: ProjectMemberRole,
  ): Observable<ProjectMember> {
    return this.http
      .patch<{ data: ProjectMemberDto }>(
        `${this.baseUrl}/projects/${projectId}/members/${userId}`,
        {
          role,
        },
      )
      .pipe(map((response) => toProjectMember(response.data)));
  }

  removeMember(projectId: string, userId: string): Observable<void> {
    return this.http
      .delete(`${this.baseUrl}/projects/${projectId}/members/${userId}`)
      .pipe(map(() => undefined));
  }

  listLabels(projectId: string): Observable<ProjectLabel[]> {
    return this.http
      .get<{ data: LabelDto[] }>(`${this.baseUrl}/projects/${projectId}/labels`)
      .pipe(map((response) => response.data.map(toProjectLabel)));
  }

  createLabel(projectId: string, name: string, color: string): Observable<ProjectLabel> {
    return this.http
      .post<{ data: LabelDto }>(`${this.baseUrl}/projects/${projectId}/labels`, { name, color })
      .pipe(map((response) => toProjectLabel(response.data)));
  }

  deleteLabel(projectId: string, labelId: string): Observable<void> {
    return this.http
      .delete(`${this.baseUrl}/projects/${projectId}/labels/${labelId}`)
      .pipe(map(() => undefined));
  }

  listComponents(projectId: string): Observable<ProjectComponent[]> {
    return this.http
      .get<{ data: ComponentDto[] }>(`${this.baseUrl}/projects/${projectId}/components`)
      .pipe(map((response) => response.data.map(toProjectComponent)));
  }

  createComponent(
    projectId: string,
    name: string,
    description: string,
  ): Observable<ProjectComponent> {
    return this.http
      .post<{ data: ComponentDto }>(`${this.baseUrl}/projects/${projectId}/components`, {
        name,
        description,
      })
      .pipe(map((response) => toProjectComponent(response.data)));
  }

  deleteComponent(projectId: string, componentId: string): Observable<void> {
    return this.http
      .delete(`${this.baseUrl}/projects/${projectId}/components/${componentId}`)
      .pipe(map(() => undefined));
  }
}

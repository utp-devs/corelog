import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map, throwError } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  CreateProjectInput,
  Project,
  ProjectComponent,
  ProjectLabel,
  ProjectMember,
  ProjectMemberRole,
  ProjectType,
  ProjectVisibility,
  UpdateProjectInput,
} from '../../models';
import { ORGANIZATION_ID_PROVIDER, noCurrentOrganizationError } from '../../utils/org-id-provider';

interface ProjectDto {
  id: string;
  organization_id: string;
  key: string;
  name: string;
  description: string;
  type: ProjectType;
  lead_id: string;
  visibility: ProjectVisibility;
  archived_at: string | null;
  created_at: string;
  updated_at: string;
}

interface ProjectMemberDto {
  project_id: string;
  user_id: string;
  role: ProjectMemberRole;
  created_at: string;
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
    archivedAt: dto.archived_at,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

function toProjectMember(dto: ProjectMemberDto): ProjectMember {
  return {
    projectId: dto.project_id,
    userId: dto.user_id,
    role: dto.role,
    createdAt: dto.created_at,
  };
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

  /** Projects of the active organization. Fails with org.no_current when no organization is active. */
  list(): Observable<Project[]> {
    const organizationId = this.organizationId();
    if (!organizationId) return throwError(() => noCurrentOrganizationError());
    const params = new HttpParams().set('organization_id', organizationId);
    return this.http
      .get<{ data: ProjectDto[] }>(`${this.baseUrl}/projects`, { params })
      .pipe(map((response) => response.data.map(toProject)));
  }

  get(id: string): Observable<Project> {
    return this.http
      .get<{ data: ProjectDto }>(`${this.baseUrl}/projects/${id}`)
      .pipe(map((response) => toProject(response.data)));
  }

  /** The lead is never sent: the creator becomes the lead. Visibility is omitted when not given. */
  create(input: CreateProjectInput): Observable<Project> {
    const body: Record<string, unknown> = {
      organization_id: input.organizationId,
      key: input.key,
      name: input.name,
      description: input.description,
      type: input.type,
    };
    if (input.visibility !== undefined) body['visibility'] = input.visibility;
    return this.http
      .post<{ data: ProjectDto }>(`${this.baseUrl}/projects`, body)
      .pipe(map((response) => toProject(response.data)));
  }

  update(id: string, input: UpdateProjectInput): Observable<Project> {
    return this.http
      .patch<{ data: ProjectDto }>(`${this.baseUrl}/projects/${id}`, {
        name: input.name,
        description: input.description,
        type: input.type,
        lead_id: input.leadId,
        visibility: input.visibility,
      })
      .pipe(map((response) => toProject(response.data)));
  }

  remove(id: string): Observable<void> {
    return this.http.delete(`${this.baseUrl}/projects/${id}`).pipe(map(() => undefined));
  }

  listMembers(projectId: string): Observable<ProjectMember[]> {
    return this.http
      .get<{ data: ProjectMemberDto[] }>(`${this.baseUrl}/projects/${projectId}/members`)
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

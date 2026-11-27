import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  OrganizationStatus,
  OrganizationStatusInput,
  OrganizationStatusPatch,
  WorkflowCategory,
  WorkflowDefinitionInput,
  WorkflowIssue,
  WorkflowMigrationResult,
  WorkflowRule,
  WorkflowSaveResult,
  WorkflowTemplate,
  WorkflowTemplateId,
  WorkflowTransition,
  WorkflowValidation,
  WorkflowView,
} from '../../models/workflow.model';
import { silentRequest } from '../../utils/silent-request';

interface RuleDto {
  type: string;
  params?: Record<string, unknown> | null;
}

interface TransitionDto {
  id: string;
  name: string;
  from?: string[] | null;
  to: string;
  conditions?: RuleDto[] | null;
  validators?: RuleDto[] | null;
  post_functions?: RuleDto[] | null;
  required_fields?: string[] | null;
}

interface WorkflowViewDto {
  statuses: {
    id: string;
    name: string;
    category: WorkflowCategory;
    position: number;
    issue_count: number;
  }[];
  initial_status_id: string;
  transitions: TransitionDto[];
  version: number;
  updated_at: string;
  is_template_default: boolean;
}

interface IssueDto {
  path: string;
  code: string;
  message: string;
}

interface ValidationDto {
  valid: boolean;
  errors?: IssueDto[] | null;
  warnings?: IssueDto[] | null;
  mapping_required?: { status_id: string; name: string; issue_count: number }[] | null;
}

interface TemplateDto {
  id: WorkflowTemplateId;
  name: string;
  description: string;
  definition: {
    initial_ref: string;
    statuses: { ref: string; name: string; category: WorkflowCategory }[];
    transitions: TransitionDto[];
  };
}

interface OrgStatusDto {
  id: string;
  name: string;
  category: WorkflowCategory;
  position: number;
}

interface Envelope<T> {
  data: T;
}

function toRule(dto: RuleDto): WorkflowRule {
  return { type: dto.type, params: dto.params ?? {} };
}

export function toWorkflowTransition(dto: TransitionDto): WorkflowTransition {
  return {
    id: dto.id,
    name: dto.name,
    from: dto.from ?? [],
    to: dto.to,
    conditions: (dto.conditions ?? []).map(toRule),
    validators: (dto.validators ?? []).map(toRule),
    postFunctions: (dto.post_functions ?? []).map(toRule),
    requiredFields: dto.required_fields ?? [],
  };
}

function fromTransition(transition: WorkflowTransition): TransitionDto {
  return {
    id: transition.id,
    name: transition.name,
    from: [...transition.from],
    to: transition.to,
    conditions: transition.conditions.map((rule) => ({
      type: rule.type,
      params: { ...rule.params },
    })),
    validators: transition.validators.map((rule) => ({
      type: rule.type,
      params: { ...rule.params },
    })),
    post_functions: transition.postFunctions.map((rule) => ({
      type: rule.type,
      params: { ...rule.params },
    })),
    required_fields: [...transition.requiredFields],
  };
}

function toView(dto: WorkflowViewDto): WorkflowView {
  return {
    statuses: dto.statuses.map((status) => ({
      id: status.id,
      name: status.name,
      category: status.category,
      position: status.position,
      issueCount: status.issue_count,
    })),
    initialStatusId: dto.initial_status_id,
    transitions: dto.transitions.map(toWorkflowTransition),
    version: dto.version,
    updatedAt: dto.updated_at,
    isTemplateDefault: dto.is_template_default,
  };
}

function toIssues(list: IssueDto[] | null | undefined): WorkflowIssue[] {
  return (list ?? []).map((item) => ({ path: item.path, code: item.code, message: item.message }));
}

function toOrgStatus(dto: OrgStatusDto): OrganizationStatus {
  return { id: dto.id, name: dto.name, category: dto.category, position: dto.position };
}

function definitionBody(input: WorkflowDefinitionInput): Record<string, unknown> {
  return {
    expected_version: input.expectedVersion,
    initial_ref: input.initialRef,
    statuses: input.statuses.map((status) => ({
      ref: status.ref,
      id: status.id,
      name: status.name,
      category: status.category,
    })),
    transitions: input.transitions.map(fromTransition),
    status_mapping: input.statusMapping,
  };
}

/** Project workflows, workflow templates and the status catalog of an organization. */
@Injectable({ providedIn: 'root' })
export class WorkflowApiService {
  private readonly http = inject(HttpClient);
  private readonly apiUrl = inject(API_URL);

  private projectUrl(projectId: string): string {
    return `${this.apiUrl}/projects/${projectId}/workflow`;
  }

  private statusesUrl(orgId: string): string {
    return `${this.apiUrl}/organizations/${orgId}/statuses`;
  }

  get(projectId: string): Observable<WorkflowView> {
    return this.http
      .get<Envelope<WorkflowViewDto>>(this.projectUrl(projectId), { context: silentRequest() })
      .pipe(map((body) => toView(body.data)));
  }

  /** Replaces the workflow. Failures keep their details (path, code, message) and mapping requirements. */
  save(projectId: string, input: WorkflowDefinitionInput): Observable<WorkflowSaveResult> {
    return this.http
      .put<Envelope<WorkflowViewDto & { warnings?: IssueDto[] | null }>>(
        this.projectUrl(projectId),
        definitionBody(input),
        { context: silentRequest() },
      )
      .pipe(
        map((body) => ({ workflow: toView(body.data), warnings: toIssues(body.data.warnings) })),
      );
  }

  validate(projectId: string, input: WorkflowDefinitionInput): Observable<WorkflowValidation> {
    return this.http
      .post<Envelope<ValidationDto>>(
        `${this.projectUrl(projectId)}/validate`,
        definitionBody(input),
        {
          context: silentRequest(),
        },
      )
      .pipe(
        map((body) => ({
          valid: body.data.valid,
          errors: toIssues(body.data.errors),
          warnings: toIssues(body.data.warnings),
          mappingRequired: (body.data.mapping_required ?? []).map((item) => ({
            statusId: item.status_id,
            name: item.name,
            issueCount: item.issue_count,
          })),
        })),
      );
  }

  /** Applies a template to the project. */
  reset(
    projectId: string,
    template: WorkflowTemplateId,
    expectedVersion: number,
    statusMapping: Readonly<Record<string, string>> = {},
  ): Observable<WorkflowSaveResult> {
    return this.http
      .post<Envelope<WorkflowViewDto & { warnings?: IssueDto[] | null }>>(
        `${this.projectUrl(projectId)}/reset`,
        { template, expected_version: expectedVersion, status_mapping: statusMapping },
        { context: silentRequest() },
      )
      .pipe(
        map((body) => ({ workflow: toView(body.data), warnings: toIssues(body.data.warnings) })),
      );
  }

  migrateIssues(
    projectId: string,
    statusMapping: Readonly<Record<string, string>>,
  ): Observable<WorkflowMigrationResult> {
    return this.http
      .post<Envelope<{ moved: number }>>(
        `${this.projectUrl(projectId)}/migrate-issues`,
        { status_mapping: statusMapping },
        { context: silentRequest() },
      )
      .pipe(map((body) => ({ moved: body.data.moved })));
  }

  listTemplates(): Observable<WorkflowTemplate[]> {
    return this.http
      .get<Envelope<TemplateDto[]>>(`${this.apiUrl}/workflow-templates`, {
        context: silentRequest(),
      })
      .pipe(
        map((body) =>
          body.data.map((template) => ({
            id: template.id,
            name: template.name,
            description: template.description,
            initialRef: template.definition.initial_ref,
            statuses: template.definition.statuses,
            transitions: template.definition.transitions.map(toWorkflowTransition),
          })),
        ),
      );
  }

  listStatuses(orgId: string): Observable<OrganizationStatus[]> {
    return this.http
      .get<Envelope<OrgStatusDto[]>>(this.statusesUrl(orgId), { context: silentRequest() })
      .pipe(map((body) => body.data.map(toOrgStatus)));
  }

  createStatus(orgId: string, input: OrganizationStatusInput): Observable<OrganizationStatus> {
    return this.http
      .post<Envelope<OrgStatusDto>>(this.statusesUrl(orgId), input, { context: silentRequest() })
      .pipe(map((body) => toOrgStatus(body.data)));
  }

  updateStatus(
    orgId: string,
    id: string,
    patch: OrganizationStatusPatch,
  ): Observable<OrganizationStatus> {
    return this.http
      .patch<Envelope<OrgStatusDto>>(`${this.statusesUrl(orgId)}/${id}`, patch, {
        context: silentRequest(),
      })
      .pipe(map((body) => toOrgStatus(body.data)));
  }

  /** Deletes a status; `reassignTo` receives its issues, history and workflows when it is in use. */
  deleteStatus(orgId: string, id: string, reassignTo?: string): Observable<number> {
    let params = new HttpParams();
    if (reassignTo) params = params.set('reassign_to', reassignTo);
    return this.http
      .delete<Envelope<{ reassigned_issues?: number }>>(`${this.statusesUrl(orgId)}/${id}`, {
        params,
        context: silentRequest(),
      })
      .pipe(map((body) => body.data?.reassigned_issues ?? 0));
  }

  reorderStatuses(orgId: string, ids: readonly string[]): Observable<OrganizationStatus[]> {
    return this.http
      .put<Envelope<OrgStatusDto[]>>(
        `${this.statusesUrl(orgId)}/reorder`,
        { status_ids: ids },
        { context: silentRequest() },
      )
      .pipe(map((body) => body.data.map(toOrgStatus)));
  }
}

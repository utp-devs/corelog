import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type {
  AutomationAction,
  AutomationCondition,
  AutomationRule,
  AutomationRuleDraft,
  AutomationRun,
  AutomationRunStatus,
  AutomationTrigger,
  AutomationValidation,
} from '../../models';
import { silentRequest } from '../../utils/silent-request';

interface TriggerDto {
  type: AutomationTrigger['type'];
  field?: string;
  from?: string;
  to?: string;
  every_minutes?: number;
  query?: string;
}
interface ConditionDto {
  type: AutomationCondition['type'];
  query?: string;
  field?: string;
  value?: string;
  values?: string[];
  role?: string;
}
interface ActionDto {
  type: AutomationAction['type'];
  status?: string;
  who?: string;
  user_id?: string;
  team_id?: string;
  field?: string;
  value?: string;
  text?: string;
  label?: string;
  url?: string;
}
interface RuleDto {
  id: string;
  project_id?: string | null;
  name: string;
  enabled?: boolean;
  trigger: TriggerDto;
  conditions?: ConditionDto[] | null;
  actions?: ActionDto[] | null;
  version: number;
  run_count?: number;
  last_run_at?: string | null;
  created_at: string;
  updated_at: string;
}
interface RunDto {
  id: string;
  rule_id: string;
  issue_id?: string | null;
  status: AutomationRunStatus;
  detail?: string;
  created_at: string;
}

/** Drops the keys without a value so the server receives only what the rule type uses. */
function compact<T extends object>(source: T): T {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(source)) {
    if (value === undefined || value === null || value === '') continue;
    if (Array.isArray(value) && value.length === 0) continue;
    out[key] = value;
  }
  return out as T;
}

export function triggerToDto(t: AutomationTrigger): TriggerDto {
  return compact({
    type: t.type,
    field: t.field,
    from: t.from,
    to: t.to,
    every_minutes: t.everyMinutes,
    query: t.query,
  });
}

export function conditionToDto(c: AutomationCondition): ConditionDto {
  return compact({
    type: c.type,
    query: c.query,
    field: c.field,
    value: c.value,
    values: c.values ? [...c.values] : undefined,
    role: c.role,
  });
}

export function actionToDto(a: AutomationAction): ActionDto {
  return compact({
    type: a.type,
    status: a.status,
    who: a.who,
    user_id: a.userId,
    team_id: a.teamId,
    field: a.field,
    value: a.value,
    text: a.text,
    label: a.label,
    url: a.url,
  });
}

function ruleBody(draft: Partial<AutomationRuleDraft>): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  if (draft.name !== undefined) body['name'] = draft.name;
  if (draft.enabled !== undefined) body['enabled'] = draft.enabled;
  if (draft.trigger) body['trigger'] = triggerToDto(draft.trigger);
  if (draft.conditions) body['conditions'] = draft.conditions.map(conditionToDto);
  if (draft.actions) body['actions'] = draft.actions.map(actionToDto);
  return body;
}

export function toRule(dto: RuleDto): AutomationRule {
  return {
    id: dto.id,
    projectId: dto.project_id ?? null,
    name: dto.name,
    enabled: dto.enabled ?? false,
    trigger: {
      type: dto.trigger.type,
      field: dto.trigger.field,
      from: dto.trigger.from,
      to: dto.trigger.to,
      everyMinutes: dto.trigger.every_minutes,
      query: dto.trigger.query,
    },
    conditions: (dto.conditions ?? []).map((c) => ({
      type: c.type,
      query: c.query,
      field: c.field,
      value: c.value,
      values: c.values,
      role: c.role,
    })),
    actions: (dto.actions ?? []).map((a) => ({
      type: a.type,
      status: a.status,
      who: a.who,
      userId: a.user_id,
      teamId: a.team_id,
      field: a.field,
      value: a.value,
      text: a.text,
      label: a.label,
      url: a.url,
    })),
    version: dto.version,
    runCount: dto.run_count ?? 0,
    lastRunAt: dto.last_run_at ?? null,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  };
}

function toRun(dto: RunDto): AutomationRun {
  return {
    id: dto.id,
    ruleId: dto.rule_id,
    issueId: dto.issue_id ?? null,
    status: dto.status,
    detail: dto.detail ?? '',
    createdAt: dto.created_at,
  };
}

/** Automation rules of a project: CRUD with optimistic versioning, enable/disable, runs and dry validation. */
@Injectable({ providedIn: 'root' })
export class AutomationApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  private rule(id: string): string {
    return `${this.baseUrl}/automation-rules/${encodeURIComponent(id)}`;
  }

  list(projectId: string): Observable<readonly AutomationRule[]> {
    const url = `${this.baseUrl}/projects/${encodeURIComponent(projectId)}/automation-rules`;
    return this.http
      .get<{ data: RuleDto[] | null }>(url, { context: silentRequest() })
      .pipe(map((body) => (body.data ?? []).map(toRule)));
  }

  get(id: string): Observable<AutomationRule> {
    return this.http
      .get<{ data: RuleDto }>(this.rule(id), { context: silentRequest() })
      .pipe(map(({ data }) => toRule(data)));
  }

  create(projectId: string, draft: AutomationRuleDraft): Observable<AutomationRule> {
    const url = `${this.baseUrl}/projects/${encodeURIComponent(projectId)}/automation-rules`;
    return this.http
      .post<{ data: RuleDto }>(url, ruleBody(draft), { context: silentRequest() })
      .pipe(map(({ data }) => toRule(data)));
  }

  /** Saves the rule; `version` is the one the editor loaded, a newer one on the server answers 409. */
  update(
    id: string,
    version: number,
    draft: Partial<AutomationRuleDraft>,
  ): Observable<AutomationRule> {
    return this.http
      .patch<{ data: RuleDto }>(
        this.rule(id),
        { ...ruleBody(draft), version },
        { context: silentRequest() },
      )
      .pipe(map(({ data }) => toRule(data)));
  }

  remove(id: string): Observable<void> {
    return this.http.delete<void>(this.rule(id), { context: silentRequest() });
  }

  setEnabled(id: string, enabled: boolean): Observable<AutomationRule> {
    return this.http
      .post<{ data: RuleDto }>(
        `${this.rule(id)}/${enabled ? 'enable' : 'disable'}`,
        {},
        { context: silentRequest() },
      )
      .pipe(map(({ data }) => toRule(data)));
  }

  runs(
    id: string,
    options: { readonly status?: AutomationRunStatus; readonly limit?: number } = {},
  ): Observable<readonly AutomationRun[]> {
    let params = new HttpParams().set('limit', options.limit ?? 50);
    if (options.status) params = params.set('status', options.status);
    return this.http
      .get<{ data: RunDto[] | null }>(`${this.rule(id)}/runs`, { params, context: silentRequest() })
      .pipe(map((body) => (body.data ?? []).map(toRun)));
  }

  validate(
    projectId: string,
    draft: AutomationRuleDraft,
    sampleIssueId?: string | null,
  ): Observable<AutomationValidation> {
    const body: Record<string, unknown> = { project_id: projectId, ...ruleBody(draft) };
    delete body['enabled'];
    if (sampleIssueId) body['issue_id'] = sampleIssueId;
    return this.http
      .post<{
        data: { valid: boolean; errors?: string[]; planned_actions?: string[] };
      }>(`${this.baseUrl}/automation-rules/validate`, body, { context: silentRequest() })
      .pipe(
        map(({ data }) => ({
          valid: data.valid,
          errors: data.errors ?? [],
          plannedActions: data.planned_actions ?? [],
        })),
      );
  }
}

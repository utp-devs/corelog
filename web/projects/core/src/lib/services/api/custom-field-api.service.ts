import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  CreateCustomFieldInput,
  CustomField,
  CustomFieldContext,
  FieldLayout,
  FieldLayoutEntryInput,
  PatchCustomFieldInput,
} from '../../models/custom-field.model';
import { CustomFieldOption, CustomFieldType } from '../../models/issue-view.model';
import { silentRequest } from '../../utils/silent-request';

type Dto = Record<string, unknown>;

interface Envelope<T> {
  data: T;
}

function toOption(value: unknown): CustomFieldOption {
  const dto = value as Dto;
  return {
    id: String(dto['id']),
    label: String(dto['label']),
    color: String(dto['color'] ?? ''),
    position: Number(dto['position'] ?? 0),
    archived: dto['archived'] === true,
  };
}

function toField(dto: Dto): CustomField {
  const settings = (dto['settings'] as Dto | undefined) ?? {};
  return {
    id: String(dto['id']),
    name: String(dto['name']),
    description: String(dto['description'] ?? ''),
    type: String(dto['type']) as CustomFieldType,
    options: ((dto['options'] as unknown[] | undefined) ?? []).map(toOption),
    min: (settings['min'] as number | null | undefined) ?? null,
    max: (settings['max'] as number | null | undefined) ?? null,
    position: Number(dto['position'] ?? 0),
    contexts: ((dto['contexts'] as Dto[] | undefined) ?? []).map(toContext),
  };
}

function toContext(dto: Dto): CustomFieldContext {
  return {
    projectId: (dto['project_id'] as string | null) ?? null,
    issueTypeId: (dto['issue_type_id'] as string | null) ?? null,
    required: dto['required'] === true,
    defaultValue: dto['default_value'] ?? null,
  };
}

function toLayout(dto: Dto): FieldLayout {
  return {
    issueTypeId: String(dto['issue_type_id']),
    configured: dto['configured'] === true,
    items: ((dto['items'] as Dto[] | undefined) ?? []).map((item) => ({
      kind: item['kind'] === 'custom' ? 'custom' : 'system',
      key: (item['key'] as string | undefined) ?? null,
      fieldId: (item['field_id'] as string | undefined) ?? null,
      name: String(item['name'] ?? item['key'] ?? ''),
      type: String(item['type'] ?? ''),
      options: ((item['options'] as unknown[] | undefined) ?? []).map(toOption),
      required: item['required'] === true,
    })),
  };
}

/** Custom field definitions of an organization and the field layout of a project. */
@Injectable({ providedIn: 'root' })
export class CustomFieldApiService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(API_URL);

  list(organizationId: string): Observable<CustomField[]> {
    return this.http
      .get<Envelope<Dto[]>>(`${this.api}/organizations/${organizationId}/custom-fields`, {
        context: silentRequest(),
      })
      .pipe(map((body) => body.data.map(toField)));
  }

  create(organizationId: string, input: CreateCustomFieldInput): Observable<CustomField> {
    const settings =
      input.min !== undefined || input.max !== undefined
        ? { min: input.min, max: input.max }
        : undefined;
    return this.http
      .post<Envelope<Dto>>(
        `${this.api}/organizations/${organizationId}/custom-fields`,
        {
          name: input.name,
          description: input.description,
          type: input.type,
          options: input.options,
          settings,
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toField(body.data)));
  }

  patch(organizationId: string, id: string, input: PatchCustomFieldInput): Observable<CustomField> {
    const settings =
      input.min !== undefined || input.max !== undefined
        ? { min: input.min, max: input.max }
        : undefined;
    return this.http
      .patch<Envelope<Dto>>(
        `${this.api}/organizations/${organizationId}/custom-fields/${id}`,
        { name: input.name, description: input.description, options: input.options, settings },
        { context: silentRequest() },
      )
      .pipe(map((body) => toField(body.data)));
  }

  remove(organizationId: string, id: string): Observable<void> {
    return this.http.delete<void>(
      `${this.api}/organizations/${organizationId}/custom-fields/${id}`,
      {
        context: silentRequest(),
      },
    );
  }

  replaceContexts(
    organizationId: string,
    id: string,
    contexts: readonly CustomFieldContext[],
  ): Observable<CustomField> {
    return this.http
      .put<Envelope<Dto>>(
        `${this.api}/organizations/${organizationId}/custom-fields/${id}/contexts`,
        {
          contexts: contexts.map((context) => ({
            project_id: context.projectId,
            issue_type_id: context.issueTypeId,
            required: context.required,
            default_value: context.defaultValue,
          })),
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toField(body.data)));
  }

  layout(projectId: string, issueTypeId: string): Observable<FieldLayout> {
    const params = new HttpParams().set('issue_type_id', issueTypeId);
    return this.http
      .get<Envelope<Dto>>(`${this.api}/projects/${projectId}/fields`, {
        params,
        context: silentRequest(),
      })
      .pipe(map((body) => toLayout(body.data)));
  }

  saveLayout(
    projectId: string,
    issueTypeId: string,
    items: readonly FieldLayoutEntryInput[],
  ): Observable<FieldLayout> {
    const params = new HttpParams().set('issue_type_id', issueTypeId);
    return this.http
      .put<Envelope<Dto>>(
        `${this.api}/projects/${projectId}/fields`,
        {
          items: items.map((item) => ({
            kind: item.kind,
            key: item.key,
            field_id: item.fieldId,
            required: item.required,
          })),
        },
        { params, context: silentRequest() },
      )
      .pipe(map((body) => toLayout(body.data)));
  }
}

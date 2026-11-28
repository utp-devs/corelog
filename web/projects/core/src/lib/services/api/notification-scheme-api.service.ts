import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type {
  NotificationChannel,
  NotificationEvent,
  NotificationRecipient,
  NotificationRuleInput,
  NotificationScheme,
  NotificationSchemeScope,
} from '../../models/notification-scheme.model';
import { silentRequest } from '../../utils/silent-request';

interface RuleDto {
  readonly event: NotificationEvent;
  readonly recipients?: readonly NotificationRecipient[];
  readonly channels?: readonly NotificationChannel[];
  readonly overridden?: boolean;
}

interface SchemeDto {
  readonly scope: 'project' | 'organization';
  readonly organization_id?: string | null;
  readonly project_id?: string | null;
  readonly is_default?: boolean;
  readonly updated_at?: string | null;
  readonly rules?: readonly RuleDto[];
}

function toScheme(dto: SchemeDto): NotificationScheme {
  return {
    scope: dto.scope,
    organizationId: dto.organization_id ?? null,
    projectId: dto.project_id ?? null,
    isDefault: dto.is_default ?? false,
    updatedAt: dto.updated_at ?? null,
    rules: (dto.rules ?? []).map((rule) => ({
      event: rule.event,
      recipients: rule.recipients ?? [],
      channels: rule.channels ?? [],
      overridden: rule.overridden ?? false,
    })),
  };
}

/** Notification schemes of a project or of the organization defaults. */
@Injectable({ providedIn: 'root' })
export class NotificationSchemeApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  private url(scope: NotificationSchemeScope): string {
    const segment = scope.kind === 'project' ? 'projects' : 'organizations';
    return `${this.baseUrl}/${segment}/${scope.id}/notification-scheme`;
  }

  get(scope: NotificationSchemeScope): Observable<NotificationScheme> {
    return this.http
      .get<{ data: SchemeDto }>(this.url(scope), { context: silentRequest() })
      .pipe(map((response) => toScheme(response.data)));
  }

  save(
    scope: NotificationSchemeScope,
    rules: readonly NotificationRuleInput[],
  ): Observable<NotificationScheme> {
    return this.http
      .put<{ data: SchemeDto }>(this.url(scope), { rules }, { context: silentRequest() })
      .pipe(map((response) => toScheme(response.data)));
  }

  reset(scope: NotificationSchemeScope): Observable<NotificationScheme> {
    return this.http
      .delete<{ data: SchemeDto }>(this.url(scope), { context: silentRequest() })
      .pipe(map((response) => toScheme(response.data)));
  }
}

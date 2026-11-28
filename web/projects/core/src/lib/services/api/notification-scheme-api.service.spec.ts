import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import type { NotificationScheme } from '../../models/notification-scheme.model';
import { NotificationSchemeApiService } from './notification-scheme-api.service';

function setup(): { service: NotificationSchemeApiService; backend: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(NotificationSchemeApiService),
    backend: TestBed.inject(HttpTestingController),
  };
}

const SCHEME = {
  scope: 'project',
  organization_id: 'o-1',
  project_id: 'p-1',
  is_default: false,
  updated_at: '2026-11-27T10:00:00Z',
  rules: [
    {
      event: 'issue_created',
      recipients: [{ type: 'assignee' }, { type: 'user', id: 'u-1' }],
      channels: ['in_app', 'email'],
      overridden: true,
    },
    { event: 'issue_updated', recipients: [], channels: ['in_app'] },
  ],
};

describe('NotificationSchemeApiService', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('reads the scheme of a project and maps it to camelCase', () => {
    const { service, backend } = setup();
    let scheme: NotificationScheme | undefined;
    service.get({ kind: 'project', id: 'p-1' }).subscribe((value) => (scheme = value));
    const request = backend.expectOne('/api/v1/projects/p-1/notification-scheme');
    expect(request.request.method).toBe('GET');
    request.flush({ data: SCHEME });

    expect(scheme).toEqual({
      scope: 'project',
      organizationId: 'o-1',
      projectId: 'p-1',
      isDefault: false,
      updatedAt: '2026-11-27T10:00:00Z',
      rules: [
        {
          event: 'issue_created',
          recipients: [{ type: 'assignee' }, { type: 'user', id: 'u-1' }],
          channels: ['in_app', 'email'],
          overridden: true,
        },
        { event: 'issue_updated', recipients: [], channels: ['in_app'], overridden: false },
      ],
    });
    backend.verify();
  });

  it('saves rules of an organization with a PUT that carries only the rules', () => {
    const { service, backend } = setup();
    const rules = [
      { event: 'issue_created' as const, recipients: [], channels: ['email' as const] },
    ];
    service.save({ kind: 'organization', id: 'o-1' }, rules).subscribe();
    const request = backend.expectOne('/api/v1/organizations/o-1/notification-scheme');
    expect(request.request.method).toBe('PUT');
    expect(request.request.body).toEqual({ rules });
    request.flush({ data: { ...SCHEME, scope: 'organization' } });
    backend.verify();
  });

  it('resets a project scheme with a DELETE', () => {
    const { service, backend } = setup();
    service.reset({ kind: 'project', id: 'p-1' }).subscribe();
    const request = backend.expectOne('/api/v1/projects/p-1/notification-scheme');
    expect(request.request.method).toBe('DELETE');
    request.flush({ data: SCHEME });
    backend.verify();
  });
});

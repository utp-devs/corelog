import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';

import { provideCorelogTesting } from '../../testing';
import { CurrentProjectStore } from '../state/current-project.store';
import { ORGANIZATION_ID_PROVIDER } from '../utils/org-id-provider';
import { PROJECT_ROUTE_PARAM, projectRouteResolver } from './project-route.resolver';

const ORG_ID = '00000000-0000-4000-8000-0000000000aa';
const BASE = '/api/v1';
const PROJECT_UUID = '0f0e0d0c-0b0a-4908-8706-050403020100';

@Component({ template: '<p>board</p>' })
class BoardStub {}

function projectDto(): Record<string, unknown> {
  return {
    id: PROJECT_UUID,
    organization_id: ORG_ID,
    key: 'CORE',
    name: 'CoreLog',
    description: '',
    type: 'kanban',
    lead_id: 'u1',
    visibility: 'open',
    archived_at: null,
    created_at: '2026-11-01T10:00:00Z',
    updated_at: '2026-11-01T10:00:00Z',
  };
}

describe('projectRouteResolver', () => {
  let backend: HttpTestingController;
  let store: CurrentProjectStore;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideCorelogTesting(),
        { provide: ORGANIZATION_ID_PROVIDER, useValue: () => ORG_ID },
        provideRouter([
          {
            path: `projects/:${PROJECT_ROUTE_PARAM}`,
            resolve: { project: projectRouteResolver },
            children: [{ path: 'board', component: BoardStub }],
          },
        ]),
      ],
    });
    backend = TestBed.inject(HttpTestingController);
    store = TestBed.inject(CurrentProjectStore);
  });

  afterEach(() => backend.verify());

  it('uses the route parameter name key', () => {
    expect(PROJECT_ROUTE_PARAM).toBe('key');
  });

  it('opens the project by key and loads its permissions (200)', async () => {
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/projects/CORE/board');
    backend
      .expectOne(`${BASE}/projects/CORE?organization_id=${ORG_ID}`)
      .flush({ data: projectDto() });
    backend
      .expectOne(`${BASE}/projects/${PROJECT_UUID}/permissions/me`)
      .flush({ role: 'lead', permissions: ['browse_project'] });

    expect(store.project()?.key).toBe('CORE');
    expect(store.role()).toBe('lead');
  });

  it('opens the project by UUID', async () => {
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl(`/projects/${PROJECT_UUID}/board`);
    backend.expectOne(`${BASE}/projects/${PROJECT_UUID}`).flush({ data: projectDto() });
    backend
      .expectOne(`${BASE}/projects/${PROJECT_UUID}/permissions/me`)
      .flush({ role: 'viewer', permissions: [] });

    expect(store.projectId()).toBe(PROJECT_UUID);
    expect(store.identifier()).toBe(PROJECT_UUID);
  });

  it('does not block the navigation and keeps a 404 in the store', async () => {
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/projects/NOPE/board');
    backend
      .expectOne(`${BASE}/projects/NOPE?organization_id=${ORG_ID}`)
      .flush({ error: { code: 'project.not_found' } }, { status: 404, statusText: 'Not Found' });

    expect(harness.routeNativeElement?.textContent).toContain('board');
    expect(store.project()).toBeUndefined();
    expect(store.error()).toMatchObject({ status: 404 });
  });

  it('keeps a 403 in the store', async () => {
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/projects/CORE/board');
    backend
      .expectOne(`${BASE}/projects/CORE?organization_id=${ORG_ID}`)
      .flush({ error: { code: 'forbidden' } }, { status: 403, statusText: 'Forbidden' });

    expect(store.error()).toMatchObject({ status: 403 });
  });
});

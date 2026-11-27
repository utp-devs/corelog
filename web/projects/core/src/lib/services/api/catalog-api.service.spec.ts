import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { CatalogApiService } from './catalog-api.service';

const ORG = 'org-1';

describe('CatalogApiService', () => {
  let service: CatalogApiService;
  let backend: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(CatalogApiService);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  it('maps a catalog item to camelCase, with null icon and hierarchy for resolutions', () => {
    let items: unknown;
    service.list(ORG, 'resolutions').subscribe((value) => (items = value));
    backend.expectOne(`/api/v1/organizations/${ORG}/resolutions`).flush({
      data: [
        {
          id: 'r-1',
          name: 'Hecho',
          position: 0,
          is_default: true,
          usage_count: 4,
          created_at: 'a',
          updated_at: 'b',
        },
      ],
    });
    expect(items).toEqual([
      {
        id: 'r-1',
        name: 'Hecho',
        description: '',
        icon: null,
        color: null,
        hierarchyLevel: null,
        position: 0,
        isDefault: true,
        usageCount: 4,
        createdAt: 'a',
        updatedAt: 'b',
      },
    ]);
  });

  it('sends the snake_case body of a create and of a patch', () => {
    service
      .create(ORG, 'issue-types', { name: 'Hotfix', icon: 'bolt', color: 'red', hierarchyLevel: 0 })
      .subscribe();
    const post = backend.expectOne(`/api/v1/organizations/${ORG}/issue-types`);
    expect(post.request.method).toBe('POST');
    expect(post.request.body).toMatchObject({ name: 'Hotfix', hierarchy_level: 0, icon: 'bolt' });
    post.flush({
      data: { id: 'x', name: 'Hotfix', position: 5, is_default: false, usage_count: 0 },
    });

    service.update(ORG, 'priorities', 'p-1', { isDefault: true }).subscribe();
    const patch = backend.expectOne(`/api/v1/organizations/${ORG}/priorities/p-1`);
    expect(patch.request.method).toBe('PATCH');
    expect(patch.request.body).toMatchObject({ is_default: true });
    patch.flush({
      data: { id: 'p-1', name: 'Alta', position: 0, is_default: true, usage_count: 0 },
    });
  });

  it('adds reassign_to only when given', () => {
    service.remove(ORG, 'priorities', 'p-1').subscribe();
    const plain = backend.expectOne(`/api/v1/organizations/${ORG}/priorities/p-1`);
    expect(plain.request.params.has('reassign_to')).toBe(false);
    plain.flush(null, { status: 204, statusText: 'No Content' });

    service.remove(ORG, 'priorities', 'p-1', 'p-2').subscribe();
    const moved = backend.expectOne(`/api/v1/organizations/${ORG}/priorities/p-1?reassign_to=p-2`);
    moved.flush(null, { status: 204, statusText: 'No Content' });
  });
});

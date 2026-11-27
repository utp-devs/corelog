import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { SavedFilter } from '../../models/ticket.model';
import { SavedFilterApiService } from './saved-filter-api.service';

const dto = {
  id: 'f-1',
  name: 'Mis errores',
  filters: { status: 'open', issue_type: 'bug', due_from: '2026-12-01' },
  created_at: 'a',
  updated_at: 'b',
};

describe('SavedFilterApiService', () => {
  let service: SavedFilterApiService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(SavedFilterApiService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpMock.verify());

  it('lists saved filters and maps the criteria to camelCase', () => {
    let filters: SavedFilter[] = [];
    service.list().subscribe((items) => (filters = items));
    httpMock.expectOne('/api/v1/saved-filters').flush({ data: [dto] });

    expect(filters[0]?.criteria).toEqual({
      status: 'open',
      priority: undefined,
      query: undefined,
      issueType: 'bug',
      dueFrom: '2026-12-01',
      dueTo: undefined,
    });
  });

  it('creates a filter sending the criteria in snake_case', () => {
    let created: SavedFilter | undefined;
    service
      .create('Mis errores', { status: 'open', issueType: 'bug' })
      .subscribe((item) => (created = item));

    const request = httpMock.expectOne('/api/v1/saved-filters');
    expect(request.request.body).toMatchObject({
      name: 'Mis errores',
      filters: { status: 'open', issue_type: 'bug' },
    });
    request.flush({ data: dto });

    expect(created?.id).toBe('f-1');
  });

  it('deletes a filter by id', () => {
    service.delete('f-1').subscribe();
    const request = httpMock.expectOne('/api/v1/saved-filters/f-1');
    expect(request.request.method).toBe('DELETE');
    request.flush(null);
  });
});

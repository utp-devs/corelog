import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { searchHitLink, SearchApiService } from './search-api.service';
import { SearchQueryApiService } from './search-query-api.service';
import { VersionApiService } from './version-api.service';

describe('SearchQueryApiService', () => {
  let api: SearchQueryApiService;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    api = TestBed.inject(SearchQueryApiService);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('runs a query with its order and maps the page', () => {
    let total = -1;
    api
      .searchIssues({ jql: 'project = KAN', order: 'created DESC', page: 2, pageSize: 20 })
      .subscribe((page) => {
        total = page.total;
        expect(page.items[0].labels).toEqual(['bug']);
        expect(page.items[0].voteCount).toBe(3);
        expect(page.warnings[0].code).toBe('w1');
      });
    const request = http.expectOne((r) => r.url === '/api/v1/issues/search');
    expect(request.request.params.get('jql')).toBe('project = KAN');
    expect(request.request.params.get('order')).toBe('created DESC');
    expect(request.request.params.get('page')).toBe('2');
    request.flush({
      data: [
        {
          id: 'i1',
          project_id: 'p1',
          key: 'KAN-1',
          project_key: 'KAN',
          summary: 's',
          description: '',
          status: 'open',
          priority: { id: 'p', name: 'Alta', icon: '', color: '' },
          issue_type: { id: 't', name: 'Error', icon: '', color: '', hierarchy_level: 0 },
          parent_id: null,
          due_date: null,
          story_points: null,
          rank: 'a',
          closed_at: null,
          reporter_id: 'u',
          reporter_name: 'U',
          assignee_id: null,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
          labels: ['bug'],
          vote_count: 3,
        },
      ],
      meta: {
        page: 2,
        page_size: 20,
        total: 41,
        query: 'project = KAN',
        warnings: [{ code: 'w1', message: 'm' }],
      },
    });
    expect(total).toBe(41);
  });

  it('validates and suggests without moving the loading bar', () => {
    api
      .validate('statu = 1')
      .subscribe((result) => expect(result.errors[0].suggestions).toEqual(['status']));
    http.expectOne('/api/v1/search/validate').flush({
      data: {
        valid: false,
        query: '',
        errors: [
          {
            position: 0,
            length: 5,
            code: 'search.unknown_field',
            message: 'm',
            suggestions: ['status'],
            expected: [],
          },
        ],
      },
    });
    api
      .values('status', 'to', 5)
      .subscribe((result) => expect(result.values[0].value).toBe('todo'));
    const suggest = http.expectOne((r) => r.url === '/api/v1/search/suggest');
    expect(suggest.request.params.get('field')).toBe('status');
    suggest.flush({ data: { values: [{ value: 'todo', label: 'Por hacer', detail: '' }] } });
  });
});

describe('search results', () => {
  it('maps links of the kinds the app has no address for', () => {
    expect(searchHitLink({ kind: 'filter', id: 'f1', link: '/filters/f1' })).toBe(
      '/issues?filter=f1',
    );
    expect(searchHitLink({ kind: 'note', id: 'n1', link: '/notes/n1' })).toBe('/notes');
    expect(searchHitLink({ kind: 'issue', id: 'i1', link: '/issues/i1' })).toBe('/browse/i1');
    expect(searchHitLink({ kind: 'person', id: 'u1', link: '/people/u1' })).toBe('/people/u1');
  });

  it('asks for every kind and returns the counts', () => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    const http = TestBed.inject(HttpTestingController);
    let counts = {};
    TestBed.inject(SearchApiService)
      .searchAll('ana')
      .subscribe((result) => (counts = result.counts));
    const request = http.expectOne((r) => r.url === '/api/v1/search');
    expect(request.request.params.get('types')).toBe('all');
    request.flush({ data: [], meta: { counts: { person: 2 } } });
    expect(counts).toEqual({ person: 2 });
  });
});

describe('VersionApiService', () => {
  it('releases moving the unresolved issues and maps the result', () => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    const http = TestBed.inject(HttpTestingController);
    let moved = 0;
    TestBed.inject(VersionApiService)
      .release('v1', 'new', '2026-02-01')
      .subscribe((result) => (moved = result.movedIssues));
    const request = http.expectOne('/api/v1/versions/v1/release');
    expect(request.request.body).toEqual({ move_unresolved_to: 'new', release_date: '2026-02-01' });
    request.flush({
      data: {
        version: { id: 'v1', project_id: 'p', name: '1.0', status: 'released' },
        moved_issues: 3,
        unresolved_issues: 0,
        next_version: { id: 'v2', project_id: 'p', name: '1.1', status: 'unreleased' },
      },
    });
    expect(moved).toBe(3);
  });
});

import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { SearchHit } from '../../models/search.model';
import { SearchApiService } from './search-api.service';

describe('SearchApiService', () => {
  let service: SearchApiService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(SearchApiService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => httpMock.verify());

  it('sends the query and returns the hits', () => {
    let hits: SearchHit[] = [];
    service.search('exportar').subscribe((result) => (hits = result));

    const request = httpMock.expectOne((req) => req.url === '/api/v1/search');
    expect(request.request.params.get('q')).toBe('exportar');
    expect(request.request.params.get('limit')).toBe('10');
    request.flush({
      data: [
        { kind: 'issue', id: 'i-1', title: 'Exportar', subtitle: 'CORE', link: '/issues/i-1' },
      ],
    });

    expect(hits[0]).toMatchObject({ kind: 'issue', link: '/issues/i-1' });
  });
});

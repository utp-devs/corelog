import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { provideCorelogTesting } from '@corelog/core/testing';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { IssuesListPage } from './issues-list-page';

const emptyPage = { data: [], meta: { page: 1, page_size: 50, total: 0, query: '', warnings: [] } };

describe('IssuesListPage (navigator)', () => {
  let http: HttpTestingController;
  let harness: RouterTestingHarness;

  beforeEach(async () => {
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(),
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([{ path: 'issues', component: IssuesListPage }]),
      ],
    });
    http = TestBed.inject(HttpTestingController);
    harness = await RouterTestingHarness.create();
  });

  afterEach(() => {
    http.match(() => true);
  });

  /** Answers the requests every visit makes besides the search (saved filters, option lists). */
  function answerSideRequests(): void {
    for (const request of http.match((r) => r.url.endsWith('/filters'))) {
      if (!request.cancelled)
        request.flush({ data: [], meta: { page: 1, page_size: 100, total: 0 } });
    }
    for (const request of http.match((r) => r.url.endsWith('/search/suggest'))) {
      if (!request.cancelled) request.flush({ data: { completions: [], values: [] } });
    }
  }

  it('searches with the basic filters of the address and shows the chips', async () => {
    await harness.navigateByUrl('/issues?project=KAN,SCRUM&status=todo');
    answerSideRequests();
    const search = http.expectOne((r) => r.url.endsWith('/issues/search'));
    expect(search.request.params.get('jql')).toBe(
      'project IN ("KAN", "SCRUM") AND statusCategory = "todo"',
    );
    expect(search.request.params.get('order')).toBe('updated DESC');
    search.flush(emptyPage);
    harness.detectChanges();

    const chips = Array.from(
      (harness.routeNativeElement as HTMLElement).querySelectorAll('[data-testid="query-chip"]'),
    ).map((chip) => chip.textContent?.trim());
    expect(chips.some((text) => text?.includes('Proyecto: KAN'))).toBe(true);
    expect(chips.some((text) => text?.includes('Estado: todo'))).toBe(true);
  });

  it('runs the advanced query of the address with its own ORDER BY and validates it', async () => {
    await harness.navigateByUrl('/issues?jql=statu%20%3D%20Done');
    answerSideRequests();
    const search = http.expectOne((r) => r.url.endsWith('/issues/search'));
    expect(search.request.params.get('jql')).toBe('statu = Done');
    expect(search.request.params.has('order')).toBe(false);
    search.flush(
      { error: { code: 'search.unknown_field', message: "campo desconocido 'statu'" } },
      { status: 422, statusText: 'Unprocessable Entity' },
    );
    harness.detectChanges();
    expect(
      (harness.routeNativeElement as HTMLElement).querySelector('[data-testid="query-error"]')
        ?.textContent,
    ).toContain("campo desconocido 'statu'");
  });

  it('keeps the page and the columns in the address', async () => {
    await harness.navigateByUrl('/issues?page=3&cols=key,summary,reporter');
    answerSideRequests();
    const search = http.expectOne((r) => r.url.endsWith('/issues/search'));
    expect(search.request.params.get('page')).toBe('3');
    search.flush(emptyPage);
    expect(TestBed.inject(Router).url).toContain('cols=key,summary,reporter');
  });
});

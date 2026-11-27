import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { SessionService } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';

import { IssueNotesSection } from './issue-notes-section';

const BASE = '/api/v1/notes';
const ISSUE = '00000000-0000-4000-8000-000000000001';

function dto(id: string, extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id,
    title: '',
    body: 'Revisar la migración',
    color: 'yellow',
    pinned: false,
    tags: [],
    issue_id: ISSUE,
    project_id: null,
    created_at: '2026-11-01T10:00:00Z',
    updated_at: '2026-11-01T10:00:00Z',
    ...extra,
  };
}

function setup(notes: Record<string, unknown>[] = []) {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideRouter([]),
      provideHttpClient(),
      provideHttpClientTesting(),
    ],
  });
  // A signed-in session: the notes store clears itself when there is none.
  TestBed.inject(SessionService).startSession('token-1');
  const fixture = TestBed.createComponent(IssueNotesSection);
  fixture.componentRef.setInput('issueId', ISSUE);
  const backend = TestBed.inject(HttpTestingController);
  fixture.detectChanges();
  TestBed.tick();
  const list = backend.expectOne((r) => r.method === 'GET' && r.url === BASE);
  expect(list.request.params.get('issue')).toBe(ISSUE);
  list.flush({ data: notes, meta: { limit: 50, next_cursor: null } });
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, backend };
}

describe('IssueNotesSection', () => {
  it('explains that the notes are private and shows an empty message', () => {
    const { el } = setup();
    expect(el.textContent).toContain('Mis apuntes');
    expect(el.textContent).toContain('Solo tú puedes ver tus notas.');
    expect(el.textContent).toContain('Aún no tienes apuntes sobre esta incidencia.');
  });

  it('lists the notes of the issue with a link that opens each one', () => {
    const { el } = setup([dto('n1')]);
    const link = el.querySelector<HTMLAnchorElement>('.issue-notes__link');
    expect(link?.textContent).toContain('Revisar la migración');
    expect(link?.getAttribute('href')).toBe('/notes?note=n1');
  });

  it('creates a note linked to the issue from the field', async () => {
    const { fixture, el, backend } = setup();
    const button = () =>
      el.querySelector<HTMLButtonElement>('p-button button') as HTMLButtonElement;
    expect(button().disabled).toBe(true);

    const field = el.querySelector('textarea') as HTMLTextAreaElement;
    field.value = ' Preguntar a QA ';
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    button().click();

    const request = backend.expectOne((r) => r.method === 'POST' && r.url === BASE);
    expect(request.request.body).toEqual({ body: 'Preguntar a QA', issue_id: ISSUE });
    request.flush({ data: dto('n2', { body: 'Preguntar a QA' }) });
    fixture.detectChanges();
    expect(el.querySelectorAll('.issue-notes__item')).toHaveLength(1);
    await fixture.whenStable();
    expect(field.value).toBe('');
  });
});

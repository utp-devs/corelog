import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { SessionService } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';

import { NOTE_UNDO_WINDOW_MS } from '@corelog/core';
import { NotesPage } from './notes-page';

const BASE = '/api/v1/notes';

function dto(id: string, extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id,
    title: `Nota ${id}`,
    body: 'Texto',
    color: 'yellow',
    pinned: false,
    tags: [],
    issue_id: null,
    project_id: null,
    created_at: '2026-11-01T10:00:00Z',
    updated_at: '2026-11-01T10:00:00Z',
    ...extra,
  };
}

async function setup(
  notes: Record<string, unknown>[],
  tags: { tag: string; count: number }[] = [],
) {
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
  const fixture = TestBed.createComponent(NotesPage);
  const backend = TestBed.inject(HttpTestingController);
  fixture.detectChanges();
  TestBed.tick();
  backend
    .expectOne((r) => r.method === 'GET' && r.url === BASE)
    .flush({ data: notes, meta: { limit: 24, next_cursor: null } });
  backend.expectOne(`${BASE}/tags`).flush({ data: tags });
  fixture.detectChanges();
  await fixture.whenStable();
  return { fixture, el: fixture.nativeElement as HTMLElement, backend };
}

describe('NotesPage', () => {
  afterEach(() => {
    vi.useRealTimers();
    TestBed.resetTestingModule();
  });

  it('shows the examples and a call to action when the user has no notes', async () => {
    const { el } = await setup([]);
    expect(el.textContent).toContain('Aún no tienes notas');
    expect(el.textContent).toContain('Crear mi primera nota');
    expect(el.querySelectorAll('.notes-page__example')).toHaveLength(3);
  });

  it('puts pinned notes in their own section above the others', async () => {
    const { el } = await setup([dto('a', { pinned: true }), dto('b'), dto('c')]);
    expect(el.querySelectorAll('[data-testid="notes-pinned"] app-note-card')).toHaveLength(1);
    expect(el.querySelectorAll('[data-testid="notes-others"] app-note-card')).toHaveLength(2);
  });

  it('lists the tags with their counts and filters by the chosen one', async () => {
    const { fixture, el, backend } = await setup(
      [dto('a', { tags: ['trabajo'] })],
      [{ tag: 'trabajo', count: 3 }],
    );
    const buttons = Array.from(el.querySelectorAll<HTMLButtonElement>('.notes-page__nav-item'));
    expect(buttons.map((b) => (b.textContent ?? '').split(' ').join('').trim())).toEqual([
      'Todas',
      'Fijadas',
      'trabajo3',
    ]);

    buttons[2].click();
    fixture.detectChanges();
    TestBed.tick();
    const request = backend.expectOne((r) => r.url === BASE && r.params.get('tag') === 'trabajo');
    request.flush({
      data: [dto('a', { tags: ['trabajo'] })],
      meta: { limit: 24, next_cursor: null },
    });
    backend.expectOne(`${BASE}/tags`).flush({ data: [{ tag: 'trabajo', count: 3 }] });
    fixture.detectChanges();
    expect(el.querySelector('[aria-current="true"]')?.textContent).toContain('trabajo');
  });

  it('removes a deleted note at once and brings it back with Deshacer', async () => {
    vi.useFakeTimers();
    const { fixture, el, backend } = await setup([dto('a'), dto('b')]);
    const remove = el.querySelectorAll<HTMLButtonElement>('.note-card__action')[2];
    remove.click();
    fixture.detectChanges();

    expect(el.querySelectorAll('app-note-card')).toHaveLength(1);
    const bar = el.querySelector('.notes-page__undo') as HTMLElement;
    expect(bar.textContent).toContain('Nota eliminada');

    el.querySelector<HTMLButtonElement>('.notes-page__undo-action')?.click();
    fixture.detectChanges();
    expect(el.querySelectorAll('app-note-card')).toHaveLength(2);
    expect(el.querySelector('.notes-page__undo')).toBeNull();

    vi.advanceTimersByTime(NOTE_UNDO_WINDOW_MS * 2);
    backend.expectNone((r) => r.method === 'DELETE');
  });

  it('asks the server to delete after the undo window', async () => {
    vi.useFakeTimers();
    const { fixture, el, backend } = await setup([dto('a')]);
    el.querySelectorAll<HTMLButtonElement>('.note-card__action')[2].click();
    fixture.detectChanges();
    vi.advanceTimersByTime(NOTE_UNDO_WINDOW_MS);
    const request = backend.expectOne((r) => r.method === 'DELETE' && r.url === `${BASE}/a`);
    request.flush(null, { status: 204, statusText: 'No Content' });
  });

  it('shows the error state with a retry when the first load fails', async () => {
    TestBed.configureTestingModule({
      providers: [
        provideCorelogTesting(),
        provideRouter([]),
        provideHttpClient(),
        provideHttpClientTesting(),
      ],
    });
    TestBed.inject(SessionService).startSession('token-1');
    const fixture = TestBed.createComponent(NotesPage);
    const backend = TestBed.inject(HttpTestingController);
    fixture.detectChanges();
    TestBed.tick();
    backend
      .expectOne((r) => r.url === BASE)
      .flush({ error: { code: 'x', message: 'Sin servicio' } }, { status: 503, statusText: 'x' });
    backend.expectOne(`${BASE}/tags`).flush({ data: [] });
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelector('app-error-state')).not.toBeNull();
  });
});

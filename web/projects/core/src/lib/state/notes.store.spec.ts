import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { SessionService } from '../services/session.service';
import {
  NOTES_PAGE_SIZE,
  NOTE_AUTOSAVE_DELAY_MS,
  NOTE_UNDO_WINDOW_MS,
  NotesStore,
} from './notes.store';

const BASE = '/api/v1/notes';

function dto(id: string, extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id,
    title: `Nota ${id}`,
    body: '',
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

function setup() {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  const store = TestBed.inject(NotesStore);
  const backend = TestBed.inject(HttpTestingController);
  // A signed-in session: the store clears itself when there is none.
  const session = TestBed.inject(SessionService);
  session.startSession('token-1');
  TestBed.tick();
  return { store, backend, session };
}

function loadTwo(context: ReturnType<typeof setup>): void {
  context.store.load();
  context.backend
    .expectOne((r) => r.url === BASE && r.params.get('limit') === String(NOTES_PAGE_SIZE))
    .flush({ data: [dto('a'), dto('b')], meta: { limit: 24, next_cursor: null } });
  context.backend.expectOne(`${BASE}/tags`).flush({ data: [{ tag: 'idea', count: 2 }] });
}

describe('NotesStore', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    TestBed.resetTestingModule();
  });

  it('loads a view with its filters and the tag counts', () => {
    const { store, backend } = setup();
    store.load({ query: ' idea ', tag: 'work', pinned: true });
    const list = backend.expectOne((r) => r.url === BASE);
    expect(list.request.params.get('q')).toBe('idea');
    expect(list.request.params.get('tag')).toBe('work');
    expect(list.request.params.get('pinned')).toBe('true');
    list.flush({ data: [dto('a', { pinned: true })], meta: { limit: 24, next_cursor: 'c2' } });
    backend.expectOne(`${BASE}/tags`).flush({ data: [{ tag: 'work', count: 3 }] });

    expect(store.items().map((n) => n.id)).toEqual(['a']);
    expect(store.pinned()).toHaveLength(1);
    expect(store.hasMore()).toBe(true);
    expect(store.tags()).toEqual([{ tag: 'work', count: 3 }]);
    expect(store.isFiltered()).toBe(true);
  });

  it('keeps the error and clears the loading flag when the list fails', () => {
    const { store, backend } = setup();
    store.load();
    backend
      .expectOne((r) => r.url === BASE)
      .flush({ error: { code: 'x', message: 'Fallo' } }, { status: 500, statusText: 'Err' });
    backend.expectOne(`${BASE}/tags`).flush({ data: [] });
    expect(store.loading()).toBe(false);
    expect(store.error()?.message).toBe('Fallo');
  });

  it('appends the next page with the cursor and never repeats a note', () => {
    const context = setup();
    context.store.load();
    context.backend
      .expectOne((r) => r.url === BASE)
      .flush({ data: [dto('a')], meta: { limit: 24, next_cursor: 'c2' } });
    context.backend.expectOne(`${BASE}/tags`).flush({ data: [] });

    context.store.loadMore();
    const next = context.backend.expectOne(
      (r) => r.url === BASE && r.params.get('cursor') === 'c2',
    );
    next.flush({ data: [dto('a'), dto('b')], meta: { limit: 24, next_cursor: null } });

    expect(context.store.items().map((n) => n.id)).toEqual(['a', 'b']);
    expect(context.store.hasMore()).toBe(false);
  });

  it('pins optimistically and keeps the pinned note first', () => {
    const context = setup();
    loadTwo(context);
    const second = context.store.items()[1];

    context.store.togglePin(second);
    expect(context.store.items()[0].id).toBe('b');
    expect(context.store.items()[0].pinned).toBe(true);

    context.backend
      .expectOne(`${BASE}/b/pin`)
      .flush({ data: dto('b', { pinned: true, updated_at: '2026-11-02T10:00:00Z' }) });
    expect(context.store.items()[0].updatedAt).toBe('2026-11-02T10:00:00Z');
  });

  it('rolls the pin back when the server refuses', () => {
    const context = setup();
    loadTwo(context);
    context.store.togglePin(context.store.items()[0]);
    expect(context.store.items()[0].pinned).toBe(true);

    context.backend
      .expectOne(`${BASE}/a/pin`)
      .flush(
        { error: { code: 'note.not_found', message: 'No existe' } },
        { status: 404, statusText: 'NF' },
      );
    expect(context.store.items()[0].pinned).toBe(false);
    expect(context.store.error()?.code).toBe('note.not_found');
  });

  it('removes a note at once and deletes it on the server after the undo window', () => {
    const context = setup();
    loadTwo(context);
    context.store.remove(context.store.items()[0]);
    expect(context.store.items().map((n) => n.id)).toEqual(['b']);

    vi.advanceTimersByTime(NOTE_UNDO_WINDOW_MS - 1);
    context.backend.expectNone(`${BASE}/a`);
    vi.advanceTimersByTime(1);
    const request = context.backend.expectOne(`${BASE}/a`);
    expect(request.request.method).toBe('DELETE');
    request.flush(null, { status: 204, statusText: 'No Content' });
    context.backend.expectOne(`${BASE}/tags`).flush({ data: [] });
  });

  it('restores a removed note when the user undoes within the window', () => {
    const context = setup();
    loadTwo(context);
    const first = context.store.items()[0];
    context.store.remove(first);
    context.store.undoRemove(first.id);
    expect(context.store.items().map((n) => n.id)).toEqual(['a', 'b']);

    vi.advanceTimersByTime(NOTE_UNDO_WINDOW_MS * 2);
    context.backend.expectNone(`${BASE}/a`);
  });

  it('puts the note back when the delete fails', () => {
    const context = setup();
    loadTwo(context);
    context.store.remove(context.store.items()[0]);
    vi.advanceTimersByTime(NOTE_UNDO_WINDOW_MS);
    context.backend.expectOne(`${BASE}/a`).flush(null, { status: 500, statusText: 'Err' });
    expect(context.store.items().map((n) => n.id)).toContain('a');
    expect(context.store.error()).not.toBeNull();
  });

  it('merges quick edits into one request after the quiet period and reports the state', () => {
    const context = setup();
    loadTwo(context);
    context.store.autosave('a', { title: 'Uno' });
    vi.advanceTimersByTime(NOTE_AUTOSAVE_DELAY_MS - 100);
    context.store.autosave('a', { body: 'Texto' });
    expect(context.store.saveState()).toBe('saving');
    expect(context.store.items().find((n) => n.id === 'a')?.title).toBe('Uno');

    vi.advanceTimersByTime(NOTE_AUTOSAVE_DELAY_MS - 1);
    context.backend.expectNone(`${BASE}/a`);
    vi.advanceTimersByTime(1);
    const request = context.backend.expectOne(`${BASE}/a`);
    expect(request.request.method).toBe('PATCH');
    expect(request.request.body).toEqual({ title: 'Uno', body: 'Texto' });
    request.flush({ data: dto('a', { title: 'Uno', body: 'Texto' }) });
    expect(context.store.saveState()).toBe('saved');
  });

  it('flags a failed autosave and sends the edit again with the next change', () => {
    const context = setup();
    loadTwo(context);
    context.store.autosave('a', { title: 'Uno' });
    vi.advanceTimersByTime(NOTE_AUTOSAVE_DELAY_MS);
    context.backend.expectOne(`${BASE}/a`).flush(null, { status: 500, statusText: 'Err' });
    expect(context.store.saveState()).toBe('error');

    context.store.autosave('a', { body: 'x' });
    vi.advanceTimersByTime(NOTE_AUTOSAVE_DELAY_MS);
    const retry = context.backend.expectOne(`${BASE}/a`);
    expect(retry.request.body).toEqual({ title: 'Uno', body: 'x' });
    retry.flush({ data: dto('a') });
    expect(context.store.saveState()).toBe('saved');
  });

  it('flush sends pending edits without waiting', () => {
    const context = setup();
    loadTwo(context);
    context.store.autosave('b', { color: 'blue' });
    context.store.flush();
    context.backend.expectOne(`${BASE}/b`).flush({ data: dto('b', { color: 'blue' }) });
    expect(context.store.items().find((n) => n.id === 'b')?.color).toBe('blue');
  });

  it('adds a created note to the view only when it matches the tag filter', () => {
    const context = setup();
    context.store.load({ query: '', tag: 'work', pinned: false });
    context.backend
      .expectOne((r) => r.url === BASE)
      .flush({ data: [], meta: { limit: 24, next_cursor: null } });
    context.backend.expectOne(`${BASE}/tags`).flush({ data: [] });

    context.store.create({ title: 'Otra', tags: ['home'] }).subscribe();
    context.backend.expectOne(BASE).flush({ data: dto('n1', { tags: ['home'] }) });
    context.backend.expectOne(`${BASE}/tags`).flush({ data: [] });
    expect(context.store.items()).toHaveLength(0);
    expect(context.store.recent().map((n) => n.id)).toEqual(['n1']);

    context.store.create({ title: 'Trabajo', tags: ['work'] }).subscribe();
    context.backend.expectOne(BASE).flush({ data: dto('n2', { tags: ['work'] }) });
    context.backend.expectOne(`${BASE}/tags`).flush({ data: [] });
    expect(context.store.items().map((n) => n.id)).toEqual(['n2']);
  });

  it('fails a create with the API error and keeps it for the page', () => {
    const { store, backend } = setup();
    let failure: unknown;
    store.create({ title: 'x' }).subscribe({ error: (e: unknown) => (failure = e) });
    backend
      .expectOne(BASE)
      .flush(
        { error: { code: 'note.invalid', message: 'Datos' } },
        { status: 422, statusText: 'U' },
      );
    expect(failure).toMatchObject({ code: 'note.invalid' });
    expect(store.error()?.code).toBe('note.invalid');
  });

  it('lists the notes of an issue', () => {
    const { store, backend } = setup();
    store.loadForIssue('issue-1');
    const list = backend.expectOne((r) => r.url === BASE);
    expect(list.request.params.get('issue')).toBe('issue-1');
    list.flush({
      data: [dto('a', { issue_id: 'issue-1' })],
      meta: { limit: 50, next_cursor: null },
    });
    expect(store.issueNotes()).toHaveLength(1);

    store.create({ title: 'Nueva', issueId: 'issue-1' }).subscribe();
    backend.expectOne(BASE).flush({ data: dto('z', { issue_id: 'issue-1' }) });
    expect(store.issueNotes().map((n) => n.id)).toContain('z');
  });

  it('clears everything when the session ends', () => {
    const context = setup();
    loadTwo(context);
    expect(context.store.items()).toHaveLength(2);

    context.session.clear();
    TestBed.tick();
    expect(context.store.items()).toEqual([]);
    expect(context.store.tags()).toEqual([]);
    expect(context.store.loaded()).toBe(false);
  });
});

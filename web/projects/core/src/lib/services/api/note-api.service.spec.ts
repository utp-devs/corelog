import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { firstValueFrom } from 'rxjs';

import { provideCorelogTesting } from '../../../testing';
import { NoteApiService } from './note-api.service';
import { normalizeNoteTags } from '../../models/note.model';

const BASE = '/api/v1/notes';

const noteDto = {
  id: 'n1',
  title: 'Idea',
  body: 'Texto',
  color: 'blue',
  pinned: true,
  tags: ['work'],
  issue_id: 'i1',
  project_id: null,
  created_at: '2026-11-01T10:00:00Z',
  updated_at: '2026-11-02T10:00:00Z',
};

describe('NoteApiService', () => {
  let api: NoteApiService;
  let backend: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    api = TestBed.inject(NoteApiService);
    backend = TestBed.inject(HttpTestingController);
  });

  afterEach(() => backend.verify());

  it('lists with filters and maps the page', async () => {
    const result = firstValueFrom(
      api.list({
        query: ' hola ',
        tag: 'work',
        pinned: false,
        issueId: 'i1',
        limit: 5,
        cursor: 'c',
      }),
    );
    const request = backend.expectOne((r) => r.url === BASE);
    expect(request.request.params.get('q')).toBe('hola');
    expect(request.request.params.get('tag')).toBe('work');
    expect(request.request.params.get('pinned')).toBe('false');
    expect(request.request.params.get('issue')).toBe('i1');
    expect(request.request.params.get('cursor')).toBe('c');
    expect(request.request.params.get('limit')).toBe('5');
    request.flush({ data: [noteDto], meta: { limit: 5, next_cursor: null } });

    const page = await result;
    expect(page.nextCursor).toBeNull();
    expect(page.items[0]).toEqual({
      id: 'n1',
      title: 'Idea',
      body: 'Texto',
      color: 'blue',
      pinned: true,
      tags: ['work'],
      issueId: 'i1',
      projectId: null,
      createdAt: '2026-11-01T10:00:00Z',
      updatedAt: '2026-11-02T10:00:00Z',
    });
  });

  it('creates with snake case fields and only the ones given', async () => {
    const result = firstValueFrom(api.create({ title: 'T', issueId: 'i1', tags: ['a'] }));
    const request = backend.expectOne(BASE);
    expect(request.request.method).toBe('POST');
    expect(request.request.body).toEqual({ title: 'T', issue_id: 'i1', tags: ['a'] });
    request.flush({ data: noteDto });
    expect((await result).id).toBe('n1');
  });

  it('patches only the fields present and sends a null link to remove it', async () => {
    const result = firstValueFrom(api.update('n1', { body: 'x', issueId: null }));
    const request = backend.expectOne(`${BASE}/n1`);
    expect(request.request.method).toBe('PATCH');
    expect(request.request.body).toEqual({ body: 'x', issue_id: null });
    request.flush({ data: noteDto });
    await result;
  });

  it('pins, unpins, deletes and reads tags', async () => {
    const pinned = firstValueFrom(api.pin('n1'));
    backend.expectOne(`${BASE}/n1/pin`).flush({ data: noteDto });
    await pinned;

    const unpinned = firstValueFrom(api.unpin('n1'));
    backend.expectOne(`${BASE}/n1/unpin`).flush({ data: { ...noteDto, pinned: false } });
    expect((await unpinned).pinned).toBe(false);

    const removed = firstValueFrom(api.delete('n1'), { defaultValue: undefined });
    const del = backend.expectOne(`${BASE}/n1`);
    expect(del.request.method).toBe('DELETE');
    del.flush(null, { status: 204, statusText: 'No Content' });
    await removed;

    const tags = firstValueFrom(api.tags());
    backend.expectOne(`${BASE}/tags`).flush({ data: [{ tag: 'work', count: 2 }] });
    expect(await tags).toEqual([{ tag: 'work', count: 2 }]);
  });
});

describe('normalizeNoteTags', () => {
  it('trims, lowercases, drops # and repeated or empty tags', () => {
    expect(normalizeNoteTags([' Work ', '#work', '', 'IDEA', '  '])).toEqual(['work', 'idea']);
  });
});

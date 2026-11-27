import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { NOTE_AUTOSAVE_DELAY_MS, SessionService, type Note } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';

import { NoteEditor } from './note-editor';

const BASE = '/api/v1/notes';

const NOTE: Note = {
  id: 'n1',
  title: 'Reunión',
  body: 'Temas',
  color: 'yellow',
  pinned: false,
  tags: [],
  issueId: null,
  projectId: null,
  createdAt: '2026-11-01T10:00:00Z',
  updatedAt: '2026-11-01T10:00:00Z',
};

function dto(extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: 'n1',
    title: 'Reunión',
    body: 'Temas',
    color: 'yellow',
    pinned: false,
    tags: [],
    issue_id: null,
    project_id: null,
    created_at: NOTE.createdAt,
    updated_at: NOTE.updatedAt,
    ...extra,
  };
}

function setup() {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideRouter([]),
      provideHttpClient(),
      provideHttpClientTesting(),
    ],
  });
  TestBed.inject(SessionService).startSession('token-1');
  const fixture = TestBed.createComponent(NoteEditor);
  fixture.componentRef.setInput('note', NOTE);
  fixture.detectChanges();
  TestBed.tick();
  fixture.detectChanges();
  return {
    fixture,
    el: fixture.nativeElement as HTMLElement,
    backend: TestBed.inject(HttpTestingController),
  };
}

describe('NoteEditor', () => {
  beforeEach(() => vi.useFakeTimers());

  afterEach(() => {
    vi.useRealTimers();
    TestBed.resetTestingModule();
    document.body.style.removeProperty('overflow');
  });

  it('opens as a labelled panel with the note values and the privacy hint', () => {
    const { el } = setup();
    const dialog = el.querySelector('[role="dialog"]') as HTMLElement;
    expect(dialog.textContent).toContain('Solo tú puedes ver tus notas.');
    expect((el.querySelector('#note-editor-title') as HTMLInputElement).value).toBe('Reunión');
  });

  it('saves a change after the pause and shows the Guardado indicator', () => {
    const { fixture, el, backend } = setup();
    const title = el.querySelector('#note-editor-title') as HTMLInputElement;
    title.value = 'Reunión de equipo';
    title.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="note-save-state"]')?.textContent).toContain('Guardando');

    vi.advanceTimersByTime(NOTE_AUTOSAVE_DELAY_MS);
    const request = backend.expectOne((r) => r.method === 'PATCH' && r.url === `${BASE}/n1`);
    expect(request.request.body).toEqual({ title: 'Reunión de equipo' });
    request.flush({ data: dto({ title: 'Reunión de equipo' }) });
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="note-save-state"]')?.textContent).toContain('Guardado');
  });

  it('shows an error message when the save fails', () => {
    const { fixture, el, backend } = setup();
    const title = el.querySelector('#note-editor-title') as HTMLInputElement;
    title.value = 'X';
    title.dispatchEvent(new Event('input'));
    vi.advanceTimersByTime(NOTE_AUTOSAVE_DELAY_MS);
    backend.expectOne(`${BASE}/n1`).flush(null, { status: 500, statusText: 'Err' });
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="note-save-state"]')?.textContent).toContain(
      'No se pudo guardar',
    );
  });

  it('sends pending edits when the panel is closed with Listo and emits closed', () => {
    const { fixture, el, backend } = setup();
    const closed = vi.fn();
    fixture.componentInstance.closed.subscribe(closed);
    const title = el.querySelector('#note-editor-title') as HTMLInputElement;
    title.value = 'Rápido';
    title.dispatchEvent(new Event('input'));

    const buttons = Array.from(
      el.querySelectorAll<HTMLButtonElement>('.side-panel__footer button'),
    );
    buttons[buttons.length - 1].click();
    backend.expectOne((r) => r.method === 'PATCH').flush({ data: dto({ title: 'Rápido' }) });
    expect(closed).toHaveBeenCalledTimes(1);
  });

  it('asks the page to delete the note', () => {
    const { fixture, el } = setup();
    const deleted = vi.fn();
    fixture.componentInstance.deleted.subscribe(deleted);
    el.querySelector<HTMLButtonElement>('.side-panel__footer button')?.click();
    expect(deleted).toHaveBeenCalledWith(NOTE);
  });
});

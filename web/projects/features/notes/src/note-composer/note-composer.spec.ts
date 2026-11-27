import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { SessionService } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';

import { NoteComposer } from './note-composer';

const BASE = '/api/v1/notes';

function noteDto(extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: 'n1',
    title: '',
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
  // A signed-in session: the notes store clears itself when there is none.
  TestBed.inject(SessionService).startSession('token-1');
  const fixture = TestBed.createComponent(NoteComposer);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const backend = TestBed.inject(HttpTestingController);
  const expand = () => {
    el.querySelector<HTMLButtonElement>('.note-composer__collapsed')?.click();
    fixture.detectChanges();
  };
  const submit = () => {
    el.querySelector('form')?.dispatchEvent(new Event('submit'));
    fixture.detectChanges();
  };
  const typeTitle = (value: string) => {
    const field = el.querySelector<HTMLInputElement>('#note-composer-title') as HTMLInputElement;
    field.value = value;
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  };
  return { fixture, el, backend, expand, submit, typeTitle, component: fixture.componentInstance };
}

describe('NoteComposer', () => {
  it('starts collapsed and expands on click', () => {
    const { el, expand } = setup();
    expect(el.querySelector('form')).toBeNull();
    expand();
    expect(el.querySelector('form')).not.toBeNull();
    expect(el.textContent).toContain('Solo tú puedes ver tus notas');
  });

  it('asks for a title or text instead of sending an empty note', () => {
    const { el, backend, expand, submit } = setup();
    expand();
    submit();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'Escribe un título o el texto de la nota.',
    );
    backend.expectNone(BASE);
  });

  it('creates a note with the title, default color and no tags, then collapses', () => {
    const { fixture, el, backend, expand, submit, typeTitle } = setup();
    let created = 0;
    fixture.componentInstance.created.subscribe(() => created++);
    expand();
    typeTitle('  Llamar al cliente ');
    submit();

    const request = backend.expectOne((r) => r.method === 'POST' && r.url === BASE);
    expect(request.request.body).toEqual({
      title: 'Llamar al cliente',
      body: '',
      color: 'yellow',
      tags: [],
    });
    request.flush({ data: noteDto({ title: 'Llamar al cliente' }) });
    fixture.detectChanges();
    expect(created).toBe(1);
    expect(el.querySelector('form')).toBeNull();
  });

  it('keeps the form open when the server refuses the note', () => {
    const { fixture, el, backend, expand, submit, typeTitle } = setup();
    expand();
    typeTitle('Algo');
    submit();
    backend
      .expectOne(BASE)
      .flush(
        { error: { code: 'note.invalid', message: 'Datos' } },
        { status: 422, statusText: 'U' },
      );
    fixture.detectChanges();
    expect(el.querySelector('form')).not.toBeNull();
  });

  it('starts with the tag the page is filtered by', () => {
    const { fixture, el, expand } = setup();
    fixture.componentRef.setInput('defaultTag', 'trabajo');
    expand();
    expect(el.querySelector('.note-tags-input__chip-text')?.textContent).toContain('trabajo');
  });
});

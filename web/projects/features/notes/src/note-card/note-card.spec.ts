import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import type { Note } from '@corelog/core';

import { NoteCard } from './note-card';

const NOTE: Note = {
  id: 'n1',
  title: 'Reunión',
  body: 'Temas **importantes**',
  color: 'blue',
  pinned: false,
  tags: ['trabajo'],
  issueId: null,
  projectId: null,
  createdAt: '2026-11-01T10:00:00Z',
  updatedAt: '2026-11-01T10:00:00Z',
};

function setup(note: Note = NOTE) {
  TestBed.configureTestingModule({ providers: [provideRouter([])] });
  const fixture = TestBed.createComponent(NoteCard);
  fixture.componentRef.setInput('note', note);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, card: fixture.componentInstance };
}

describe('NoteCard', () => {
  it('shows title, rendered text, tags and the color of the note', () => {
    const { el } = setup();
    expect(el.querySelector('.note-card__title')?.textContent).toContain('Reunión');
    expect(el.querySelector('.note-card__body strong')?.textContent).toBe('importantes');
    expect(el.querySelector('.note-card__chip')?.textContent).toContain('trabajo');
    expect(el.querySelector('.note-card')?.getAttribute('data-color')).toBe('blue');
  });

  it('names every icon-only action', () => {
    const { el } = setup();
    const labels = Array.from(el.querySelectorAll('.note-card__action')).map((button) =>
      button.getAttribute('aria-label'),
    );
    expect(labels).toEqual(['Fijar la nota', 'Cambiar el color de la nota', 'Eliminar la nota']);
  });

  it('emits open, pin and delete', () => {
    const { el, card } = setup();
    const opened = vi.fn();
    const pinned = vi.fn();
    const deleted = vi.fn();
    card.openNote.subscribe(opened);
    card.togglePin.subscribe(pinned);
    card.deleteNote.subscribe(deleted);

    el.querySelector<HTMLButtonElement>('.note-card__open')?.click();
    const [pin, , remove] = Array.from(
      el.querySelectorAll<HTMLButtonElement>('.note-card__action'),
    );
    pin.click();
    remove.click();
    expect(opened).toHaveBeenCalledWith(NOTE);
    expect(pinned).toHaveBeenCalledWith(NOTE);
    expect(deleted).toHaveBeenCalledWith(NOTE);
  });

  it('reflects the pinned state for assistive technology', () => {
    const { el } = setup({ ...NOTE, pinned: true });
    const pin = el.querySelector('.note-card__action') as HTMLElement;
    expect(pin.getAttribute('aria-pressed')).toBe('true');
    expect(pin.getAttribute('aria-label')).toBe('Quitar la fijación de la nota');
  });

  it('labels a note without title or text', () => {
    const { el } = setup({ ...NOTE, title: '', body: '' });
    expect(el.textContent).toContain('Nota sin contenido');
  });
});

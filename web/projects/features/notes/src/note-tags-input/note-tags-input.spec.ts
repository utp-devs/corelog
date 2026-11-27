import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { NoteTagsInput } from './note-tags-input';

@Component({
  imports: [NoteTagsInput],
  template: '<app-note-tags-input [(tags)]="tags" [suggestions]="suggestions" />',
})
class Host {
  readonly tags = signal<readonly string[]>([]);
  suggestions = ['trabajo', 'casa'];
}

function setup() {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const field = () => el.querySelector('input') as HTMLInputElement;
  const type = (value: string) => {
    field().value = value;
    field().dispatchEvent(new Event('input'));
    fixture.detectChanges();
  };
  const enter = () => {
    field().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    fixture.detectChanges();
  };
  return { fixture, host: fixture.componentInstance, el, field, type, enter };
}

describe('NoteTagsInput', () => {
  it('adds a normalized tag with Enter and clears the field', () => {
    const { host, type, enter, field } = setup();
    type('  Trabajo ');
    enter();
    expect(host.tags()).toEqual(['trabajo']);
    expect(field().value).toBe('');
  });

  it('adds on comma and ignores repeated tags', () => {
    const { host, type } = setup();
    type('uno,');
    type('uno,');
    type('dos,');
    expect(host.tags()).toEqual(['uno', 'dos']);
  });

  it('rejects a tag longer than 30 characters with a message', () => {
    const { host, type, enter, el } = setup();
    type('a'.repeat(31));
    enter();
    expect(host.tags()).toEqual([]);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('30 caracteres');
  });

  it('stops at 10 tags and disables the field', () => {
    const { fixture, host, type, enter, field } = setup();
    for (let i = 0; i < 10; i++) {
      type(`t${i}`);
      enter();
    }
    expect(host.tags()).toHaveLength(10);
    expect(field().disabled).toBe(true);
    expect(field().placeholder).toContain('Máximo de etiquetas alcanzado');
    fixture.detectChanges();
  });

  it('removes a tag with its button and offers existing tags as quick picks', () => {
    const { fixture, host, el } = setup();
    el.querySelector<HTMLButtonElement>('.note-tags-input__pick')?.click();
    fixture.detectChanges();
    expect(host.tags()).toEqual(['trabajo']);

    el.querySelector<HTMLButtonElement>('.note-tags-input__remove')?.click();
    fixture.detectChanges();
    expect(host.tags()).toEqual([]);
  });
});

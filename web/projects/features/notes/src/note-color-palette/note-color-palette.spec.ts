import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import type { NoteColor } from '@corelog/core';

import { NoteColorPalette } from './note-color-palette';

@Component({
  imports: [NoteColorPalette],
  template: '<app-note-color-palette [(value)]="color" />',
})
class Host {
  readonly color = signal<NoteColor>('yellow');
}

function setup() {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const swatches = () => Array.from(el.querySelectorAll<HTMLButtonElement>('[role="radio"]'));
  return { fixture, host: fixture.componentInstance, swatches };
}

describe('NoteColorPalette', () => {
  it('offers the eight colors as a radio group with Spanish names', () => {
    const { swatches } = setup();
    expect(swatches().map((s) => s.getAttribute('aria-label'))).toEqual([
      'Amarillo',
      'Azul',
      'Verde',
      'Rosa',
      'Morado',
      'Naranja',
      'Rojo',
      'Gris',
    ]);
    expect(swatches()[0].getAttribute('aria-checked')).toBe('true');
  });

  it('selects a color on click and keeps one tab stop', () => {
    const { fixture, host, swatches } = setup();
    swatches()[3].click();
    fixture.detectChanges();
    expect(host.color()).toBe('pink');
    expect(swatches().map((s) => s.getAttribute('tabindex'))).toEqual([
      '-1',
      '-1',
      '-1',
      '0',
      '-1',
      '-1',
      '-1',
      '-1',
    ]);
  });

  it('moves with the arrow keys and wraps around', () => {
    const { fixture, host, swatches } = setup();
    swatches()[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    fixture.detectChanges();
    expect(host.color()).toBe('gray');
    swatches()[7].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    fixture.detectChanges();
    expect(host.color()).toBe('yellow');
  });
});

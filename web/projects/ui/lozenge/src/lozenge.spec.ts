import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { Lozenge, LozengeCategory, LozengeTone } from './lozenge';

@Component({
  imports: [Lozenge],
  template: `<app-lozenge [category]="category" [tone]="tone">{{ text }}</app-lozenge>`,
})
class LozengeHost {
  category: LozengeCategory | undefined = undefined;
  tone: LozengeTone = 'neutral';
  text = '';
}

function create(options: { text: string; category?: LozengeCategory; tone?: LozengeTone }): {
  el: HTMLElement;
  host: LozengeHost;
  detect: () => void;
} {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(LozengeHost);
  const host = fixture.componentInstance;
  host.text = options.text;
  host.category = options.category;
  if (options.tone !== undefined) {
    host.tone = options.tone;
  }
  fixture.detectChanges();
  return {
    el: fixture.nativeElement as HTMLElement,
    host,
    detect: () => fixture.detectChanges(),
  };
}

function lozengeOf(el: HTMLElement): HTMLElement | null {
  return el.querySelector('.lozenge');
}

describe('Lozenge', () => {
  it('always shows the text', () => {
    const { el } = create({ text: 'Hecho', category: 'done' });
    expect(lozengeOf(el)?.textContent?.trim()).toBe('Hecho');
  });

  it('uses the category when given and the tone otherwise', () => {
    const first = create({ text: 'En curso', category: 'progress', tone: 'danger' });
    expect(lozengeOf(first.el)?.getAttribute('data-variant')).toBe('progress');
    TestBed.resetTestingModule();
    const second = create({ text: 'Revisar', tone: 'warning' });
    expect(lozengeOf(second.el)?.getAttribute('data-variant')).toBe('warning');
  });

  it('defaults to the neutral tone', () => {
    const { el } = create({ text: 'Sin estado' });
    expect(lozengeOf(el)?.getAttribute('data-variant')).toBe('neutral');
  });

  it('renders hostile text as text', () => {
    const { el } = create({ text: '<img src=x onerror=alert(1)>', tone: 'info' });
    expect(el.querySelector('img')).toBeNull();
  });
});

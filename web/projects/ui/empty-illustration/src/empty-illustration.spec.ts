import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { EmptyIllustration, type EmptyIllustrationKind } from './empty-illustration';

@Component({
  imports: [EmptyIllustration],
  template: `<app-empty-illustration [kind]="kind()" data-testid="illustration" />`,
})
class Host {
  readonly kind = signal<EmptyIllustrationKind>('projects');
}

const kinds: EmptyIllustrationKind[] = ['projects', 'issues', 'search', 'notes', 'generic'];

function render(kind: EmptyIllustrationKind): HTMLElement {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.componentInstance.kind.set(kind);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('EmptyIllustration', () => {
  afterEach(() => TestBed.resetTestingModule());

  it.each(kinds)('renders the %s drawing as a decorative svg', (kind) => {
    const svg = render(kind).querySelector('svg');
    expect(svg).not.toBeNull();
    expect(svg?.getAttribute('aria-hidden')).toBe('true');
    expect(svg?.getAttribute('data-kind')).toBe(kind);
    expect(svg?.querySelectorAll('path, rect, circle').length).toBeGreaterThan(0);
  });

  it.each(kinds)('uses no hex or rgb literals in the %s drawing', (kind) => {
    const el = render(kind);
    expect(el.innerHTML).not.toMatch(/#[0-9a-fA-F]{3,6}\b|rgba?\(/);
  });

  it('puts the test id on the svg element', () => {
    expect(render('search').querySelector('svg')?.getAttribute('data-testid')).toBe('illustration');
  });
});

import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { CardGrid } from './card-grid';

@Component({
  imports: [CardGrid],
  template: `
    <app-card-grid [minCardWidth]="min()" [ariaLabel]="label()" [fill]="fill()">
      <article>Equipo A</article>
      <article>Equipo B</article>
    </app-card-grid>
  `,
})
class Host {
  readonly min = signal(280);
  readonly label = signal<string | undefined>(undefined);
  readonly fill = signal(false);
}

describe('CardGrid', () => {
  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [Host] });
  });

  it('marks itself as a card grid for the layout audit and renders every card', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('[data-card-grid]')).not.toBeNull();
    expect(fixture.nativeElement.textContent).toContain('Equipo A');
    expect(fixture.nativeElement.textContent).toContain('Equipo B');
  });

  it('defaults the minimum card width to 280 px and accepts an override', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    const grid = fixture.nativeElement.querySelector('[data-card-grid]') as HTMLElement;
    expect(grid.style.getPropertyValue('--cl-card-min')).toBe('280px');
    fixture.componentInstance.min.set(200);
    fixture.detectChanges();
    expect(grid.style.getPropertyValue('--cl-card-min')).toBe('200px');
  });

  it('names the grid as a group when ariaLabel is given', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.componentInstance.label.set('Proyectos');
    fixture.detectChanges();
    const group = fixture.nativeElement.querySelector('[role="group"]') as HTMLElement;
    expect(group.getAttribute('aria-label')).toBe('Proyectos');
  });

  it('wraps the grid in a filled panel with fill and keeps the group name on the panel', () => {
    const fixture = TestBed.createComponent(Host);
    fixture.componentInstance.fill.set(true);
    fixture.componentInstance.label.set('Proyectos');
    fixture.detectChanges();
    const panel = fixture.nativeElement.querySelector('[data-fill="true"]') as HTMLElement;
    expect(panel.getAttribute('role')).toBe('group');
    expect(panel.getAttribute('aria-label')).toBe('Proyectos');
    expect(panel.querySelector('[data-card-grid]')).not.toBeNull();
  });
});

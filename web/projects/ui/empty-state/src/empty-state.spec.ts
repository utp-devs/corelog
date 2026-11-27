import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EmptyState } from './empty-state';

@Component({
  imports: [EmptyState],
  template: `
    <app-empty-state [title]="title()" [description]="description()">
      @if (withAction()) {
        <button action type="button">Crear</button>
      }
    </app-empty-state>
  `,
})
class Host {
  readonly title = signal('Vacío');
  readonly description = signal<string | undefined>(undefined);
  readonly withAction = signal(false);
}

function create(): ComponentFixture<Host> {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return fixture;
}

describe('EmptyState', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('renders the title and the optional description', () => {
    const fixture = create();
    fixture.componentInstance.title.set('Aún no hay incidencias');
    fixture.componentInstance.description.set('Crea la primera para empezar');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.textContent).toContain('Aún no hay incidencias');
    expect(el.textContent).toContain('Crea la primera para empezar');
  });

  it('renders the action when provided', () => {
    const fixture = create();
    fixture.componentInstance.withAction.set(true);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(
      el.querySelector('[role="status"] .empty-state__action button')?.textContent?.trim(),
    ).toBe('Crear');
  });

  it('omits the description and action when not provided', () => {
    const fixture = create();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.empty-state__title')?.textContent).toBe('Vacío');
    expect(el.querySelector('.empty-state__description')).toBeNull();
    expect(el.querySelector('button')).toBeNull();
  });

  it('shows the state photograph instead of the icon tile when an image is given', () => {
    TestBed.configureTestingModule({ imports: [EmptyState] });
    const fixture = TestBed.createComponent(EmptyState);
    fixture.componentRef.setInput('title', 'Aún no hay proyectos');
    fixture.componentRef.setInput('image', 'empty-projects');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('img')?.getAttribute('src')).toBe('/images/states/empty-projects.webp');
    expect(el.querySelector('.empty-state__icon')).toBeNull();
  });
});

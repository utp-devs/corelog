import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { Breadcrumbs, BreadcrumbItem } from './breadcrumbs';

@Component({
  imports: [Breadcrumbs],
  template: `<app-breadcrumbs [items]="items()" />`,
})
class Host {
  readonly items = signal<BreadcrumbItem[]>([]);
}

function render(items: BreadcrumbItem[]): HTMLElement {
  TestBed.configureTestingModule({ imports: [Host], providers: [provideRouter([])] });
  const fixture = TestBed.createComponent(Host);
  fixture.componentInstance.items.set(items);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('Breadcrumbs', () => {
  it('renders a navigation landmark named "Ruta de navegación" with an ordered list', () => {
    const el = render([{ label: 'Proyectos', to: '/projects' }, { label: 'CoreLog' }]);
    const nav = el.querySelector('nav');
    expect(nav?.getAttribute('aria-label')).toBe('Ruta de navegación');
    expect(nav?.querySelectorAll('ol > li')).toHaveLength(2);
  });

  it('links every item with a route except the last one', () => {
    const el = render([
      { label: 'Inicio', to: '/' },
      { label: 'Proyectos', to: '/projects' },
      { label: 'CoreLog' },
    ]);
    const links = Array.from(el.querySelectorAll('a'));
    expect(links.map((a) => a.textContent?.trim())).toEqual(['Inicio', 'Proyectos']);
    expect(links[0].getAttribute('href')).toBe('/');
    expect(links[1].getAttribute('href')).toBe('/projects');
    expect(el.querySelectorAll('a')).toHaveLength(2);
  });

  it('marks the last item as the current page', () => {
    const el = render([{ label: 'Proyectos', to: '/projects' }, { label: 'CoreLog' }]);
    const current = el.querySelector('[aria-current="page"]');
    expect(current?.textContent?.trim()).toBe('CoreLog');
    expect(el.querySelector('a')?.hasAttribute('aria-current')).toBe(false);
  });

  it('renders an item without a route as plain text', () => {
    const el = render([{ label: 'Ajustes' }, { label: 'Seguridad' }]);
    const plain = el.querySelector('.breadcrumbs__plain');
    expect(plain?.textContent?.trim()).toBe('Ajustes');
    expect(plain?.tagName).not.toBe('A');
    expect(plain?.hasAttribute('href')).toBe(false);
  });

  it('renders nothing for an empty trail', () => {
    const el = render([]);
    expect(el.querySelector('nav')).toBeNull();
  });
});

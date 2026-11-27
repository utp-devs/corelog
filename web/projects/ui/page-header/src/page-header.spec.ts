import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { PageHeader, PageHeaderActions } from './page-header';

function create(inputs: { title: string; subtitle?: string }): HTMLElement {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(PageHeader);
  fixture.componentRef.setInput('title', inputs.title);
  if (inputs.subtitle !== undefined) {
    fixture.componentRef.setInput('subtitle', inputs.subtitle);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

@Component({
  imports: [PageHeader, PageHeaderActions],
  template: `
    <app-page-header title="Equipos">
      @if (withActions) {
        <button appPageHeaderActions type="button">Crear equipo</button>
      }
    </app-page-header>
  `,
})
class HostWithActions {
  withActions = false;
}

describe('PageHeader', () => {
  it('shows the title as the page heading', () => {
    const el = create({ title: 'Mis proyectos' });
    expect(el.querySelector('h1')?.textContent).toBe('Mis proyectos');
    expect(el.querySelector('.page-header__subtitle')).toBeNull();
  });

  it('shows the subtitle when one is given', () => {
    const el = create({ title: 'Equipos', subtitle: 'Grupos de trabajo' });
    expect(el.querySelector('.page-header__subtitle')?.textContent).toContain('Grupos de trabajo');
  });

  it('projects actions into the actions area when provided', () => {
    TestBed.configureTestingModule({ imports: [HostWithActions] });
    const fixture = TestBed.createComponent(HostWithActions);
    fixture.componentInstance.withActions = true;
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.page-header__actions button')?.textContent?.trim()).toBe(
      'Crear equipo',
    );
  });

  it('renders no actions area when no actions are projected', () => {
    TestBed.configureTestingModule({ imports: [HostWithActions] });
    const fixture = TestBed.createComponent(HostWithActions);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('.page-header__actions')).toBeNull();
    expect(el.querySelector('h1')?.textContent).toBe('Equipos');
  });

  it('draws no banner image by default', () => {
    const el = create({ title: 'Equipos' });
    expect(el.querySelector('img')).toBeNull();
    expect(el.querySelector('.page-header--banner')).toBeNull();
  });

  it('draws the section banner as a decorative image behind the title', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(PageHeader);
    fixture.componentRef.setInput('title', 'Proyectos');
    fixture.componentRef.setInput('banner', 'projects');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const img = el.querySelector('img');
    expect(img?.getAttribute('src')).toBe('/images/banners/banner-projects.webp');
    expect(img?.getAttribute('alt')).toBe('');
    expect(img?.getAttribute('width')).toBe('1800');
    expect(el.querySelector('.page-header--banner')).not.toBeNull();
    expect(el.querySelector('h1')?.textContent).toBe('Proyectos');
  });
});

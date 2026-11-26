import { TestBed } from '@angular/core/testing';

import { PageHeader } from './page-header';

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

describe('PageHeader', () => {
  it('shows the title as the page heading', () => {
    const el = create({ title: 'Mis proyectos' });
    expect(el.querySelector('h1')?.textContent).toBe('Mis proyectos');
    expect(el.querySelector('.page-subtitle')).toBeNull();
  });

  it('shows the subtitle when one is given', () => {
    const el = create({ title: 'Equipos', subtitle: 'Grupos de trabajo' });
    expect(el.querySelector('.page-subtitle')?.textContent).toContain('Grupos de trabajo');
  });
});

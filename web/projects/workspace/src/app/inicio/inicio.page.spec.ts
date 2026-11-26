import { TestBed } from '@angular/core/testing';

import { InicioPage } from './inicio.page';

describe('InicioPage', () => {
  it('shows the workspace home heading', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(InicioPage);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelector('h1')?.textContent).toBe('Inicio');
  });
});

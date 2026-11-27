import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { NotFound } from './not-found';

describe('NotFound', () => {
  it('shows the not-found heading with a way back home', () => {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(NotFound);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('h1')?.textContent).toBe('Página no encontrada');
    expect(root.querySelector('a')?.textContent).toContain('Volver al inicio');
  });
});

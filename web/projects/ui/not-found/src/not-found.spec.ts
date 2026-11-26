import { TestBed } from '@angular/core/testing';

import { NotFound } from './not-found';

describe('NotFound', () => {
  it('shows the not-found heading', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(NotFound);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelector('h1')?.textContent).toBe(
      'Página no encontrada',
    );
  });
});

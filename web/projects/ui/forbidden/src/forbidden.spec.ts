import { TestBed } from '@angular/core/testing';

import { Forbidden } from './forbidden';

describe('Forbidden', () => {
  it('explains that the user has no permission for the page', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(Forbidden);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('h1')?.textContent).toBe('Acceso denegado');
    expect(el.querySelector('[role="alert"]')).not.toBeNull();
  });
});

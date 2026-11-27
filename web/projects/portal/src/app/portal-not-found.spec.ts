import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { PortalNotFound } from './portal-not-found';

describe('PortalNotFound', () => {
  it('shows the brand, the shared 404 page and a way back home', () => {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(PortalNotFound);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('img[src*="/brand/corelog-banner"]')).not.toBeNull();
    expect(el.querySelector('h1')?.textContent).toContain('Página no encontrada');
    const home = Array.from(el.querySelectorAll('a')).filter((a) => a.getAttribute('href') === '/');
    expect(home.length).toBeGreaterThanOrEqual(2);
  });
});

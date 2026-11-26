import { TestBed } from '@angular/core/testing';

import { AUTH_ROUTES } from '../auth.routes';
import { LoginPage } from './login-page';

describe('AUTH_ROUTES', () => {
  it('lazy-loads the login placeholder at the empty path', async () => {
    const route = AUTH_ROUTES[0];
    expect(route.path).toBe('');
    expect(route.pathMatch).toBe('full');
    const component = await route.loadComponent?.();
    expect(component).toBe(LoginPage);
  });

  it('renders the placeholder message', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(LoginPage);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('próximamente');
  });
});

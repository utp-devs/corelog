import { routes } from './app.routes';

describe('portal routes', () => {
  it('redirects the root path to the login page', () => {
    const root = routes.find((route) => route.path === '');
    expect(root?.redirectTo).toBe('login');
    expect(root?.pathMatch).toBe('full');
  });

  it('lazy-loads the auth feature at /login', () => {
    const login = routes.find((route) => route.path === 'login');
    expect(login?.loadChildren).toBeDefined();
  });
});

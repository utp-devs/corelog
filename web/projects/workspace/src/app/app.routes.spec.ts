import { routes } from './app.routes';

describe('workspace routes', () => {
  it('protects the shell with the auth guard', () => {
    const shell = routes.find((route) => route.path === '');
    expect(shell?.canActivate?.length).toBe(1);
    expect(shell?.canActivateChild?.length).toBe(1);
  });

  it('redirects the shell root to inicio', () => {
    const shell = routes.find((route) => route.path === '');
    expect(shell?.children?.find((child) => child.path === '')?.redirectTo).toBe('inicio');
  });
});

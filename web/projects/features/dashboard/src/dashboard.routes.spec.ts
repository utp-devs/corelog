import { DASHBOARD_ROUTES } from './dashboard.routes';

describe('DASHBOARD_ROUTES', () => {
  it('serves the dashboard page at the mount point with a Spanish title', () => {
    expect(DASHBOARD_ROUTES.map((route) => route.path)).toEqual(['']);
    expect(DASHBOARD_ROUTES[0]?.title).toBe('Paneles');
  });

  it('lazy-loads the page component', async () => {
    const component = await DASHBOARD_ROUTES[0]?.loadComponent?.();
    expect(component).toBeDefined();
  });
});

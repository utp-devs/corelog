import { DASHBOARDS_ROUTES, DASHBOARD_ROUTES } from './dashboard.routes';

describe('dashboard routes', () => {
  it('serves the personal dashboard at the mount point', async () => {
    expect(DASHBOARD_ROUTES.map((route) => route.path)).toEqual(['']);
    expect(DASHBOARD_ROUTES[0]?.title).toBe('Mi resumen');
    expect(await DASHBOARD_ROUTES[0]?.loadComponent?.()).toBeDefined();
  });

  it('serves the manager and a single dashboard, both guarded against lost edits when editable', async () => {
    expect(DASHBOARDS_ROUTES.map((route) => route.path)).toEqual(['', ':id']);
    expect(DASHBOARDS_ROUTES[1]?.canDeactivate?.length).toBe(1);
    expect(await DASHBOARDS_ROUTES[0]?.loadComponent?.()).toBeDefined();
    expect(await DASHBOARDS_ROUTES[1]?.loadComponent?.()).toBeDefined();
  });
});

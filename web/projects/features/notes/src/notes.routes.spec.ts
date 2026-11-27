import { NOTES_ROUTES } from './notes.routes';

describe('NOTES_ROUTES', () => {
  it('serves the notes page at the mount point with a Spanish title', () => {
    expect(NOTES_ROUTES.map((route) => route.path)).toEqual(['']);
    expect(NOTES_ROUTES[0]?.title).toBe('Mis apuntes');
  });

  it('lazy-loads the page component', async () => {
    const component = await NOTES_ROUTES[0]?.loadComponent?.();
    expect(component).toBeDefined();
  });
});

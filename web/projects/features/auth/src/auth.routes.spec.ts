import { AUTH_ROUTES } from './auth.routes';

describe('AUTH_ROUTES', () => {
  it('declares sign-in, registration and invitation routes with Spanish titles', () => {
    expect(AUTH_ROUTES.map((route) => route.path)).toEqual([
      'login',
      'register',
      'forgot-password',
      'reset-password',
      'verify-email',
      'invite/:token',
    ]);
    expect(AUTH_ROUTES.map((route) => route.title)).toEqual([
      'Iniciar sesión',
      'Crear cuenta',
      'Recuperar contraseña',
      'Nueva contraseña',
      'Verificar correo',
      'Invitación',
    ]);
  });

  it('lazy-loads each page component', async () => {
    for (const route of AUTH_ROUTES) {
      expect(route.loadComponent).toBeDefined();
      const component = await route.loadComponent?.();
      expect(component).toBeDefined();
    }
  });
});

import type { HeroSlide } from '@corelog/ui/hero-carousel';

/** Which access screen the hero belongs to; each one has its own photographs and copy. */
export type AuthHeroVariant = 'login' | 'register' | 'invite' | 'security';

/** Headline in two lines with one highlighted word: `before` + `accent` + `after`. */
export interface AuthHeroCopy {
  readonly lineOne: string;
  readonly before: string;
  readonly accent: string;
  readonly after: string;
  readonly subtitle: string;
  readonly label: string;
  readonly slides: readonly HeroSlide[];
}

const IMAGES = '/images/auth';

export const AUTH_HERO: Readonly<Record<AuthHeroVariant, AuthHeroCopy>> = {
  login: {
    lineOne: 'Todo el trabajo de tu equipo,',
    before: '',
    accent: 'claro',
    after: ' y a tiempo.',
    subtitle: 'Organiza incidencias, tableros y sprints en un solo lugar.',
    label: 'Equipos trabajando con CoreLog',
    slides: [
      {
        src: `${IMAGES}/auth-login-hero-1.webp`,
        caption: 'Planifica con tu equipo',
        position: '70% 50%',
      },
      { src: `${IMAGES}/auth-login-hero-2.webp`, caption: 'Prioriza lo importante' },
      { src: `${IMAGES}/auth-login-hero-3.webp`, caption: 'Sigue el avance sin reuniones de más' },
      { src: `${IMAGES}/auth-login-hero-4.webp`, caption: 'Entrega a tiempo, todos juntos' },
    ],
  },
  register: {
    lineOne: 'Empieza con tu equipo',
    before: 'en ',
    accent: 'minutos',
    after: '.',
    subtitle: 'Crea tu cuenta y reúne proyectos, tableros y sprints en un solo lugar.',
    label: 'Equipo reunido alrededor de un proyecto',
    slides: [
      {
        src: `${IMAGES}/auth-register-hero.webp`,
        caption: 'Tu espacio de trabajo, listo en minutos',
      },
    ],
  },
  invite: {
    lineOne: 'Tu equipo ya',
    before: 'te ',
    accent: 'espera',
    after: '.',
    subtitle: 'Acepta la invitación y súmate al trabajo de tu organización.',
    label: 'Equipo que da la bienvenida',
    slides: [{ src: `${IMAGES}/auth-invite-hero.webp`, caption: 'Bienvenido a bordo' }],
  },
  security: {
    lineOne: 'Tu cuenta,',
    before: '',
    accent: 'protegida',
    after: '.',
    subtitle: 'Controlamos quién entra y qué puede hacer cada persona.',
    label: 'Seguridad de la cuenta',
    slides: [{ src: `${IMAGES}/auth-security-hero.webp`, caption: 'Acceso seguro' }],
  },
};

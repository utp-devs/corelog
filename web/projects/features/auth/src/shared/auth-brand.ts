/**
 * Logo shown on the access screens. Every screen reads it from here, so replacing the artwork means
 * changing this one constant. The current artwork has dark lettering, so the layout draws it on a light pill.
 */
export interface AuthLogo {
  readonly src: string;
  readonly srcset: string;
  readonly sizes: string;
  readonly alt: string;
  readonly width: number;
  readonly height: number;
}

export const AUTH_LOGO: AuthLogo = {
  src: '/brand/corelog-banner-480.png',
  srcset: '/brand/corelog-banner-480.png 480w, /brand/corelog-banner-960.png 960w',
  sizes: '140px',
  alt: 'CoreLog',
  width: 140,
  height: 37,
};

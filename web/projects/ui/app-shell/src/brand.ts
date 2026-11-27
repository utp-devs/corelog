/** Logo of the product and the place it links to. Apps can replace it through the `brand` input. */
export interface ShellBrand {
  /** Route of the logo link. */
  readonly homePath: string;
  readonly ariaLabel: string;
  readonly iconSrc: string;
  readonly iconSrcset: string;
  readonly bannerSrc: string;
  readonly bannerSrcset: string;
}

export const DEFAULT_SHELL_BRAND: ShellBrand = {
  homePath: '/issues',
  ariaLabel: 'CoreLog, ir a incidencias',
  iconSrc: '/brand/corelog-icon-64.png',
  iconSrcset: '/brand/corelog-icon-64.png 64w, /brand/corelog-icon-128.png 128w',
  bannerSrc: '/brand/corelog-banner-480.png',
  bannerSrcset: '/brand/corelog-banner-480.png 480w, /brand/corelog-banner-960.png 960w',
};

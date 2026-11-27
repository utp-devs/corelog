import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { RouterLink } from '@angular/router';

let nextLayoutId = 0;

/** Id of the main landmark; the skip link points here. */
export const AUTH_MAIN_ID = 'auth-main';

/** Value point shown in the brand panel. */
interface BrandPoint {
  readonly icon: string;
  readonly text: string;
}

const BRAND_POINTS: readonly BrandPoint[] = [
  { icon: 'pi-check-circle', text: 'Organiza incidencias, tableros y sprints en un solo lugar.' },
  { icon: 'pi-users', text: 'Trabaja con tu equipo y comparte avances con claridad.' },
  { icon: 'pi-shield', text: 'Controla quién ve y edita cada proyecto.' },
];

/**
 * Shared template for the public auth screens: a brand panel with the value proposition beside the form
 * card. Below the desktop breakpoint the brand panel collapses to a compact header. The optional switch
 * link ("no account? create one") is rendered under the form.
 */
@Component({
  selector: 'app-auth-layout',
  imports: [RouterLink],
  templateUrl: './auth-layout.html',
  styleUrl: './auth-layout.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuthLayout {
  readonly title = input.required<string>();
  readonly subtitle = input<string | null>(null);
  /** Text before the switch link, for example "¿No tienes cuenta?". */
  readonly switchText = input<string | null>(null);
  readonly switchLabel = input<string | null>(null);
  readonly switchPath = input<string | null>(null);
  readonly switchQuery = input<Readonly<Record<string, string>>>({});

  protected readonly mainId = AUTH_MAIN_ID;
  protected readonly titleId = `auth-title-${nextLayoutId++}`;
  protected readonly points = BRAND_POINTS;
}

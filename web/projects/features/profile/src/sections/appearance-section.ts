import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { ThemeMode, ThemeService, UiDensity, UiPreferencesService } from '@corelog/core';
import { Section } from '@corelog/ui/section';

interface ThemeOption {
  readonly value: ThemeMode;
  readonly label: string;
  readonly hint: string;
}

interface DensityOption {
  readonly value: UiDensity;
  readonly label: string;
  readonly hint: string;
}

const THEME_OPTIONS: readonly ThemeOption[] = [
  { value: 'light', label: 'Claro', hint: 'Fondo claro, el más habitual' },
  { value: 'dark', label: 'Oscuro', hint: 'Fondo oscuro, más cómodo con poca luz' },
  { value: 'system', label: 'Sistema', hint: 'Sigue el tema de tu dispositivo' },
];

const DENSITY_OPTIONS: readonly DensityOption[] = [
  { value: 'comfortable', label: 'Cómoda', hint: 'Filas con más aire entre sí' },
  { value: 'compact', label: 'Compacta', hint: 'Más filas visibles en las tablas' },
];

/** Theme (light, dark or follow the system) and table density. Both are saved in this browser. */
@Component({
  selector: 'app-profile-appearance-section',
  imports: [Section],
  templateUrl: './appearance-section.html',
  styleUrl: './appearance-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfileAppearanceSection {
  protected readonly theme = inject(ThemeService);
  protected readonly preferences = inject(UiPreferencesService);
  protected readonly themeOptions = THEME_OPTIONS;
  protected readonly densityOptions = DENSITY_OPTIONS;
}

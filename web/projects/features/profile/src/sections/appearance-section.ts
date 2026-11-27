import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ThemeMode, ThemeService } from '@corelog/core';
import { ToneTag } from '@corelog/ui/tone-tag';
import { Select } from 'primeng/select';

const THEME_OPTIONS: readonly { value: ThemeMode; label: string }[] = [
  { value: 'light', label: 'Claro' },
  { value: 'dark', label: 'Oscuro' },
];

const LANGUAGE_OPTIONS = [{ value: 'es', label: 'Español' }];

/** Theme (saved in this browser) and language (Spanish only for now). */
@Component({
  selector: 'app-profile-appearance-section',
  imports: [FormsModule, Select, ToneTag],
  templateUrl: './appearance-section.html',
  styleUrl: './appearance-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfileAppearanceSection {
  protected readonly theme = inject(ThemeService);
  protected readonly themeOptions = THEME_OPTIONS;
  protected readonly languageOptions = LANGUAGE_OPTIONS;
}

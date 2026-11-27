import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { ThemeService } from '@corelog/core';

/**
 * Floating round button that switches the app between the light and dark theme. It works on the shared theme
 * service, so the choice is stored and applies to every app and after sign-in. The label names the action.
 */
@Component({
  selector: 'app-theme-toggle-fab',
  templateUrl: './theme-toggle-fab.html',
  styleUrl: './theme-toggle-fab.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ThemeToggleFab {
  private readonly theme = inject(ThemeService);

  protected readonly isDark = this.theme.isDark;
  protected readonly label = computed(() =>
    this.theme.isDark() ? 'Cambiar a tema claro' : 'Cambiar a tema oscuro',
  );

  protected toggle(): void {
    this.theme.toggle();
  }
}

import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import { NotificationApiService } from '@corelog/core';
import { Section } from '@corelog/ui/section';
import { ToggleSwitch } from 'primeng/toggleswitch';

import { messageOr } from '../shared/feature-error';

/** In-app notification preference. */
@Component({
  selector: 'app-profile-notifications-section',
  imports: [FormsModule, Section, ToggleSwitch],
  templateUrl: './notifications-section.html',
  styleUrl: './notifications-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfileNotificationsSection {
  private readonly preferences = inject(NotificationApiService);
  private readonly destroyRef = inject(DestroyRef);

  /** Null while the preference is not loaded (or not available): the switch stays disabled. */
  protected readonly inAppEnabled = signal<boolean | null>(null);
  /** True until the preferences answer; the switch shows a spinner and stays disabled meanwhile. */
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);
  protected readonly status = signal<string | null>(null);

  constructor() {
    this.preferences
      .getPreferences()
      .pipe(takeUntilDestroyed())
      .subscribe({
        next: (prefs) => {
          this.inAppEnabled.set(prefs.inApp);
          this.loading.set(false);
        },
        error: () => this.loading.set(false),
      });
  }

  protected toggleInApp(checked: boolean): void {
    const previous = this.inAppEnabled();
    this.inAppEnabled.set(checked);
    this.error.set(null);
    this.preferences
      .updatePreferences(checked)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (saved) => {
          this.inAppEnabled.set(saved.inApp);
          this.status.set(
            saved.inApp
              ? 'Avisos en la aplicación activados'
              : 'Avisos en la aplicación desactivados',
          );
        },
        error: (failure: unknown) => {
          this.inAppEnabled.set(previous);
          this.error.set(messageOr(failure, 'No se pudo guardar la preferencia'));
        },
      });
  }
}

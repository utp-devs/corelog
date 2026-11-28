import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormsModule } from '@angular/forms';
import {
  NotificationApiService,
  type EmailNotificationMode,
  type NotificationPreferencesChange,
} from '@corelog/core';
import { Section } from '@corelog/ui/section';
import { SelectField, type SelectFieldOption } from '@corelog/ui/select-field';
import { ToggleSwitch } from 'primeng/toggleswitch';

import { messageOr } from '../shared/feature-error';

const EMAIL_MODES: readonly SelectFieldOption[] = [
  { value: 'immediate', label: 'Inmediato' },
  { value: 'daily', label: 'Resumen diario' },
  { value: 'off', label: 'Apagado' },
];

/** In-app and email notification preferences. */
@Component({
  selector: 'app-profile-notifications-section',
  imports: [FormsModule, Section, SelectField, ToggleSwitch],
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
  protected readonly emailModes = EMAIL_MODES;
  protected readonly emailMode = signal<EmailNotificationMode>('immediate');
  protected readonly notifyOwnActions = signal(false);

  constructor() {
    this.preferences
      .getPreferences()
      .pipe(takeUntilDestroyed())
      .subscribe({
        next: (prefs) => {
          this.inAppEnabled.set(prefs.inApp);
          this.emailMode.set(prefs.email ?? 'immediate');
          this.notifyOwnActions.set(prefs.notifyOwnActions ?? false);
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

  protected changeEmailMode(mode: string | null): void {
    if (mode !== 'immediate' && mode !== 'daily' && mode !== 'off') return;
    const previous = this.emailMode();
    this.emailMode.set(mode);
    this.save({ email: mode }, 'Correo actualizado', () => this.emailMode.set(previous));
  }

  protected toggleOwnActions(checked: boolean): void {
    const previous = this.notifyOwnActions();
    this.notifyOwnActions.set(checked);
    this.save(
      { notifyOwnActions: checked },
      checked
        ? 'Recibirás avisos de tus propias acciones'
        : 'No recibirás avisos de tus propias acciones',
      () => this.notifyOwnActions.set(previous),
    );
  }

  /** The in-app flag is required by the API, so every change sends the current value with it. */
  private save(change: NotificationPreferencesChange, message: string, restore: () => void): void {
    this.error.set(null);
    this.preferences
      .updatePreferences(this.inAppEnabled() ?? true, change)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => this.status.set(message),
        error: (failure: unknown) => {
          restore();
          this.error.set(messageOr(failure, 'No se pudo guardar la preferencia'));
        },
      });
  }
}

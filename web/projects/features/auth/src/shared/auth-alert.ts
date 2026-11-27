import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

export type AuthAlertTone = 'error' | 'warning' | 'info';

const ICONS: Readonly<Record<AuthAlertTone, string>> = {
  error: 'pi-exclamation-circle',
  warning: 'pi-exclamation-triangle',
  info: 'pi-info-circle',
};

/**
 * Banner of the auth screens. Errors and warnings are announced at once (role alert); informational
 * banners are announced politely (role status). The content is projected.
 */
@Component({
  selector: 'app-auth-alert',
  templateUrl: './auth-alert.html',
  styleUrl: './auth-alert.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuthAlert {
  readonly tone = input<AuthAlertTone>('error');

  protected readonly role = computed(() => (this.tone() === 'info' ? 'status' : 'alert'));
  protected readonly icon = computed(() => `pi ${ICONS[this.tone()]} auth-alert__icon`);
}

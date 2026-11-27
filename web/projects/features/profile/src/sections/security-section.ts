import { DatePipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import {
  AccountSecurityApiService,
  AccountSession,
  AuthStore,
  ChangePasswordPayload,
  describeUserAgent,
  LOCATION_ASSIGN,
  LOGIN_URL,
  NotificationService,
} from '@corelog/core';
import { ConfirmModal } from '@corelog/ui/confirm-modal';
import { Dialog } from '@corelog/ui/dialog';
import { FieldControl, FieldRow } from '@corelog/ui/field-row';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PasswordInput } from '@corelog/ui/password-input';
import { Section, SectionActions } from '@corelog/ui/section';
import { ToneTag } from '@corelog/ui/tone-tag';
import { Button } from 'primeng/button';
import { firstValueFrom } from 'rxjs';

import { messageOr } from '../shared/feature-error';
import { ProfilePasswordForm } from './password-form';
import { ProfileTwoFactorSection } from './two-factor-section';

const SESSIONS_PAGE_SIZE = 5;
const MOBILE_PATTERN = /Android|iPhone|iPad|Mobile/i;

/** Password, sessions (first five, then all on request) and account deactivation. */
@Component({
  selector: 'app-profile-security-section',
  imports: [
    Button,
    ConfirmModal,
    DatePipe,
    Dialog,
    FieldRow,
    LoadingSkeleton,
    PasswordInput,
    ProfilePasswordForm,
    ProfileTwoFactorSection,
    ReactiveFormsModule,
    Section,
    SectionActions,
    ToneTag,
  ],
  templateUrl: './security-section.html',
  styleUrl: './security-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfileSecuritySection {
  private readonly auth = inject(AuthStore);
  private readonly accountSecurity = inject(AccountSecurityApiService);
  private readonly notifications = inject(NotificationService);
  private readonly assignLocation = inject(LOCATION_ASSIGN);
  private readonly loginUrl = inject(LOGIN_URL);

  protected readonly describeAgent = describeUserAgent;

  protected isMobile(userAgent: string): boolean {
    return MOBILE_PATTERN.test(userAgent);
  }

  protected readonly savingPassword = signal(false);
  protected readonly sessions = signal<AccountSession[] | null>(null);
  protected readonly sessionsError = signal<string | null>(null);
  protected readonly savingSessions = signal(false);
  protected readonly confirmOthersOpen = signal(false);
  protected readonly showAll = signal(false);
  protected readonly visibleSessions = computed(() => {
    const all = this.sessions() ?? [];
    return this.showAll() ? all : all.slice(0, SESSIONS_PAGE_SIZE);
  });
  protected readonly hiddenCount = computed(
    () => (this.sessions()?.length ?? 0) - this.visibleSessions().length,
  );
  protected readonly moreLabel = computed(() => {
    const count = this.hiddenCount();
    return `Mostrar ${count} ${count === 1 ? 'sesión más' : 'sesiones más'}`;
  });
  protected readonly hasOtherSessions = computed(() =>
    (this.sessions() ?? []).some((session) => !session.current),
  );

  protected readonly deactivateOpen = signal(false);
  protected readonly deactivating = signal(false);
  protected readonly deactivatePassword = new FormControl('', { nonNullable: true });
  private readonly typedPassword = toSignal(this.deactivatePassword.valueChanges, {
    initialValue: '',
  });
  protected readonly canDeactivate = computed(
    () => this.typedPassword().length > 0 && !this.deactivating(),
  );

  constructor() {
    void this.loadSessions();
  }

  /** Saves the password; failures are reported here and rethrown so the form keeps its values. */
  protected readonly changePassword = async (payload: ChangePasswordPayload): Promise<void> => {
    this.savingPassword.set(true);
    try {
      await this.auth.changePassword(payload);
      this.notifications.success('Contraseña cambiada');
    } catch (error) {
      this.notifications.error(messageOr(error, 'No se pudo cambiar la contraseña'));
      throw error;
    } finally {
      this.savingPassword.set(false);
    }
  };

  protected async revokeSession(id: string): Promise<void> {
    this.savingSessions.set(true);
    try {
      await firstValueFrom(this.accountSecurity.revokeSession(id));
      this.notifications.success('Sesión cerrada');
      await this.loadSessions();
    } catch (error) {
      this.notifications.error(messageOr(error, 'No se pudo cerrar la sesión'));
    } finally {
      this.savingSessions.set(false);
    }
  }

  protected async revokeOtherSessions(): Promise<void> {
    this.confirmOthersOpen.set(false);
    this.savingSessions.set(true);
    try {
      await firstValueFrom(this.accountSecurity.revokeOtherSessions());
      this.notifications.success('Las demás sesiones se cerraron');
      await this.loadSessions();
    } catch (error) {
      this.notifications.error(messageOr(error, 'No se pudieron cerrar las sesiones'));
    } finally {
      this.savingSessions.set(false);
    }
  }

  protected openDeactivate(): void {
    this.deactivateOpen.set(true);
  }

  protected closeDeactivate(): void {
    this.deactivateOpen.set(false);
    this.deactivatePassword.reset('');
  }

  protected async deactivate(): Promise<void> {
    this.deactivating.set(true);
    try {
      await firstValueFrom(this.accountSecurity.deactivateAccount(this.deactivatePassword.value));
      this.auth.logout();
      this.notifications.success('Tu cuenta fue desactivada');
      // The login page belongs to another application: a full-page move, not a router navigation.
      this.assignLocation(this.loginUrl);
    } catch (error) {
      this.notifications.error(messageOr(error, 'No se pudo desactivar la cuenta'));
    } finally {
      this.deactivating.set(false);
    }
  }

  private async loadSessions(): Promise<void> {
    try {
      this.sessions.set(await firstValueFrom(this.accountSecurity.listSessions()));
      this.sessionsError.set(null);
    } catch (error) {
      this.sessionsError.set(messageOr(error, 'No se pudieron cargar las sesiones'));
    }
  }
}

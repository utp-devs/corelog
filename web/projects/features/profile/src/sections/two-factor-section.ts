import { DatePipe } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import {
  NotificationService,
  TwoFactorStore,
  asApiError,
  type TwoFactorEnrollment,
} from '@corelog/core';
import { Dialog } from '@corelog/ui/dialog';
import { FieldRow } from '@corelog/ui/field-row';
import { PasswordInput } from '@corelog/ui/password-input';
import { Section } from '@corelog/ui/section';
import { ToneTag } from '@corelog/ui/tone-tag';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';

import { qrModules, qrPath } from './qr-code';

const QUIET_ZONE = 2;
const CODE_INVALID = 'El código no es válido o ya se usó. Revísalo e inténtalo de nuevo.';
const PASSWORD_INVALID = 'La contraseña no es correcta.';

type Dialogs = 'enroll' | 'codes' | 'disable' | 'regenerate' | null;

/** Message of a failed security call, with the wording of the codes this section knows. */
function failureText(error: unknown): string {
  const api = asApiError(error);
  if (api.code === 'auth.two_factor_invalid') return CODE_INVALID;
  if (api.code === 'auth.invalid_credentials') return PASSWORD_INVALID;
  return api.message;
}

/**
 * Two-step verification of the account: turn it on with an authenticator app (QR and secret), save the
 * recovery codes, turn it off with the password and a code, and regenerate the codes. The recovery codes live
 * in memory only while their dialog is open.
 */
@Component({
  selector: 'app-profile-two-factor-section',
  imports: [
    Button,
    DatePipe,
    Dialog,
    FieldRow,
    InputText,
    PasswordInput,
    ReactiveFormsModule,
    Section,
    ToneTag,
  ],
  templateUrl: './two-factor-section.html',
  styleUrl: './two-factor-section.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfileTwoFactorSection {
  private readonly store = inject(TwoFactorStore);
  private readonly notifications = inject(NotificationService);

  protected readonly status = this.store.status.value;
  protected readonly statusLoading = this.store.status.loading;
  protected readonly statusError = computed(() => this.store.status.error()?.message ?? null);
  protected readonly busy = this.store.busy;
  protected readonly recoveryCodes = this.store.recoveryCodes;

  protected readonly dialog = signal<Dialogs>(null);
  protected readonly enrollment = signal<TwoFactorEnrollment | null>(null);
  protected readonly error = signal<string | null>(null);
  protected readonly useRecovery = signal(false);
  protected readonly saved = signal(false);
  protected readonly copied = signal(false);

  protected readonly code = new FormControl('', { nonNullable: true });
  protected readonly password = new FormControl('', { nonNullable: true });

  protected readonly qrSize = computed(() => {
    const current = this.enrollment();
    return current ? qrModules(current.otpauthUri).length + QUIET_ZONE * 2 : 0;
  });
  protected readonly qrPathData = computed(() => {
    const current = this.enrollment();
    return current ? qrPath(qrModules(current.otpauthUri), QUIET_ZONE) : '';
  });
  /** The secret in groups of four so it is easy to type by hand. */
  protected readonly groupedSecret = computed(() =>
    (this.enrollment()?.secret ?? '').replace(/(.{4})/g, '$1 ').trim(),
  );

  constructor() {
    this.store.load();
    // Recovery codes show in their own dialog as soon as the store has them.
    effect(() => {
      if (this.recoveryCodes() !== null) {
        this.saved.set(false);
        this.copied.set(false);
        this.dialog.set('codes');
      }
    });
  }

  protected async startEnrollment(): Promise<void> {
    this.resetForm();
    this.dialog.set('enroll');
    try {
      this.enrollment.set(await this.store.beginEnrollment());
    } catch (error) {
      this.error.set(failureText(error));
    }
  }

  protected async confirmEnrollment(): Promise<void> {
    if (this.code.value.trim() === '') return;
    this.error.set(null);
    try {
      await this.store.confirmEnrollment(this.code.value.trim());
      this.enrollment.set(null);
      this.notifications.success('Verificación en dos pasos activada');
    } catch (error) {
      this.error.set(failureText(error));
      this.code.reset('');
    }
  }

  protected closeEnrollment(): void {
    this.store.cancelEnrollment();
    this.enrollment.set(null);
    this.close();
  }

  protected openRegenerate(): void {
    this.resetForm();
    this.dialog.set('regenerate');
  }

  protected async regenerate(): Promise<void> {
    if (this.code.value.trim() === '') return;
    this.error.set(null);
    try {
      await this.store.regenerateRecoveryCodes(this.code.value.trim());
    } catch (error) {
      this.error.set(failureText(error));
      this.code.reset('');
    }
  }

  protected openDisable(): void {
    this.resetForm();
    this.dialog.set('disable');
  }

  protected toggleRecovery(): void {
    this.useRecovery.update((value) => !value);
    this.code.reset('');
    this.error.set(null);
  }

  protected async disable(): Promise<void> {
    const proof = this.code.value.trim();
    if (this.password.value === '' || proof === '') return;
    this.error.set(null);
    try {
      await this.store.disable({
        password: this.password.value,
        ...(this.useRecovery() ? { recoveryCode: proof } : { code: proof }),
      });
      this.notifications.success('Verificación en dos pasos desactivada');
      this.close();
    } catch (error) {
      this.error.set(failureText(error));
      this.code.reset('');
    }
  }

  protected async copyCodes(): Promise<void> {
    const codes = this.recoveryCodes();
    if (!codes) return;
    try {
      await navigator.clipboard.writeText(codes.join('\n'));
      this.copied.set(true);
      this.saved.set(true);
    } catch {
      this.notifications.error('No se pudo copiar. Descarga el archivo o anótalos a mano.');
    }
  }

  protected async copySecret(): Promise<void> {
    const secret = this.enrollment()?.secret;
    if (!secret) return;
    try {
      await navigator.clipboard.writeText(secret);
      this.notifications.success('Clave copiada');
    } catch {
      this.notifications.error('No se pudo copiar la clave');
    }
  }

  protected downloadCodes(): void {
    const codes = this.recoveryCodes();
    if (!codes) return;
    const text = `Códigos de recuperación de CoreLog\nCada código sirve una sola vez.\n\n${codes.join('\n')}\n`;
    const url = URL.createObjectURL(new Blob([text], { type: 'text/plain;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = 'codigos-de-recuperacion.txt';
    link.click();
    URL.revokeObjectURL(url);
    this.saved.set(true);
  }

  /** Closes the codes dialog and forgets the codes. */
  protected finishCodes(): void {
    this.store.clearRecoveryCodes();
    this.close();
  }

  protected close(): void {
    this.dialog.set(null);
    this.resetForm();
  }

  private resetForm(): void {
    this.code.reset('');
    this.password.reset('');
    this.error.set(null);
    this.useRecovery.set(false);
  }
}

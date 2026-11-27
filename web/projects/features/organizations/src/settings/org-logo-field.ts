import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  booleanAttribute,
  inject,
  input,
  signal,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Button } from 'primeng/button';
import { LOGO_MIME_TYPES, OrganizationAdminStore, validateLogoFile } from '@corelog/core';
import { NotificationService } from '@corelog/core';
import { firstValueFrom } from 'rxjs';

import { adminErrorMessage } from '../shared/admin-error-message';

/** Organization logo: preview, upload with client validation (PNG, JPEG, WebP, at most 2 MB) and removal. */
@Component({
  selector: 'app-org-logo-field',
  imports: [Button],
  templateUrl: './org-logo-field.html',
  styleUrl: './org-logo-field.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OrgLogoField {
  private readonly admin = inject(OrganizationAdminStore);
  private readonly notify = inject(NotificationService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly fileInput = viewChild.required<ElementRef<HTMLInputElement>>('fileInput');

  readonly organizationId = input.required<string>();
  readonly organizationName = input.required<string>();
  readonly canEdit = input(false, { transform: booleanAttribute });

  protected readonly logoUrl = this.admin.logoUrl;
  protected readonly busy = signal(false);
  protected readonly clientError = signal<string | null>(null);
  protected readonly accept = LOGO_MIME_TYPES.join(',');

  protected chooseFile(): void {
    this.fileInput().nativeElement.click();
  }

  protected onFileChosen(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;
    const problem = validateLogoFile(file);
    this.clientError.set(problem);
    if (problem) return;

    this.busy.set(true);
    this.admin
      .uploadLogo(this.organizationId(), file)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.busy.set(false);
          this.notify.success('Logo actualizado');
        },
        error: (error: unknown) => {
          this.busy.set(false);
          this.notify.error(adminErrorMessage(error, 'No se pudo subir el logo.'));
        },
      });
  }

  protected removeLogo(): void {
    this.busy.set(true);
    firstValueFrom(this.admin.removeLogo(this.organizationId()))
      .then(() => {
        this.notify.success('Logo eliminado');
      })
      .catch((error: unknown) => {
        this.notify.error(adminErrorMessage(error, 'No se pudo quitar el logo.'));
      })
      .finally(() => {
        this.busy.set(false);
      });
  }
}

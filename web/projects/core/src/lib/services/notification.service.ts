import { Injectable, inject } from '@angular/core';
import { MessageService } from 'primeng/api';

const SUCCESS_LIFE_MS = 3000;
const INFO_LIFE_MS = 3000;
const ERROR_LIFE_MS = 6000;

/** Short feedback toasts. The `<p-toast>` outlet is rendered by the app root. */
@Injectable({ providedIn: 'root' })
export class NotificationService {
  private readonly messages = inject(MessageService);

  success(detail: string): void {
    this.messages.add({ severity: 'success', summary: 'Listo', detail, life: SUCCESS_LIFE_MS });
  }

  info(detail: string): void {
    this.messages.add({ severity: 'info', summary: 'Información', detail, life: INFO_LIFE_MS });
  }

  error(detail: string): void {
    this.messages.add({ severity: 'error', summary: 'Error', detail, life: ERROR_LIFE_MS });
  }
}

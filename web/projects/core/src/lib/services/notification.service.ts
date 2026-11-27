import { HttpErrorResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { MessageService, ToastMessageOptions } from 'primeng/api';

import { ApiError } from '../models';
import { asApiError, isApiError } from '../utils/api-error';

/** Auto-close times. Errors stay longer because they need to be read and acted on. */
export const TOAST_LIFE_MS = 5000;
export const TOAST_ERROR_LIFE_MS = 8000;
/** Toasts on screen at once. When a new one arrives the oldest is dropped. */
export const TOAST_MAX_VISIBLE = 3;

interface ToastCopy {
  readonly summary: string;
  readonly detail: string;
}

const FORBIDDEN: ToastCopy = {
  summary: 'Sin permiso',
  detail: 'No tienes permiso para hacer esto. Pide acceso a quien administra el espacio.',
};
const NOT_FOUND: ToastCopy = {
  summary: 'No encontrado',
  detail: 'No encontramos lo que buscas. Puede que se haya movido o eliminado.',
};
const NETWORK: ToastCopy = {
  summary: 'Sin conexión',
  detail: 'No se pudo conectar con el servidor. Revisa tu conexión e inténtalo de nuevo.',
};
const VALIDATION: ToastCopy = {
  summary: 'Revisa los datos',
  detail: 'Algunos datos no son válidos. Revísalos e inténtalo de nuevo.',
};
const ARCHIVED: ToastCopy = {
  summary: 'Proyecto archivado',
  detail: 'Este proyecto está archivado y es de solo lectura. Restáuralo para hacer cambios.',
};
const RATE_LIMIT: ToastCopy = {
  summary: 'Demasiadas solicitudes',
  detail: 'Hiciste demasiadas solicitudes seguidas. Espera un momento e inténtalo de nuevo.',
};

/** Words that only appear in Spanish text: a message with one of them is shown as it is. */
const SPANISH_HINT =
  /[áéíóúñ¿¡]|\b(el|la|los|las|de|del|no|se|un|una|que|por|para|con|tu|tus|puede)\b/i;

/** English wording of the common API failures, mapped to their Spanish copy. */
const ENGLISH_PATTERNS: readonly (readonly [RegExp, ToastCopy])[] = [
  [/forbidden|permission|not allowed|unauthori[sz]ed access/i, FORBIDDEN],
  [/not found|does not exist|no such/i, NOT_FOUND],
  [/network|failed to fetch|unreachable|timed? ?out|offline/i, NETWORK],
  [/too many requests|rate.?limit|throttl/i, RATE_LIMIT],
  [/invalid|validation|required|must be|malformed/i, VALIDATION],
];

/** Copy of the issue failures that deserve their own sentence. */
const ISSUE_CODE_COPY: Readonly<Record<string, ToastCopy>> = {
  'issue.summary_required': {
    summary: 'Falta el resumen',
    detail: 'Escribe un resumen para la incidencia.',
  },
  'issue.reporter_required': {
    summary: 'Falta quien reporta',
    detail: 'Indica quién reporta la incidencia.',
  },
  'issue.reporter_not_found': {
    summary: 'Persona no encontrada',
    detail: 'No encontramos a la persona que reporta la incidencia.',
  },
  'issue.key_ambiguous': {
    summary: 'Clave repetida',
    detail:
      'Esa clave existe en varias organizaciones. Abre la incidencia desde la organización que corresponde.',
  },
  'issue.invalid_id': {
    summary: 'Incidencia no válida',
    detail: 'El identificador no es una clave ni un código de incidencia válido.',
  },
};

function copyForStatus(status: number): ToastCopy | null {
  if (status === 0) return NETWORK;
  if (status === 403) return FORBIDDEN;
  if (status === 404) return NOT_FOUND;
  if (status === 429) return RATE_LIMIT;
  return null;
}

function copyForCode(code: string): ToastCopy | null {
  if (code === 'project.archived') return ARCHIVED;
  const issueCopy = ISSUE_CODE_COPY[code];
  if (issueCopy) return issueCopy;
  if (/forbidden|permission|not_allowed/i.test(code)) return FORBIDDEN;
  if (/not_found/i.test(code)) return NOT_FOUND;
  if (/^network\./i.test(code)) return NETWORK;
  if (/rate_limit|too_many/i.test(code)) return RATE_LIMIT;
  if (/validation|invalid/i.test(code)) return VALIDATION;
  return null;
}

/**
 * The single place that turns an error (an ApiError, an HTTP error, or the text of one) into the copy of
 * an error toast. Known failures get a short title and a Spanish sentence; English text from the server
 * is translated when it matches a common failure; Spanish messages are kept as they are.
 */
export function toastCopyOf(source: unknown, fallback = 'Ocurrió un error inesperado.'): ToastCopy {
  if (typeof source === 'string') return copyForText(source, fallback);
  if (isApiError(source) || source instanceof HttpErrorResponse) {
    const error: ApiError = asApiError(source);
    const known = copyForCode(error.code) ?? copyForStatus(error.status);
    if (known) return known;
    return { summary: 'Error', detail: error.message ? asSentence(error.message) : fallback };
  }
  if (source instanceof Error && source.message) return copyForText(source.message, fallback);
  return { summary: 'Error', detail: fallback };
}

/** Short title for a Spanish message that already reads well: the text itself says what failed. */
function summaryOfSpanish(message: string): string {
  if (/permiso/i.test(message)) return FORBIDDEN.summary;
  if (/no existe|no se encontr|no encontrad/i.test(message)) return NOT_FOUND.summary;
  if (/conectar con el servidor|conexión/i.test(message)) return NETWORK.summary;
  if (/demasiadas solicitudes|demasiadas peticiones/i.test(message)) return RATE_LIMIT.summary;
  return 'Error';
}

/** A server message that starts in lower case or has no final stop reads as a sentence. */
function asSentence(message: string): string {
  const capitalized = message.charAt(0).toLocaleUpperCase('es') + message.slice(1);
  return /[.!?…]$/.test(capitalized) ? capitalized : capitalized + '.';
}

function copyForText(text: string, fallback: string): ToastCopy {
  const message = text.trim();
  if (!message) return { summary: 'Error', detail: fallback };
  if (SPANISH_HINT.test(message)) {
    return { summary: summaryOfSpanish(message), detail: asSentence(message) };
  }
  const match = ENGLISH_PATTERNS.find(([pattern]) => pattern.test(message));
  return match ? match[1] : { summary: 'Error', detail: message };
}

interface ShownToast {
  readonly signature: string;
  readonly message: ToastMessageOptions;
  readonly expiresAt: number;
}

/**
 * Short feedback toasts. The toast outlet (`<app-toast-host>`) is rendered by the app root.
 * Identical messages that are still on screen are shown once, at most TOAST_MAX_VISIBLE are visible
 * (the oldest goes first), and errors are translated through `toastCopyOf`.
 */
@Injectable({ providedIn: 'root' })
export class NotificationService {
  private readonly messages = inject(MessageService);
  private shown: ShownToast[] = [];

  success(detail: string): void {
    this.show({ severity: 'success', summary: 'Listo', detail, life: TOAST_LIFE_MS });
  }

  info(detail: string): void {
    this.show({ severity: 'info', summary: 'Aviso', detail, life: TOAST_LIFE_MS });
  }

  /** Accepts the message text or the error itself; both go through the same Spanish mapper. */
  error(source: unknown): void {
    const { summary, detail } = toastCopyOf(source);
    this.show({ severity: 'error', summary, detail, life: TOAST_ERROR_LIFE_MS });
  }

  /** Called by the toast outlet when a toast leaves the screen, so its slot is free again. */
  released(message: ToastMessageOptions): void {
    const signature = `${message.severity}|${message.summary}|${message.detail}`;
    this.shown = this.shown.filter(
      (entry) => entry.message !== message && entry.signature !== signature,
    );
  }

  private show(message: ToastMessageOptions): void {
    const now = Date.now();
    this.shown = this.shown.filter((entry) => entry.expiresAt > now);

    const signature = `${message.severity}|${message.summary}|${message.detail}`;
    if (this.shown.some((entry) => entry.signature === signature)) return;

    if (this.shown.length >= TOAST_MAX_VISIBLE) {
      // PrimeNG cannot remove one toast: clear the stack and show again the ones that stay.
      const keep = this.shown.slice(this.shown.length - (TOAST_MAX_VISIBLE - 1));
      this.messages.clear();
      this.shown = [];
      keep.forEach((entry) => this.push(entry.signature, entry.message, entry.expiresAt - now));
    }
    this.push(signature, message, message.life ?? TOAST_LIFE_MS);
  }

  private push(signature: string, message: ToastMessageOptions, life: number): void {
    const toast: ToastMessageOptions = { ...message, life: Math.max(life, TOAST_LIFE_MS / 5) };
    this.shown.push({ signature, message: toast, expiresAt: Date.now() + (toast.life ?? 0) });
    this.messages.add(toast);
  }
}

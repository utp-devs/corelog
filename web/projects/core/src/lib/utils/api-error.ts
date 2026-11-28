import { HttpErrorResponse } from '@angular/common/http';
import { TimeoutError } from 'rxjs';

import { ApiError, ApiErrorDetail, ApiMappingRequirement, ErrorResponse } from '../models';

const MESSAGES_BY_STATUS: Readonly<Record<number, string>> = {
  400: 'La solicitud contiene datos inválidos.',
  401: 'Tu sesión ha expirado o no es válida.',
  403: 'No tienes permisos para realizar esta operación.',
  404: 'El recurso solicitado no existe.',
  409: 'La operación entra en conflicto con el estado actual de los datos.',
  500: 'Ocurrió un error interno en el servidor.',
  502: 'El servidor no está disponible en este momento.',
  503: 'El servidor no está disponible en este momento.',
  504: 'El servidor tardó demasiado en responder.',
};

const MESSAGE_NETWORK =
  'No se pudo conectar con el servidor. Verifica tu conexión e inténtalo nuevamente.';
const MESSAGE_TIMEOUT = 'El servidor tardó demasiado en responder. Inténtalo nuevamente.';
const MESSAGE_GENERIC = 'Ocurrió un error inesperado.';

export function defaultMessageForStatus(status: number): string {
  return MESSAGES_BY_STATUS[status] ?? MESSAGE_GENERIC;
}

export function isApiError(value: unknown): value is ApiError {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const candidate = value as ApiError;
  return (
    typeof candidate.status === 'number' &&
    typeof candidate.code === 'string' &&
    typeof candidate.message === 'string' &&
    Array.isArray(candidate.details)
  );
}

function isErrorResponse(body: unknown): body is ErrorResponse {
  if (typeof body !== 'object' || body === null || !('error' in body)) {
    return false;
  }
  const inner = (body as ErrorResponse).error;
  return typeof inner === 'object' && inner !== null && typeof inner.message === 'string';
}

function fieldDetails(details: unknown): readonly ApiErrorDetail[] {
  if (!Array.isArray(details)) {
    return [];
  }
  const result: ApiErrorDetail[] = [];
  for (const item of details as unknown[]) {
    if (typeof item !== 'object' || item === null) continue;
    const entry = item as { field?: unknown; path?: unknown; message?: unknown; code?: unknown };
    const field = typeof entry.field === 'string' ? entry.field : entry.path;
    if (typeof field !== 'string' || typeof entry.message !== 'string') continue;
    result.push(
      typeof entry.code === 'string'
        ? { field, message: entry.message, code: entry.code }
        : { field, message: entry.message },
    );
  }
  return result;
}

function mappingRequirements(value: unknown): ApiMappingRequirement[] | undefined {
  if (!Array.isArray(value)) return undefined;
  const result: ApiMappingRequirement[] = [];
  for (const item of value as unknown[]) {
    const entry = item as { status_id?: unknown; name?: unknown; issue_count?: unknown };
    if (typeof entry?.status_id === 'string') {
      result.push({
        statusId: entry.status_id,
        name: typeof entry.name === 'string' ? entry.name : '',
        issueCount: typeof entry.issue_count === 'number' ? entry.issue_count : 0,
      });
    }
  }
  return result.length > 0 ? result : undefined;
}

/** Seconds of the Retry-After header when it is a plain number, or undefined. */
function retryAfterOf(error: HttpErrorResponse): { retryAfterSeconds?: number } {
  const raw = error.headers?.get('Retry-After');
  const seconds = raw === null || raw === undefined ? NaN : Number(raw);
  return Number.isFinite(seconds) && seconds >= 0 ? { retryAfterSeconds: Math.ceil(seconds) } : {};
}

function fromHttpError(error: HttpErrorResponse): ApiError {
  if (error.status === 0) {
    return { status: 0, code: 'network.unreachable', message: MESSAGE_NETWORK, details: [] };
  }
  const body: unknown = error.error;
  if (isErrorResponse(body)) {
    return {
      status: error.status,
      code: body.error.code ?? `http.${error.status}`,
      message: body.error.message,
      details: fieldDetails(body.error.details),
      ...retryAfterOf(error),
      ...(mappingRequirements(body.error.mapping_required)
        ? { mappingRequired: mappingRequirements(body.error.mapping_required) }
        : {}),
    };
  }
  return {
    status: error.status,
    code: `http.${error.status}`,
    message: defaultMessageForStatus(error.status),
    details: [],
    ...retryAfterOf(error),
  };
}

/** Normalizes any thrown value into an ApiError. Already normalized values are returned as is. */
export function asApiError(error: unknown): ApiError {
  if (isApiError(error)) {
    return error;
  }
  if (error instanceof HttpErrorResponse) {
    return fromHttpError(error);
  }
  if (error instanceof TimeoutError) {
    return { status: 0, code: 'network.timeout', message: MESSAGE_TIMEOUT, details: [] };
  }
  return {
    status: 0,
    code: 'unknown',
    message: error instanceof Error && error.message ? error.message : MESSAGE_GENERIC,
    details: [],
  };
}

/** Maps field-level details to a `{ field: message }` record for forms and summaries. */
export function fieldErrorsOf(error: ApiError): Readonly<Record<string, string>> {
  return Object.fromEntries(error.details.map((detail) => [detail.field, detail.message]));
}

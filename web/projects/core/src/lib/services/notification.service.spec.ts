import { HttpErrorResponse } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { MessageService } from 'primeng/api';

import {
  NotificationService,
  TOAST_ERROR_LIFE_MS,
  TOAST_LIFE_MS,
  TOAST_MAX_VISIBLE,
  toastCopyOf,
} from './notification.service';

function setup() {
  TestBed.configureTestingModule({ providers: [MessageService] });
  const messages = TestBed.inject(MessageService);
  return {
    notify: TestBed.inject(NotificationService),
    add: vi.spyOn(messages, 'add'),
    clear: vi.spyOn(messages, 'clear'),
  };
}

describe('NotificationService', () => {
  afterEach(() => vi.useRealTimers());

  it('sends success toasts that close after 5 seconds', () => {
    const { notify, add } = setup();
    notify.success('Guardado');
    expect(add).toHaveBeenCalledWith({
      severity: 'success',
      summary: 'Listo',
      detail: 'Guardado',
      life: TOAST_LIFE_MS,
    });
    expect(TOAST_LIFE_MS).toBe(5000);
  });

  it('keeps error toasts visible for 8 seconds', () => {
    const { notify, add } = setup();
    notify.error('No se pudo guardar');
    expect(add).toHaveBeenCalledWith(
      expect.objectContaining({ severity: 'error', detail: 'No se pudo guardar', life: 8000 }),
    );
    expect(TOAST_ERROR_LIFE_MS).toBe(8000);
  });

  it('sends info toasts with a short title', () => {
    const { notify, add } = setup();
    notify.info('Sesión iniciada');
    expect(add).toHaveBeenCalledWith(
      expect.objectContaining({ severity: 'info', summary: 'Aviso', life: TOAST_LIFE_MS }),
    );
  });

  it('shows identical messages once while the first is still on screen', () => {
    const { notify, add } = setup();
    notify.error('No se pudo guardar');
    notify.error('No se pudo guardar');
    notify.success('No se pudo guardar');
    expect(add).toHaveBeenCalledTimes(2);
  });

  it('shows the same message again once the first one has expired', () => {
    vi.useFakeTimers();
    const { notify, add } = setup();
    notify.success('Guardado');
    vi.advanceTimersByTime(TOAST_LIFE_MS + 1);
    notify.success('Guardado');
    expect(add).toHaveBeenCalledTimes(2);
  });

  it('shows the same message again once the outlet released it', () => {
    const { notify, add } = setup();
    notify.success('Guardado');
    notify.released(add.mock.calls[0][0]);
    notify.success('Guardado');
    expect(add).toHaveBeenCalledTimes(2);
  });

  it('never keeps more than three toasts: the oldest is dropped', () => {
    const { notify, add, clear } = setup();
    for (const text of ['Uno', 'Dos', 'Tres']) notify.success(text);
    expect(clear).not.toHaveBeenCalled();

    notify.success('Cuatro');
    expect(TOAST_MAX_VISIBLE).toBe(3);
    expect(clear).toHaveBeenCalledTimes(1);
    const shownAfterClear = add.mock.calls.slice(3).map(([message]) => message.detail);
    expect(shownAfterClear).toEqual(['Dos', 'Tres', 'Cuatro']);
  });
});

describe('toastCopyOf', () => {
  const api = (status: number, code: string, message = 'Mensaje del servidor') => ({
    status,
    code,
    message,
    details: [],
  });

  it('maps forbidden, not found, network, validation and rate limit to Spanish copy', () => {
    expect(toastCopyOf(api(403, 'http.403')).summary).toBe('Sin permiso');
    expect(toastCopyOf(api(404, 'http.404')).summary).toBe('No encontrado');
    expect(toastCopyOf(api(0, 'network.unreachable')).summary).toBe('Sin conexión');
    expect(toastCopyOf(api(429, 'http.429')).summary).toBe('Demasiadas solicitudes');
    expect(toastCopyOf(api(422, 'issue.validation_failed')).summary).toBe('Revisa los datos');
  });

  it('maps by error code before the status', () => {
    expect(toastCopyOf(api(400, 'project.forbidden')).summary).toBe('Sin permiso');
  });

  it('keeps the server message of an unknown failure', () => {
    expect(toastCopyOf(api(409, 'issue.conflict', 'La incidencia ya existe.'))).toEqual({
      summary: 'Error',
      detail: 'La incidencia ya existe.',
    });
  });

  it('translates English server text and keeps Spanish text', () => {
    expect(toastCopyOf('forbidden').summary).toBe('Sin permiso');
    expect(toastCopyOf('Project not found').summary).toBe('No encontrado');
    expect(toastCopyOf('Failed to fetch').summary).toBe('Sin conexión');
    expect(toastCopyOf('Too many requests').summary).toBe('Demasiadas solicitudes');
    expect(toastCopyOf('invalid label id').summary).toBe('Revisa los datos');
    expect(toastCopyOf('No se pudo guardar el equipo.')).toEqual({
      summary: 'Error',
      detail: 'No se pudo guardar el equipo.',
    });
    expect(toastCopyOf('No tienes permisos para realizar esta operación.').summary).toBe(
      'Sin permiso',
    );
  });

  it('understands raw HTTP errors and falls back for unknown values', () => {
    const offline = new HttpErrorResponse({ status: 0 });
    expect(toastCopyOf(offline).summary).toBe('Sin conexión');
    expect(toastCopyOf(undefined, 'Algo falló.')).toEqual({
      summary: 'Error',
      detail: 'Algo falló.',
    });
  });
});

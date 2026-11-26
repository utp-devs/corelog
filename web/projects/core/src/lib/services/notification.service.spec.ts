import { TestBed } from '@angular/core/testing';
import { MessageService } from 'primeng/api';

import { NotificationService } from './notification.service';

describe('NotificationService', () => {
  it('sends success toasts with a 3 second life', () => {
    TestBed.configureTestingModule({ providers: [MessageService] });
    const add = vi.spyOn(TestBed.inject(MessageService), 'add');
    TestBed.inject(NotificationService).success('Guardado');
    expect(add).toHaveBeenCalledWith({
      severity: 'success',
      summary: 'Listo',
      detail: 'Guardado',
      life: 3000,
    });
  });

  it('keeps error toasts visible for 6 seconds', () => {
    TestBed.configureTestingModule({ providers: [MessageService] });
    const add = vi.spyOn(TestBed.inject(MessageService), 'add');
    TestBed.inject(NotificationService).error('No se pudo guardar');
    expect(add).toHaveBeenCalledWith(
      expect.objectContaining({ severity: 'error', detail: 'No se pudo guardar', life: 6000 }),
    );
  });

  it('sends info toasts', () => {
    TestBed.configureTestingModule({ providers: [MessageService] });
    const add = vi.spyOn(TestBed.inject(MessageService), 'add');
    TestBed.inject(NotificationService).info('Sesión iniciada');
    expect(add).toHaveBeenCalledWith(expect.objectContaining({ severity: 'info', life: 3000 }));
  });
});

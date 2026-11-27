import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { SessionService } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';

import { ScratchpadDrawer } from './scratchpad-drawer';

const BASE = '/api/v1/notes';

function dto(id: string, extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id,
    title: `Nota ${id}`,
    body: 'Texto de la nota',
    color: 'yellow',
    pinned: false,
    tags: [],
    issue_id: null,
    project_id: null,
    created_at: '2026-11-01T10:00:00Z',
    updated_at: '2026-11-01T10:00:00Z',
    ...extra,
  };
}

@Component({
  imports: [ScratchpadDrawer],
  template: '<app-scratchpad-drawer [(open)]="open" />',
})
class Host {
  readonly open = signal(false);
}

async function openDrawer(): Promise<{
  fixture: ComponentFixture<Host>;
  el: HTMLElement;
  backend: HttpTestingController;
}> {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  // A signed-in session: the notes store clears itself when there is none.
  TestBed.inject(SessionService).startSession('token-1');
  const fixture = TestBed.createComponent(Host);
  const backend = TestBed.inject(HttpTestingController);
  fixture.detectChanges();
  fixture.componentInstance.open.set(true);
  fixture.detectChanges();
  await fixture.whenStable();
  backend
    .expectOne((r) => r.url === BASE && r.params.get('limit') === '5')
    .flush({ data: [dto('a'), dto('b', { pinned: true })], meta: { limit: 5, next_cursor: null } });
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, backend };
}

describe('ScratchpadDrawer', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
    document.body.style.removeProperty('overflow');
  });

  it('shows the privacy hint, a composer and the recent notes', async () => {
    const { el } = await openDrawer();
    expect(el.textContent).toContain('Solo tú puedes ver tus notas.');
    expect(el.querySelector('textarea')).not.toBeNull();
    expect(el.querySelectorAll('.scratchpad-drawer__item')).toHaveLength(2);
  });

  it('keeps Guardar disabled until there is text, then creates the note', async () => {
    const { fixture, el, backend } = await openDrawer();
    const save = () => el.querySelector<HTMLButtonElement>('p-button button') as HTMLButtonElement;
    expect(save().disabled).toBe(true);

    const field = el.querySelector('textarea') as HTMLTextAreaElement;
    field.value = '  Llamar a soporte ';
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(save().disabled).toBe(false);

    save().click();
    const request = backend.expectOne((r) => r.method === 'POST' && r.url === BASE);
    expect(request.request.body).toEqual({ body: 'Llamar a soporte' });
    request.flush({ data: dto('c', { title: '', body: 'Llamar a soporte' }) });
    fixture.detectChanges();
    await fixture.whenStable();
    expect(field.value).toBe('');
    expect(el.querySelectorAll('.scratchpad-drawer__item')).toHaveLength(3);
  });

  it('pins a note and opens the notes page from "Ver todas"', async () => {
    const { fixture, el, backend } = await openDrawer();
    const router = TestBed.inject(Router);
    const navigate = vi.spyOn(router, 'navigate').mockResolvedValue(true);

    el.querySelector<HTMLButtonElement>('.scratchpad-drawer__pin')?.click();
    backend.expectOne(`${BASE}/a/pin`).flush({ data: dto('a', { pinned: true }) });

    el.querySelector<HTMLButtonElement>('.scratchpad-drawer__all')?.click();
    expect(navigate).toHaveBeenCalledWith(['/notes']);
    fixture.detectChanges();
    expect(fixture.componentInstance.open()).toBe(false);
  });
});

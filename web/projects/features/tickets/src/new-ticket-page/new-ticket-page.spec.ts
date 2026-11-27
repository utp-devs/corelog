import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, Routes } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { NotificationService, UNSAVED_CHANGES_PROMPT, unsavedChangesGuard } from '@corelog/core';
import { guardConsole, installViewport } from '@corelog/core/testing';
import { Observable, throwError, of } from 'rxjs';
import { type Mock, afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { projectOf, setupTickets, ticketOf } from '../testing/ticket-test-kit';
import { NewTicketPage } from './new-ticket-page';

@Component({ selector: 'app-list-stub', template: '<p>Listado de incidencias</p>' })
class ListStub {}

@Component({ selector: 'app-detail-stub', template: '<p>Detalle de la incidencia</p>' })
class DetailStub {}

const ROUTES: Routes = [
  { path: 'issues/new', component: NewTicketPage, canDeactivate: [unsavedChangesGuard] },
  { path: 'issues/:id', component: DetailStub },
  { path: 'issues', component: ListStub },
];

function setField(selector: string, value: string): void {
  const element = document.querySelector<HTMLInputElement | HTMLTextAreaElement>(selector);
  if (!element) throw new Error(`no field ${selector}`);
  element.value = value;
  element.dispatchEvent(new Event('input'));
  element.dispatchEvent(new Event('blur'));
}

function fillTitle(value: string): void {
  setField('input[formcontrolname="title"]', value);
}

function fillDescription(value: string): void {
  setField('textarea[aria-label="Descripción"]', value);
}

function buttonByText(text: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === text,
  );
}

function clickButton(text: string): void {
  const button = buttonByText(text);
  if (!button) throw new Error(`no button named ${text}`);
  button.click();
}

function headingNamed(text: string): HTMLElement | undefined {
  return Array.from(document.querySelectorAll<HTMLElement>('h1, h2, h3')).find(
    (heading) => heading.textContent?.trim() === text,
  );
}

async function renderPage(): Promise<RouterTestingHarness> {
  const harness = await RouterTestingHarness.create('/issues/new');
  harness.fixture.detectChanges();
  await harness.fixture.whenStable();
  harness.fixture.detectChanges();
  return harness;
}

describe('NewTicketPage', () => {
  const guard = guardConsole();
  let restoreViewport: () => void;
  let prompt: Mock<(copy: { title: string }) => Promise<boolean>>;

  beforeEach(() => {
    window.localStorage.clear();
    restoreViewport = installViewport(1280);
    prompt = vi.fn((_copy: { title: string }) => Promise.resolve(false));
  });

  afterEach(() => {
    restoreViewport();
  });

  function setup(create: () => Observable<unknown> = () => of(ticketOf({ id: 'issue-9' }))) {
    const ticketSetup = setupTickets({ ticketApi: { create } });
    TestBed.configureTestingModule({
      providers: [{ provide: UNSAVED_CHANGES_PROMPT, useValue: prompt }],
    });
    TestBed.inject(Router).resetConfig(ROUTES);
    return ticketSetup;
  }

  it('groups the form in sections with labelled fields', async () => {
    setup();
    await renderPage();

    expect(headingNamed('Nueva incidencia')).toBeDefined();
    expect(headingNamed('Datos principales')).toBeDefined();
    expect(headingNamed('Clasificación')).toBeDefined();
    expect(document.querySelector('textarea[aria-label="Descripción"]')).not.toBeNull();
    expect(document.querySelector('input[formcontrolname="title"]')).not.toBeNull();
    expect(buttonByText('Crear incidencia')).toBeDefined();
    // Two columns on desktop: the main fields and the classification side column.
    expect(document.querySelector('aside[aria-label="Clasificación"]')).not.toBeNull();
    guard.expectNoConsoleOutput();
  });

  it('shows validation messages on submit without calling the server', async () => {
    const setupResult = setup();
    const harness = await renderPage();

    clickButton('Crear incidencia');
    harness.fixture.detectChanges();

    expect(document.body.textContent).toContain('Escribe un título.');
    expect(document.body.textContent).toContain('Describe la incidencia.');
    expect(setupResult.ticketApi.create).not.toHaveBeenCalled();
  });

  it('shows no error on untouched empty fields', async () => {
    setup();
    await renderPage();

    expect(document.querySelector('.field-row__text--error')).toBeNull();
    expect(document.querySelector('[aria-invalid="true"]')).toBeNull();
  });

  it('creates the issue in the preselected project and remembers it', async () => {
    const setupResult = setup();
    const harness = await renderPage();
    fillTitle('Pantalla en blanco');
    fillDescription('Al abrir el panel');
    harness.fixture.detectChanges();

    clickButton('Crear incidencia');
    await harness.fixture.whenStable();

    expect(setupResult.ticketApi.create).toHaveBeenCalledWith(
      expect.objectContaining({ projectId: 'project-1' }),
    );
    expect(window.localStorage.getItem('corelog.create-issue.project')).toBe('project-1');
  });

  it('preselects the project used last time', async () => {
    window.localStorage.setItem('corelog.create-issue.project', 'project-2');
    const setupResult = setup();
    setupResult.projectApi['list'].mockReturnValue(
      of([projectOf(), projectOf({ id: 'project-2', name: 'Otro', key: 'OTR' })]),
    );
    await renderPage();

    expect(document.body.textContent).toContain('Otro (OTR)');
  });

  it('leaves the page without a prompt when nothing was typed', async () => {
    setup();
    const harness = await renderPage();

    clickButton('Cancelar');
    await harness.fixture.whenStable();

    expect(TestBed.inject(Router).url).toBe('/issues');
    expect(prompt).not.toHaveBeenCalled();
  });

  it('asks before leaving with typed values and stays when the user keeps editing', async () => {
    setup();
    const harness = await renderPage();
    fillTitle('Pantalla en blanco');
    harness.fixture.detectChanges();

    clickButton('Cancelar');
    await harness.fixture.whenStable();

    expect(prompt).toHaveBeenCalledWith(
      expect.objectContaining({ title: 'Tienes cambios sin guardar' }),
    );
    expect(TestBed.inject(Router).url).toBe('/issues/new');
    expect(document.querySelector<HTMLInputElement>('input[formcontrolname="title"]')?.value).toBe(
      'Pantalla en blanco',
    );
  });

  it('leaves after confirming the discard dialog', async () => {
    setup();
    prompt.mockResolvedValue(true);
    const harness = await renderPage();
    fillTitle('Pantalla en blanco');
    harness.fixture.detectChanges();

    clickButton('Cancelar');
    await harness.fixture.whenStable();

    expect(TestBed.inject(Router).url).toBe('/issues');
  });

  it('redirects to the new issue after a successful create without showing the prompt', async () => {
    const setupResult = setup();
    const harness = await renderPage();
    fillTitle('Pantalla en blanco');
    fillDescription('Al abrir el panel');
    harness.fixture.detectChanges();

    clickButton('Crear incidencia');
    await harness.fixture.whenStable();

    expect(TestBed.inject(Router).url).toBe('/issues/issue-9');
    expect(prompt).not.toHaveBeenCalled();
    expect(setupResult.ticketApi.create).toHaveBeenCalledWith(
      expect.objectContaining({ title: 'Pantalla en blanco' }),
    );
  });

  it('stays on the form with the typed values when the server refuses the issue', async () => {
    setup(() => throwError(() => new Error('El proyecto está archivado')));
    const notify = vi.spyOn(TestBed.inject(NotificationService), 'error');
    const harness = await renderPage();
    fillTitle('Pantalla en blanco');
    fillDescription('Al abrir el panel');
    harness.fixture.detectChanges();

    clickButton('Crear incidencia');
    await harness.fixture.whenStable();

    expect(notify).toHaveBeenCalledWith('El proyecto está archivado');
    expect(TestBed.inject(Router).url).toBe('/issues/new');
    expect(document.querySelector<HTMLInputElement>('input[formcontrolname="title"]')?.value).toBe(
      'Pantalla en blanco',
    );
  });

  it('renders without console warnings or errors', async () => {
    setup();
    await renderPage();
    guard.expectNoConsoleOutput();
  });
});

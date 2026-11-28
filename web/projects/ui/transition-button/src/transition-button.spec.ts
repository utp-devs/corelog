import { ComponentFixture, TestBed } from '@angular/core/testing';
import { IssueTransition } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { TransitionButton } from './transition-button';

const START: IssueTransition = {
  id: 'start',
  name: 'Iniciar',
  toStatus: 'in_progress',
  to: { id: 's2', name: 'En curso', category: 'in_progress' },
  requiredFields: [],
  requiresComment: false,
  requiresResolution: false,
  requiresAssignee: false,
  needsScreen: false,
};
const FINISH: IssueTransition = {
  ...START,
  id: 'finish',
  name: 'Terminar',
  toStatus: 'closed',
  to: { id: 's3', name: 'Hecho', category: 'done' },
};

function create(
  inputs: Partial<{
    transitions: IssueTransition[];
    loading: boolean;
    error: string | null;
    busy: boolean;
    disabledReason: string;
  }> = {},
): { fixture: ComponentFixture<TransitionButton>; el: HTMLElement } {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(TransitionButton);
  fixture.componentRef.setInput('current', { name: 'Por hacer', category: 'todo' });
  for (const [key, value] of Object.entries({ transitions: [START, FINISH], ...inputs })) {
    fixture.componentRef.setInput(key, value);
  }
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

function trigger(el: HTMLElement): HTMLButtonElement {
  return el.querySelector('[data-testid="transition-button"]') as HTMLButtonElement;
}

/** The menu lives in the overlay container, so the whole document is the search root. */
const menu = (): HTMLElement | null => document.querySelector('[role="menu"]');

describe('TransitionButton', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('shows the current status with the color of its category', () => {
    const { el } = create();
    expect(trigger(el).textContent).toContain('Por hacer');
    expect(trigger(el).classList).toContain('transition-button__trigger--todo');
  });

  it('opens a menu with one option per transition, colored by the target category', () => {
    const { fixture, el } = create();
    trigger(el).click();
    fixture.detectChanges();
    expect(menu()?.closest('.cdk-overlay-container')).not.toBeNull();
    const options = document.querySelectorAll('[role="menuitem"]');
    expect(options).toHaveLength(2);
    expect(options[1].querySelector('.transition-button__target--done')?.textContent).toContain(
      'Hecho',
    );
  });

  it('emits the chosen transition and closes the menu', () => {
    const { fixture, el } = create();
    const chosen: string[] = [];
    fixture.componentInstance.selected.subscribe((item) => chosen.push(item.id));
    trigger(el).click();
    fixture.detectChanges();
    (document.querySelector('[data-testid="transition-option-finish"]') as HTMLElement).click();
    fixture.detectChanges();
    expect(chosen).toEqual(['finish']);
    expect(menu()).toBeNull();
  });

  it('is inert with an explanation when there is no transition to run', () => {
    const { fixture, el } = create({ transitions: [] });
    expect(trigger(el).getAttribute('aria-disabled')).toBe('true');
    expect(trigger(el).getAttribute('title')).toContain('No hay transiciones');
    trigger(el).click();
    fixture.detectChanges();
    expect(menu()).toBeNull();
  });

  it('shows the loading state and blocks the menu while a transition runs', () => {
    const loading = create({ loading: true });
    expect(trigger(loading.el).getAttribute('aria-busy')).toBe('true');
    TestBed.resetTestingModule();
    const busy = create({ busy: true });
    expect(busy.el.querySelector('.pi-spinner')).not.toBeNull();
    trigger(busy.el).click();
    busy.fixture.detectChanges();
    expect(busy.el.querySelector('[role="menu"]')).toBeNull();
  });

  it('offers a retry when the transitions could not be loaded', () => {
    const { fixture, el } = create({ transitions: [], error: 'No se pudo cargar' });
    let retried = 0;
    fixture.componentInstance.retry.subscribe(() => (retried += 1));
    trigger(el).click();
    fixture.detectChanges();
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('No se pudo cargar');
    (document.querySelector('.transition-button__retry') as HTMLElement).click();
    expect(retried).toBe(1);
  });

  it('blocks the button with the reason it is given', () => {
    const { el } = create({ disabledReason: 'El proyecto está archivado' });
    expect(trigger(el).getAttribute('title')).toBe('El proyecto está archivado');
    expect(trigger(el).getAttribute('aria-disabled')).toBe('true');
  });

  it('closes with Escape and when the pointer goes down outside', () => {
    const { fixture, el } = create();
    trigger(el).click();
    fixture.detectChanges();
    el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    fixture.detectChanges();
    expect(menu()).toBeNull();
    trigger(el).click();
    fixture.detectChanges();
    document.body.click();
    fixture.detectChanges();
    expect(menu()).toBeNull();
  });

  it('returns focus to the button when Escape closes the menu', () => {
    const { fixture, el } = create();
    trigger(el).click();
    fixture.detectChanges();
    menu()?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    fixture.detectChanges();
    expect(menu()).toBeNull();
    expect(document.activeElement).toBe(trigger(el));
  });
});

import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { IssueTransition } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { describe, expect, it } from 'vitest';

import { TransitionDialog, TransitionDialogValue } from './transition-dialog';

const CLOSE: IssueTransition = {
  id: 'close',
  name: 'Cerrar',
  toStatus: 'closed',
  to: { id: 's3', name: 'Hecho', category: 'done' },
  requiredFields: ['due_date', 'story_points'],
  requiresComment: true,
  requiresResolution: false,
  requiresAssignee: false,
  needsScreen: true,
};

function create(transition: IssueTransition = CLOSE): {
  fixture: ComponentFixture<TransitionDialog>;
  el: HTMLElement;
  confirmed: TransitionDialogValue[];
} {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  const fixture = TestBed.createComponent(TransitionDialog);
  fixture.componentRef.setInput('open', true);
  fixture.componentRef.setInput('transition', transition);
  const confirmed: TransitionDialogValue[] = [];
  fixture.componentInstance.confirmed.subscribe((value) => confirmed.push(value));
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, confirmed };
}

function type(el: HTMLElement, selector: string, value: string): void {
  const input = el.querySelector(selector) as HTMLInputElement | HTMLTextAreaElement;
  input.value = value;
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

function confirm(fixture: ComponentFixture<TransitionDialog>, el: HTMLElement): void {
  (el.querySelector('[data-testid="transition-confirm"] button') as HTMLButtonElement).click();
  fixture.detectChanges();
}

describe('TransitionDialog', () => {
  it('renders a field per required field, from its type, and the comment', () => {
    const { el } = create();
    expect(
      el.querySelector('[data-testid="transition-field-story_points"]')?.getAttribute('type'),
    ).toBe('number');
    expect(el.querySelector('app-date-field')).not.toBeNull();
    expect(el.querySelector('[data-testid="transition-comment"]')).not.toBeNull();
    expect(el.querySelector('.dialog__subtitle')?.textContent).toContain('Hecho');
  });

  it('does not confirm until every required value is there and shows what is missing', () => {
    const { fixture, el, confirmed } = create();
    confirm(fixture, el);
    expect(confirmed).toEqual([]);
    expect(el.textContent).toContain('Escribe un comentario.');
    expect(el.textContent).toContain('Este campo es obligatorio.');
  });

  it('emits the typed values once the requirements are met', () => {
    const { fixture, el, confirmed } = create({ ...CLOSE, requiredFields: ['story_points'] });
    type(el, '[data-testid="transition-field-story_points"]', '5');
    type(el, '[data-testid="transition-comment"]', ' Verificado ');
    fixture.detectChanges();
    confirm(fixture, el);
    expect(confirmed).toEqual([{ fields: { story_points: '5' }, comment: 'Verificado' }]);
  });

  it('puts the server details next to the fields they name', () => {
    const { fixture, el } = create();
    fixture.componentRef.setInput('serverErrors', [
      { field: 'comment', message: 'el comentario es obligatorio para esta transición' },
      { field: 'subtasks', message: 'todas las subtareas deben estar completadas' },
    ]);
    fixture.detectChanges();
    expect(el.textContent).toContain('el comentario es obligatorio para esta transición');
    // A detail without a field of its own is shown as an alert in the dialog.
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('subtareas');
  });

  it('shows a failure that has no field next to it and keeps the dialog usable', () => {
    const { fixture, el } = create();
    fixture.componentRef.setInput('error', 'Otra persona cambió el estado');
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="transition-error"]')?.textContent).toContain(
      'Otra persona',
    );
  });

  it('asks for the resolution and the assignee only when the transition requires them', () => {
    const { el } = create({
      ...CLOSE,
      requiredFields: [],
      requiresComment: false,
      requiresResolution: true,
      requiresAssignee: true,
    });
    expect(el.textContent).toContain('Resolución');
    expect(el.textContent).toContain('Responsable');
    expect(el.textContent).toContain('(opcional)');
  });
});

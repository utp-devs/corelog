import { ComponentFixture, TestBed } from '@angular/core/testing';
import { WorkflowRule, specsOfKind } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { WorkflowRuleList } from './rule-list';

if (!window.matchMedia) {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    }),
  });
}

function create(
  kind: 'condition' | 'validator' | 'postFunction',
  rules: WorkflowRule[],
  canEdit = true,
): { fixture: ComponentFixture<WorkflowRuleList>; el: HTMLElement; emitted: WorkflowRule[][] } {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(WorkflowRuleList);
  fixture.componentRef.setInput('kind', kind);
  fixture.componentRef.setInput('title', 'Reglas');
  fixture.componentRef.setInput('rules', rules);
  fixture.componentRef.setInput('canEdit', canEdit);
  const emitted: WorkflowRule[][] = [];
  fixture.componentInstance.rulesChange.subscribe((next) => emitted.push([...next]));
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, emitted };
}

describe('WorkflowRuleList', () => {
  it('builds the form of a condition from the registry: roles as checkboxes', () => {
    const { el } = create('condition', [{ type: 'user_in_role', params: { roles: ['lead'] } }]);
    const checks = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
    expect(checks).toHaveLength(3);
    expect(checks.map((check) => check.checked)).toEqual([true, false, false]);
  });

  it('writes the roles back when one is ticked', () => {
    const { el, emitted } = create('condition', [
      { type: 'user_in_role', params: { roles: ['lead'] } },
    ]);
    const second = el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')[1];
    second.checked = true;
    second.dispatchEvent(new Event('change', { bubbles: true }));
    expect(emitted[0][0].params['roles']).toEqual(['lead', 'contributor']);
  });

  it('flags a condition without roles', () => {
    const { el } = create('condition', [{ type: 'user_in_role', params: { roles: [] } }]);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('al menos un rol');
  });

  it('shows the text parameters of a validator and writes them back', () => {
    const { el, emitted } = create('validator', [
      { type: 'required_field', params: { field: '' } },
    ]);
    const input = el.querySelector('input[type="text"]') as HTMLInputElement;
    input.value = 'due_date';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    expect(emitted[0][0]).toEqual({ type: 'required_field', params: { field: 'due_date' } });
  });

  it('renders no parameters for a rule without them', () => {
    const { el } = create('validator', [{ type: 'comment_required', params: {} }]);
    expect(el.querySelector('input[type="text"]')).toBeNull();
    expect(el.querySelector('[role="alert"]')).toBeNull();
  });

  it('removes a rule and offers only the rules of its kind', () => {
    const { el, emitted } = create('postFunction', [
      { type: 'clear_resolution', params: {} },
      { type: 'emit', params: { event: 'x.y' } },
    ]);
    (
      el.querySelector('button[aria-label="Quitar Borrar la resolución"]') as HTMLButtonElement
    ).click();
    expect(emitted[0].map((rule) => rule.type)).toEqual(['emit']);
    expect(specsOfKind('postFunction')).toHaveLength(6);
  });

  it('is read-only without edit rights', () => {
    const { el } = create('validator', [{ type: 'comment_required', params: {} }], false);
    expect(el.querySelector('button[aria-label^="Quitar"]')).toBeNull();
    expect(el.textContent).toContain('Exigir un comentario');
  });
});

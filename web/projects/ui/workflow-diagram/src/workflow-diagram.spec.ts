import { ComponentFixture, TestBed } from '@angular/core/testing';
import { WorkflowStatusRef, WorkflowTransition } from '@corelog/core';
import { installViewport } from '@corelog/core/testing';
import { afterEach, describe, expect, it } from 'vitest';

import { WorkflowDiagram } from './workflow-diagram';

const STATUSES: WorkflowStatusRef[] = [
  { id: 'a', name: 'Por hacer', category: 'todo' },
  { id: 'b', name: 'En curso', category: 'in_progress' },
  { id: 'c', name: 'Hecho', category: 'done' },
];

function transition(id: string, from: string[], to: string): WorkflowTransition {
  return {
    id,
    name: id,
    from,
    to,
    conditions: [],
    validators: [],
    postFunctions: [],
    requiredFields: [],
  };
}

function create(): { fixture: ComponentFixture<WorkflowDiagram>; el: HTMLElement } {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(WorkflowDiagram);
  fixture.componentRef.setInput('statuses', STATUSES);
  fixture.componentRef.setInput('initialId', 'a');
  fixture.componentRef.setInput('transitions', [
    transition('start', ['a'], 'b'),
    transition('finish', ['b'], 'c'),
    transition('reopen', [], 'a'),
  ]);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

describe('WorkflowDiagram', () => {
  let restore: (() => void) | null = null;
  afterEach(() => restore?.());

  it('leaves the nodes out of the tab order on a phone, where the table is the way in', () => {
    restore = installViewport(390);
    const { el } = create();
    expect(el.querySelector('[data-testid="diagram-node-a"]')?.getAttribute('tabindex')).toBeNull();
  });

  it('draws a focusable node per status and a virtual one for any-source transitions', () => {
    restore = installViewport(1280);
    const { el } = create();
    const nodes = el.querySelectorAll('.workflow-diagram__node');
    expect(nodes).toHaveLength(4);
    expect(el.querySelector('[data-testid="diagram-node-a"]')?.getAttribute('tabindex')).toBe('0');
    expect(el.querySelector('.workflow-diagram__node--any')?.getAttribute('tabindex')).toBeNull();
  });

  it('draws an edge with an arrowhead and a label for every transition', () => {
    const { el } = create();
    const edges = el.querySelectorAll('.workflow-diagram__edge');
    expect(edges).toHaveLength(3);
    expect(el.querySelector('marker')).not.toBeNull();
    expect(el.querySelector('.workflow-diagram__line')?.getAttribute('marker-end')).toContain(
      'workflow-diagram-arrow',
    );
    expect(el.querySelector('.workflow-diagram__label')?.textContent?.trim()).not.toBe('');
  });

  it('zooms with the buttons and fits back to the view', () => {
    const { fixture, el } = create();
    const zoom = (): string | null =>
      el.querySelector('.workflow-diagram__zoom')?.textContent ?? null;
    expect(zoom()?.trim()).toBe('100 %');
    (el.querySelector('button[aria-label="Acercar"]') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(zoom()?.trim()).toBe('120 %');
    (el.querySelector('button[aria-label="Alejar"]') as HTMLButtonElement).click();
    (el.querySelector('button[aria-label="Alejar"]') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(zoom()?.trim()).toBe('83 %');
    (el.querySelector('button[aria-label="Ajustar a la vista"]') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(zoom()?.trim()).toBe('100 %');
  });

  it('zooms with the wheel', () => {
    const { fixture, el } = create();
    el.querySelector('svg')?.dispatchEvent(
      new WheelEvent('wheel', { deltaY: -100, bubbles: true, cancelable: true }),
    );
    fixture.detectChanges();
    expect(el.querySelector('.workflow-diagram__zoom')?.textContent?.trim()).toBe('120 %');
  });

  it('selects a status from its node with the keyboard and a transition from its edge with the pointer', () => {
    const { fixture, el } = create();
    const transitions: string[] = [];
    const statuses: string[] = [];
    fixture.componentInstance.transitionSelected.subscribe((id) => transitions.push(id));
    fixture.componentInstance.statusSelected.subscribe((id) => statuses.push(id));
    el.querySelector('[data-testid="diagram-edge-start"]')?.dispatchEvent(
      new MouseEvent('click', { bubbles: true }),
    );
    el.querySelector('[data-testid="diagram-node-b"]')?.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }),
    );
    expect(transitions).toEqual(['start']);
    expect(statuses).toEqual(['b']);
  });
});

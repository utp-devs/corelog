import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { BoardCardField, BoardViewCard } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { KanbanCard, KanbanMoveTarget } from './kanban-card';

function cardOf(extra: Partial<BoardViewCard> = {}): BoardViewCard {
  return {
    id: 'i1',
    key: 'CORE-7',
    summary: 'Arreglar el pago',
    type: { name: 'Error', key: 'bug', icon: 'bug', color: 'red' },
    priority: { name: 'Alta', key: 'high', icon: 'arrow-up', color: 'orange' },
    status: { id: 's', name: 'Por hacer', category: 'todo' },
    assignee: { id: 'u1', name: 'Ada Lovelace' },
    labels: ['pagos', 'urgente', 'backend'],
    storyPoints: 5,
    remainingEstimateSeconds: null,
    flagged: true,
    rank: 'a',
    sprintId: null,
    epic: { id: 'e1', key: 'CORE-1', summary: 'Checkout' },
    parent: null,
    subtasks: { total: 4, done: 1 },
    swimlaneId: '',
    ...extra,
  };
}

interface Created {
  el: HTMLElement;
  moved: string[];
  shifted: number[];
  detect: () => void;
}

function create(
  card: BoardViewCard,
  targets: readonly KanbanMoveTarget[] = [],
  fields?: readonly BoardCardField[],
): Created {
  TestBed.resetTestingModule();
  TestBed.configureTestingModule({ providers: [provideRouter([])] });
  const fixture = TestBed.createComponent(KanbanCard);
  fixture.componentRef.setInput('card', card);
  fixture.componentRef.setInput('targets', targets);
  if (fields) fixture.componentRef.setInput('fields', fields);
  const moved: string[] = [];
  const shifted: number[] = [];
  fixture.componentInstance.moved.subscribe((id) => moved.push(id));
  fixture.componentInstance.shifted.subscribe((step) => shifted.push(step));
  fixture.detectChanges();
  return {
    el: fixture.nativeElement as HTMLElement,
    moved,
    shifted,
    detect: () => fixture.detectChanges(),
  };
}

const MENU_LABEL = 'Mover CORE-7 a otra columna';

describe('KanbanCard', () => {
  it('shows key, summary, points, subtasks, epic and the first two labels', () => {
    const { el } = create(cardOf());
    expect(el.querySelector('.kanban-card__key')?.textContent).toContain('CORE-7');
    const link = el.querySelector<HTMLAnchorElement>('.kanban-card__summary');
    expect(link?.textContent).toContain('Arreglar el pago');
    expect(link?.getAttribute('href')).toBe('/browse/CORE-7');
    expect(el.querySelector('.kanban-card__points')?.textContent).toContain('5');
    expect(el.querySelector('.kanban-card__subtasks')?.textContent).toContain('1/4');
    expect(el.querySelector('.kanban-card__epic')?.textContent).toContain('Checkout');
    const labels = Array.from(el.querySelectorAll('.kanban-card__label')).map((label) =>
      label.textContent?.trim(),
    );
    expect(labels).toEqual(['pagos', 'urgente', '+1']);
  });

  it('marks a flagged card and names the flag for screen readers', () => {
    const { el } = create(cardOf());
    expect(el.querySelector('.kanban-card--flagged')).not.toBeNull();
    expect(el.querySelector('[aria-label="Con impedimento"]')).not.toBeNull();
  });

  it('shows a placeholder for a card without assignee', () => {
    const { el } = create(cardOf({ assignee: null, flagged: false }));
    expect(el.querySelector('[aria-label="Sin responsable"]')).not.toBeNull();
    expect(el.querySelector('.kanban-card--flagged')).toBeNull();
  });

  it('hides the menu without targets and lists the targets when it has them', () => {
    expect(create(cardOf()).el.querySelector(`[aria-label="${MENU_LABEL}"]`)).toBeNull();
    const { el, detect } = create(cardOf(), [
      { id: 'c2', name: 'En curso' },
      { id: 'c3', name: 'Hecho' },
    ]);
    const trigger = el.querySelector<HTMLButtonElement>(`[aria-label="${MENU_LABEL}"]`);
    expect(trigger).not.toBeNull();
    trigger?.click();
    detect();
    const items = Array.from(el.querySelectorAll('[role="menuitem"]')).map((item) =>
      item.textContent?.trim(),
    );
    expect(items).toEqual(['En curso', 'Hecho']);
  });

  it('emits the chosen column from the menu', () => {
    const { el, moved, detect } = create(cardOf(), [{ id: 'c2', name: 'En curso' }]);
    el.querySelector<HTMLButtonElement>(`[aria-label="${MENU_LABEL}"]`)?.click();
    detect();
    el.querySelector<HTMLButtonElement>('[role="menuitem"]')?.click();
    expect(moved).toEqual(['c2']);
  });

  it('moves to the neighbor column with Alt+Arrow keys and ignores plain arrows', () => {
    const { el, shifted } = create(cardOf(), [{ id: 'c2', name: 'En curso' }]);
    const article = el.querySelector('article') as HTMLElement;
    const press = (key: string, altKey: boolean): void => {
      article.dispatchEvent(new KeyboardEvent('keydown', { key, altKey, bubbles: true }));
    };
    press('ArrowRight', true);
    press('ArrowLeft', true);
    press('ArrowRight', false);
    expect(shifted).toEqual([1, -1]);
  });

  it('does not react to Alt+Arrow when the card cannot move', () => {
    const { el, shifted } = create(cardOf());
    (el.querySelector('article') as HTMLElement).dispatchEvent(
      new KeyboardEvent('keydown', { key: 'ArrowRight', altKey: true, bubbles: true }),
    );
    expect(shifted).toEqual([]);
  });

  it('shows only the fields the board chose', () => {
    const { el } = create(cardOf(), [], ['key']);
    expect(el.querySelector('.kanban-card__key')).not.toBeNull();
    expect(el.querySelector('.kanban-card__points')).toBeNull();
    expect(el.querySelector('.kanban-card__epic')).toBeNull();
    expect(el.querySelector('.kanban-card__assignee, .kanban-card__unassigned')).toBeNull();
  });
});

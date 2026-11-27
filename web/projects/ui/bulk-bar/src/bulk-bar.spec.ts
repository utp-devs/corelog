import { TestBed } from '@angular/core/testing';

import { BulkBar, BulkAction } from './bulk-bar';

function create(count: number, actions: BulkAction[] = []) {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(BulkBar);
  fixture.componentRef.setInput('count', count);
  fixture.componentRef.setInput('actions', actions);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

function buttonByText(el: HTMLElement, text: string): HTMLButtonElement {
  return Array.from(el.querySelectorAll('button')).find(
    (b) => b.textContent?.trim() === text,
  ) as HTMLButtonElement;
}

describe('BulkBar', () => {
  it('renders nothing when no row is selected', () => {
    const { el } = create(0);
    expect(el.querySelector('[role="region"][aria-label="Acciones masivas"]')).toBeNull();
  });

  it('shows the count, the actions and the clear button when rows are selected', () => {
    const onDelete = vi.fn();
    const onClear = vi.fn();
    const { fixture, el } = create(3, [
      { key: 'delete', label: 'Eliminar', danger: true, onClick: onDelete },
    ]);
    fixture.componentInstance.clear.subscribe(onClear);

    const region = el.querySelector(
      '[role="region"][aria-label="Acciones masivas"]',
    ) as HTMLElement;
    expect(region.textContent).toContain('3 seleccionados');
    expect(region.querySelector('.count')?.getAttribute('aria-live')).toBe('polite');

    buttonByText(el, 'Eliminar').click();
    expect(onDelete).toHaveBeenCalledTimes(1);

    buttonByText(el, 'Limpiar selección').click();
    expect(onClear).toHaveBeenCalledTimes(1);
  });

  it('uses the singular form for one selected row', () => {
    const { el } = create(1);
    expect(el.textContent).toContain('1 seleccionado');
  });
});

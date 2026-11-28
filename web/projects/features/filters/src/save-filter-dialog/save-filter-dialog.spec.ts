import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { SaveFilterDialog, type SaveFilterRequest } from './save-filter-dialog';

function setup(current: { name: string; description: string } | null = null) {
  const fixture = TestBed.createComponent(SaveFilterDialog);
  fixture.componentRef.setInput('open', true);
  fixture.componentRef.setInput('current', current);
  const saved: SaveFilterRequest[] = [];
  fixture.componentInstance.save.subscribe((request) => saved.push(request));
  fixture.detectChanges();
  return { fixture, saved, root: fixture.nativeElement as HTMLElement };
}

function type(root: HTMLElement, value: string): void {
  const input = root.querySelector<HTMLInputElement>('[data-testid="filter-name"]');
  if (!input) throw new Error('name field not rendered');
  input.value = value;
  input.dispatchEvent(new Event('input'));
}

describe('SaveFilterDialog', () => {
  it('keeps the save button disabled until there is a name', async () => {
    const { fixture, root } = setup();
    const button = (): HTMLButtonElement | null =>
      root.querySelector('[data-testid="filter-save"] button');
    expect(button()?.disabled).toBe(true);
    type(root, '  Mis errores  ');
    await fixture.whenStable();
    fixture.detectChanges();
    expect(button()?.disabled).toBe(false);
  });

  it('emits the trimmed name as a new filter', async () => {
    const { fixture, root, saved } = setup();
    type(root, '  Mis errores  ');
    await fixture.whenStable();
    fixture.detectChanges();
    root.querySelector<HTMLButtonElement>('[data-testid="filter-save"] button')?.click();
    expect(saved).toEqual([{ name: 'Mis errores', description: '', replace: false }]);
  });

  it('offers to update the loaded filter and copies it with another name otherwise', async () => {
    const { fixture, root, saved } = setup({ name: 'Bugs', description: 'd' });
    expect(root.textContent).toContain('Actualizar el filtro actual');
    root.querySelector<HTMLButtonElement>('[data-testid="filter-save"] button')?.click();
    expect(saved[0]).toEqual({ name: 'Bugs', description: 'd', replace: true });

    const copy = Array.from(root.querySelectorAll<HTMLInputElement>('input[type="radio"]'))[1];
    copy.click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(root.querySelector<HTMLInputElement>('[data-testid="filter-name"]')?.value).toBe(
      'Bugs (copia)',
    );
  });
});

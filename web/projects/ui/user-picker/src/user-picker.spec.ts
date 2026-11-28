import { ComponentFixture, TestBed } from '@angular/core/testing';
import { describe, expect, it, vi } from 'vitest';

import { PickableUser, UserPicker } from './user-picker';

const users: PickableUser[] = [
  { id: 'u1', name: 'Ana Torres', email: 'ana@example.test' },
  { id: 'u2', name: 'Luis Gómez' },
];

function create(
  inputs: Partial<{
    users: PickableUser[];
    value: string;
    placeholder: string;
    loading: boolean;
    disabled: boolean;
    allowClear: boolean;
  }> = {},
): { fixture: ComponentFixture<UserPicker>; el: HTMLElement } {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(UserPicker);
  fixture.componentRef.setInput('users', inputs.users ?? users);
  for (const [key, value] of Object.entries(inputs)) {
    if (key !== 'users') {
      fixture.componentRef.setInput(key, value);
    }
  }
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

function field(el: HTMLElement): HTMLInputElement {
  return el.querySelector('input') as HTMLInputElement;
}

function press(fixture: ComponentFixture<UserPicker>, key: string): KeyboardEvent {
  const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true });
  field(fixture.nativeElement as HTMLElement).dispatchEvent(event);
  fixture.detectChanges();
  return event;
}

function typeText(fixture: ComponentFixture<UserPicker>, text: string): void {
  const input = field(fixture.nativeElement as HTMLElement);
  input.value = text;
  input.dispatchEvent(new Event('input'));
  fixture.detectChanges();
}

// The list lives in the overlay container, so the whole document is the search root.
function options(_el: HTMLElement): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>('[role="option"]'));
}

describe('UserPicker', () => {
  it('shows the selected user name with its avatar', () => {
    const { el } = create({ value: 'u1' });
    expect(field(el).value).toBe('Ana Torres');
    expect(el.querySelector('.user-picker__selected-avatar app-avatar')).not.toBeNull();
  });

  it('renders the placeholder and a named combobox when nothing is selected', () => {
    const { el } = create({ users: [], placeholder: 'Elegir responsable' });
    expect(field(el).placeholder).toBe('Elegir responsable');
    expect(field(el).getAttribute('role')).toBe('combobox');
    expect(field(el).getAttribute('aria-label')).toBe('Elegir responsable');
  });

  it('gives the toggle button an accessible name', () => {
    const { el } = create();
    expect(el.querySelector('button[aria-label="Mostrar personas"]')).not.toBeNull();
  });

  it('shows avatar, name and email in every option', () => {
    const { fixture, el } = create();
    press(fixture, 'ArrowDown');
    const [first, second] = options(el);
    expect(first.querySelector('app-avatar')).not.toBeNull();
    expect(first.textContent).toContain('Ana Torres');
    expect(first.textContent).toContain('ana@example.test');
    expect(second.querySelector('.user-picker__option-email')).toBeNull();
  });

  it('emits the typed text and narrows the options by name or email', () => {
    const { fixture, el } = create();
    const onSearch = vi.fn();
    fixture.componentInstance.search.subscribe(onSearch);

    typeText(fixture, 'luis');
    expect(onSearch).toHaveBeenLastCalledWith('luis');
    expect(options(el).map((o) => o.textContent)).toEqual([expect.stringContaining('Luis Gómez')]);

    typeText(fixture, 'ANA@');
    expect(options(el).map((o) => o.textContent)).toEqual([expect.stringContaining('Ana Torres')]);
  });

  it('opens the list with a click on the field', () => {
    const { fixture, el } = create();
    field(el).click();
    fixture.detectChanges();
    expect(options(el)).toHaveLength(2);
  });

  it('moves with the arrows and chooses with Enter', () => {
    const { fixture, el } = create();
    press(fixture, 'ArrowDown');
    expect(field(el).getAttribute('aria-expanded')).toBe('true');
    press(fixture, 'ArrowDown');
    expect(field(el).getAttribute('aria-activedescendant')).toBe(options(el)[1].id);
    press(fixture, 'Enter');
    expect(fixture.componentInstance.value()).toBe('u2');
    expect(options(el)).toHaveLength(0);
    expect(field(el).value).toBe('Luis Gómez');
  });

  it('closes the list with Escape without letting the key reach a surrounding dialog', () => {
    const { fixture, el } = create();
    press(fixture, 'ArrowDown');
    const event = press(fixture, 'Escape');
    expect(event.defaultPrevented).toBe(true);
    expect(options(el)).toHaveLength(0);
  });

  it('chooses an option with a click', () => {
    const { fixture, el } = create();
    press(fixture, 'ArrowDown');
    options(el)[0].click();
    fixture.detectChanges();
    expect(fixture.componentInstance.value()).toBe('u1');
  });

  it('does not select typed text without a matching option', () => {
    const { fixture, el } = create({ value: 'u1' });
    typeText(fixture, 'zzz');
    expect(document.body.textContent).toContain('Sin resultados');
    press(fixture, 'Enter');
    expect(fixture.componentInstance.value()).toBe('u1');
    press(fixture, 'Tab');
    expect(field(el).value).toBe('Ana Torres');
  });

  it('shows a loading state while the parent searches', () => {
    const { fixture, el } = create({ loading: true });
    press(fixture, 'ArrowDown');
    expect(document.querySelector('[role="status"]')?.textContent).toContain('Buscando personas');
    expect(field(el).getAttribute('aria-busy')).toBe('true');
  });

  it('clears the selection from the clear button and hides it when not allowed', () => {
    const { fixture, el } = create({ value: 'u1' });
    (el.querySelector('button[aria-label="Quitar selección"]') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(fixture.componentInstance.value()).toBeUndefined();
    expect(field(el).value).toBe('');

    TestBed.resetTestingModule();
    const locked = create({ value: 'u1', allowClear: false });
    expect(locked.el.querySelector('button[aria-label="Quitar selección"]')).toBeNull();
  });

  it('disables the field and its buttons', () => {
    const { el } = create({ disabled: true });
    expect(field(el).disabled).toBe(true);
    expect(
      (el.querySelector('button[aria-label="Mostrar personas"]') as HTMLButtonElement).disabled,
    ).toBe(true);
  });
});

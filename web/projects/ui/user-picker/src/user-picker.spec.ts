import { TestBed } from '@angular/core/testing';
import { describe, expect, it, vi } from 'vitest';

import { PickableUser, UserPicker } from './user-picker';

const users: PickableUser[] = [{ id: 'u1', name: 'Ana Torres' }];

function create(inputs: {
  users: PickableUser[];
  value?: string;
  placeholder?: string;
  onSearch?: (term: string) => void;
}): HTMLElement {
  TestBed.configureTestingModule({});
  const fixture = TestBed.createComponent(UserPicker);
  fixture.componentRef.setInput('users', inputs.users);
  if (inputs.value !== undefined) {
    fixture.componentRef.setInput('value', inputs.value);
  }
  if (inputs.placeholder !== undefined) {
    fixture.componentRef.setInput('placeholder', inputs.placeholder);
  }
  if (inputs.onSearch !== undefined) {
    fixture.componentInstance.search.subscribe(inputs.onSearch);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('UserPicker', () => {
  it('shows the selected user name', async () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(UserPicker);
    fixture.componentRef.setInput('users', users);
    fixture.componentRef.setInput('value', 'u1');
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const input = (fixture.nativeElement as HTMLElement).querySelector('input');
    expect(input?.value).toBe('Ana Torres');
  });

  it('renders the placeholder when nothing is selected', () => {
    const el = create({ users: [], placeholder: 'Elegir responsable' });
    expect(el.querySelector('input')?.placeholder).toBe('Elegir responsable');
  });

  it('shows the email next to the name in the options when the user has one', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(UserPicker);
    fixture.componentRef.setInput('users', [
      { id: 'u1', name: 'Ana Torres', email: 'ana@example.test' },
      { id: 'u2', name: 'Luis Gómez' },
    ]);
    fixture.detectChanges();
    const component = fixture.componentInstance;

    expect(component['options']().map((option) => option.label)).toEqual([
      'Ana Torres (ana@example.test)',
      'Luis Gómez',
    ]);
  });

  it('disables the control when the disabled input is set', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(UserPicker);
    fixture.componentRef.setInput('users', users);
    fixture.componentRef.setInput('disabled', true);
    fixture.detectChanges();

    const input = (fixture.nativeElement as HTMLElement).querySelector('input');
    expect(input?.disabled).toBe(true);
  });

  it('narrows the suggestions to the typed text, by name or email', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(UserPicker);
    fixture.componentRef.setInput('users', [
      { id: 'u1', name: 'Ana Torres', email: 'ana@example.test' },
      { id: 'u2', name: 'Luis Gómez' },
    ]);
    fixture.detectChanges();
    const component = fixture.componentInstance;
    component['onComplete']('luis');
    expect(component['suggestions']().map((option) => option.id)).toEqual(['u2']);
    component['onComplete']('ANA@');
    expect(component['suggestions']().map((option) => option.id)).toEqual(['u1']);
  });

  it('sets the value only from a suggestion; typed text without one clears it', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(UserPicker);
    fixture.componentRef.setInput('users', users);
    fixture.detectChanges();
    const component = fixture.componentInstance;
    component['onSelect']({
      id: 'u1',
      name: 'Ana Torres',
      label: 'Ana Torres',
      initials: 'AT',
      searchText: 'Ana Torres',
    });
    expect(component.value()).toBe('u1');
    component['onSelect']('texto libre');
    expect(component.value()).toBeUndefined();
  });

  it('accepts a search callback without calling it on render', () => {
    const onSearch = vi.fn();
    create({ users, onSearch });
    expect(onSearch).not.toHaveBeenCalled();
    TestBed.resetTestingModule();
  });
});

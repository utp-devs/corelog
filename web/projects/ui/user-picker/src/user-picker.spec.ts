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
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('Ana Torres');
  });

  it('renders the placeholder when nothing is selected', () => {
    const el = create({ users: [], placeholder: 'Elegir responsable' });
    expect(el.textContent).toContain('Elegir responsable');
  });

  it('accepts a search callback without calling it on render', () => {
    const onSearch = vi.fn();
    create({ users, onSearch });
    expect(onSearch).not.toHaveBeenCalled();
    TestBed.resetTestingModule();
  });
});

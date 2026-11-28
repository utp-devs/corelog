import { TestBed } from '@angular/core/testing';

import { FieldView, formatFieldDate } from './field-view';

const OPTIONS = [
  { id: 'a', label: 'Alta' },
  { id: 'b', label: 'Baja' },
];

function render(inputs: Record<string, unknown>): HTMLElement {
  const fixture = TestBed.createComponent(FieldView);
  for (const [key, value] of Object.entries(inputs)) fixture.componentRef.setInput(key, value);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('FieldView', () => {
  it('says there is no value for empty values', () => {
    expect(render({ type: 'text', value: null }).textContent).toContain('Sin valor');
    expect(render({ type: 'multiselect', value: [] }).textContent).toContain('Sin valor');
  });

  it('shows the label of a selected option and chips for multiselect', () => {
    expect(render({ type: 'select', value: 'b', options: OPTIONS }).textContent).toContain('Baja');
    const chips = render({ type: 'multiselect', value: ['a', 'b'], options: OPTIONS });
    expect(Array.from(chips.querySelectorAll('li')).map((li) => li.textContent?.trim())).toEqual([
      'Alta',
      'Baja',
    ]);
  });

  it('shows yes or no for a checkbox, even when unchecked', () => {
    expect(render({ type: 'checkbox', value: false }).textContent).toContain('No');
    expect(render({ type: 'checkbox', value: true }).textContent).toContain('Sí');
  });

  it('formats dates and resolves users by name', () => {
    expect(render({ type: 'date', value: '2026-11-27' }).textContent).toContain('27/11/2026');
    const el = render({ type: 'user', value: 'u1', userName: () => 'Ana Torres' });
    expect(el.textContent).toContain('Ana Torres');
  });

  it('renders only http links as anchors', () => {
    expect(render({ type: 'url', value: 'https://example.com' }).querySelector('a')).not.toBeNull();
    expect(render({ type: 'url', value: 'javascript:alert(1)' }).querySelector('a')).toBeNull();
  });
});

describe('formatFieldDate', () => {
  it('writes day first, with the time for datetimes', () => {
    expect(formatFieldDate('2026-01-05', false)).toBe('05/01/2026');
    expect(formatFieldDate('2026-01-05T08:09:00', true)).toBe('05/01/2026 08:09');
  });

  it('returns unreadable text unchanged', () => {
    expect(formatFieldDate('pronto', false)).toBe('pronto');
  });
});

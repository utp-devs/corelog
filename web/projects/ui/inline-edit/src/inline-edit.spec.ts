import { ComponentFixture, TestBed } from '@angular/core/testing';

import { INLINE_EDIT_SAVE_ERROR, InlineEdit, InlineEditType } from './inline-edit';

interface Setup {
  fixture: ComponentFixture<InlineEdit>;
  el: HTMLElement;
}

function create(inputs: {
  label: string;
  value: string | null;
  type?: InlineEditType;
  options?: { value: string; label: string }[];
  onSave?: (next: string | null) => void | Promise<void>;
}): Setup {
  const fixture = TestBed.createComponent(InlineEdit);
  fixture.componentRef.setInput('label', inputs.label);
  fixture.componentRef.setInput('value', inputs.value);
  if (inputs.type) fixture.componentRef.setInput('type', inputs.type);
  if (inputs.options) fixture.componentRef.setInput('options', inputs.options);
  if (inputs.onSave) fixture.componentRef.setInput('onSave', inputs.onSave);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

function openEditor(setup: Setup, label: string, text: string): void {
  (
    setup.el.querySelector(`button[aria-label="Editar ${label}: ${text}"]`) as HTMLButtonElement
  ).click();
  setup.fixture.detectChanges();
}

function editor(setup: Setup, label: string): HTMLInputElement {
  return setup.el.querySelector(`[aria-label="${label}"]`) as HTMLInputElement;
}

function keydown(setup: Setup, target: HTMLElement, key: string): void {
  target.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }));
  setup.fixture.detectChanges();
}

function type(setup: Setup, target: HTMLInputElement, text: string): void {
  target.value = text;
  target.dispatchEvent(new Event('input'));
  setup.fixture.detectChanges();
}

function focusOut(setup: Setup, target: HTMLElement): void {
  target.dispatchEvent(new FocusEvent('focusout', { bubbles: true }));
  setup.fixture.detectChanges();
}

async function settle(setup: Setup): Promise<void> {
  for (let i = 0; i < 3; i++) {
    await Promise.resolve();
  }
  setup.fixture.detectChanges();
}

describe('InlineEdit', () => {
  beforeEach(() => {
    TestBed.configureTestingModule({});
  });

  it('shows the value and opens the editor on click', () => {
    const setup = create({ label: 'Título', value: 'Login roto' });
    openEditor(setup, 'Título', 'Login roto');
    expect(editor(setup, 'Título').value).toBe('Login roto');
  });

  it('saves the new value on Enter', () => {
    const onSave = vi.fn();
    const setup = create({ label: 'Título', value: 'Viejo', onSave });
    openEditor(setup, 'Título', 'Viejo');
    const input = editor(setup, 'Título');
    type(setup, input, 'Nuevo');
    keydown(setup, input, 'Enter');
    expect(onSave).toHaveBeenCalledWith('Nuevo');
    expect(onSave).toHaveBeenCalledTimes(1);
  });

  it('cancels on Escape without saving and restores the value', () => {
    const onSave = vi.fn();
    const setup = create({ label: 'Título', value: 'Viejo', onSave });
    openEditor(setup, 'Título', 'Viejo');
    const input = editor(setup, 'Título');
    type(setup, input, 'Cambio descartado');
    keydown(setup, input, 'Escape');
    expect(onSave).not.toHaveBeenCalled();
    expect(editor(setup, 'Título')).toBeFalsy();
    expect(setup.el.querySelector('button[aria-label="Editar Título: Viejo"]')).not.toBeNull();
  });

  it('saves on blur, and does not save twice when Enter is followed by blur', () => {
    const onSave = vi.fn();
    const first = create({ label: 'Título', value: 'Viejo', onSave });
    openEditor(first, 'Título', 'Viejo');
    const input = editor(first, 'Título');
    type(first, input, 'Por blur');
    focusOut(first, input.parentElement as HTMLElement);
    expect(onSave).toHaveBeenCalledWith('Por blur');

    const second = create({ label: 'Estado', value: 'A', onSave });
    openEditor(second, 'Estado', 'A');
    const field = editor(second, 'Estado');
    type(second, field, 'B');
    keydown(second, field, 'Enter');
    focusOut(second, field.parentElement as HTMLElement);
    expect(onSave).toHaveBeenCalledTimes(2);
  });

  it('does not save when the value is unchanged', () => {
    const onSave = vi.fn();
    const setup = create({ label: 'Título', value: 'Igual', onSave });
    openEditor(setup, 'Título', 'Igual');
    keydown(setup, editor(setup, 'Título'), 'Enter');
    expect(onSave).not.toHaveBeenCalled();
  });

  it('shows the message of the error thrown by the save', async () => {
    const onSave = vi.fn(() =>
      Promise.reject(new Error('Ya existe una incidencia con ese título.')),
    );
    const setup = create({ label: 'Título', value: 'Viejo', onSave });
    openEditor(setup, 'Título', 'Viejo');
    const input = editor(setup, 'Título');
    type(setup, input, 'Nuevo');
    keydown(setup, input, 'Enter');
    await settle(setup);

    const alert = setup.el.querySelector('[role="alert"]') as HTMLElement;
    expect(alert.textContent).toContain('Ya existe una incidencia con ese título.');
    expect(alert.textContent).not.toContain(INLINE_EDIT_SAVE_ERROR);
  });

  it('shows the Spanish fallback when the error has no message', async () => {
    const onSave = vi.fn(() => Promise.reject(new Error('')));
    const setup = create({ label: 'Título', value: 'Viejo', onSave });
    openEditor(setup, 'Título', 'Viejo');
    const input = editor(setup, 'Título');
    type(setup, input, 'Nuevo');
    keydown(setup, input, 'Enter');
    await settle(setup);

    const alert = setup.el.querySelector('[role="alert"]') as HTMLElement;
    expect(alert.textContent).toContain(INLINE_EDIT_SAVE_ERROR);
  });

  it('shows a pending state while saving and the Spanish error when saving fails', async () => {
    let reject: (reason: Error) => void = () => {};
    const onSave = vi.fn(
      () =>
        new Promise<void>((_resolve, rej) => {
          reject = rej;
        }),
    );
    const setup = create({ label: 'Título', value: 'Viejo', onSave });
    openEditor(setup, 'Título', 'Viejo');
    const input = editor(setup, 'Título');
    type(setup, input, 'Nuevo');
    keydown(setup, input, 'Enter');

    const status = setup.el.querySelector('[role="status"]') as HTMLElement;
    expect(status.textContent).toContain('Guardando');
    expect(
      (setup.el.querySelector('button.inline-edit__trigger') as HTMLButtonElement).disabled,
    ).toBe(true);

    reject(new Error(''));
    await settle(setup);
    const alert = setup.el.querySelector('[role="alert"]') as HTMLElement;
    expect(alert.textContent).toContain(INLINE_EDIT_SAVE_ERROR);
    expect(setup.el.querySelector('[role="status"]')).toBeNull();
    expect(setup.el.querySelector('button[aria-label="Editar Título: Viejo"]')).not.toBeNull();
  });

  it('renders hostile text values as text', () => {
    const setup = create({ label: 'Título', value: '<script>alert(1)</script>' });
    expect(setup.el.querySelector('script')).toBeNull();
    expect(setup.el.textContent).toContain('<script>alert(1)</script>');
  });

  it('shows an ISO date as dd/MM/yyyy and edits it with the date field', () => {
    const onSave = vi.fn();
    const setup = create({ label: 'Vence', type: 'date', value: '2026-12-01', onSave });
    openEditor(setup, 'Vence', '01/12/2026');
    expect(setup.el.querySelector('app-date-field')).not.toBeNull();
    expect(setup.el.querySelector('input[type="date"]')).toBeNull();

    const field = setup.fixture.debugElement.query((el) => el.name === 'app-date-field');
    field.triggerEventHandler('ngModelChange', '2026-12-15');
    setup.fixture.detectChanges();
    expect(onSave).toHaveBeenCalledWith('2026-12-15');
  });

  it('ends a date edit without saving when the calendar closes with no change', () => {
    const onSave = vi.fn();
    const setup = create({ label: 'Vence', type: 'date', value: '2026-12-01', onSave });
    openEditor(setup, 'Vence', '01/12/2026');
    const field = setup.fixture.debugElement.query((el) => el.name === 'app-date-field');
    field.triggerEventHandler('closed', undefined);
    setup.fixture.detectChanges();
    expect(onSave).not.toHaveBeenCalled();
    expect(setup.el.querySelector('app-date-field')).toBeNull();
  });

  it('shows a pencil for plain text and a chevron for lists', () => {
    const text = create({ label: 'Título', value: 'Hola' });
    expect(text.el.querySelector('.inline-edit__icon--pencil')).not.toBeNull();

    const list = create({
      label: 'Estado',
      type: 'select',
      value: 'a',
      options: [{ value: 'a', label: 'Abierto' }],
    });
    expect(list.el.querySelector('.pi-chevron-down')).not.toBeNull();
    expect(list.el.querySelector('.inline-edit__icon--pencil')).toBeNull();
  });

  it('renders the field and lozenge appearances with their tone', () => {
    const setup = create({
      label: 'Estado',
      type: 'select',
      value: 'a',
      options: [{ value: 'a', label: 'En curso' }],
    });
    setup.fixture.componentRef.setInput('appearance', 'lozenge');
    setup.fixture.componentRef.setInput('tone', 'progress');
    setup.fixture.detectChanges();
    const trigger = setup.el.querySelector('.inline-edit__trigger') as HTMLElement;
    expect(trigger.classList).toContain('inline-edit__trigger--lozenge');
    expect(trigger.classList).toContain('inline-edit__trigger--progress');

    setup.fixture.componentRef.setInput('appearance', 'field');
    setup.fixture.detectChanges();
    expect(trigger.classList).toContain('inline-edit__trigger--field');
  });

  it('explains why it cannot be edited when disabled', () => {
    const setup = create({ label: 'Título', value: 'Hola' });
    setup.fixture.componentRef.setInput('disabledReason', 'No tienes permiso para editar.');
    setup.fixture.componentInstance.setDisabledState(true);
    setup.fixture.detectChanges();
    const trigger = setup.el.querySelector('.inline-edit__trigger') as HTMLButtonElement;
    expect(trigger.disabled).toBe(true);
    expect(trigger.getAttribute('title')).toBe('No tienes permiso para editar.');
  });

  it('shows the placeholder for an empty value and does not save on Escape in the select editor', () => {
    const onSave = vi.fn();
    const setup = create({
      label: 'Responsable',
      type: 'select',
      value: null,
      options: [{ value: 'u1', label: 'Ana' }],
      onSave,
    });
    (
      setup.el.querySelector(
        'button[aria-label="Editar Responsable: Sin valor"]',
      ) as HTMLButtonElement
    ).click();
    setup.fixture.detectChanges();
    keydown(setup, setup.el.querySelector('.inline-edit__editor') as HTMLElement, 'Escape');
    expect(onSave).not.toHaveBeenCalled();
  });
});

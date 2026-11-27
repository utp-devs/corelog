import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';

import { RichTextEditor, type IssueLite, type UserLite } from './rich-text-editor';
import { draftStorageKey } from './draft';

const UID = '3f2a9c1e-7b4d-4e8a-9c2f-1a2b3c4d5e6f';
const UID2 = '00000000-0000-4000-8000-000000000002';

@Component({
  imports: [RichTextEditor, ReactiveFormsModule],
  template: `
    <app-rich-text-editor
      [formControl]="control"
      ariaLabel="Descripción"
      [searchUsers]="searchUsers()"
      [searchIssues]="searchIssues()"
      [uploadImage]="uploadImage()"
      [draftKey]="draftKey()"
      [submittable]="submittable()"
      [minRows]="minRows()"
      [maxRows]="maxRows()"
      (submitted)="submitCount = submitCount + 1"
    />
  `,
})
class HostComponent {
  readonly control = new FormControl('', { nonNullable: true });
  readonly searchUsers = signal<((query: string) => Promise<UserLite[]>) | undefined>(undefined);
  readonly searchIssues = signal<((query: string) => Promise<IssueLite[]>) | undefined>(undefined);
  readonly uploadImage = signal<((file: File) => Promise<{ url: string }>) | undefined>(undefined);
  readonly draftKey = signal<string | undefined>(undefined);
  readonly submittable = signal(false);
  readonly minRows = signal(4);
  readonly maxRows = signal(16);
  submitCount = 0;
}

interface Setup {
  fixture: ComponentFixture<HostComponent>;
  host: HostComponent;
  el: HTMLElement;
}

function setup(
  options: {
    submittable?: boolean;
    minRows?: number;
    maxRows?: number;
    initial?: string;
    searchUsers?: (query: string) => Promise<UserLite[]>;
    searchIssues?: (query: string) => Promise<IssueLite[]>;
    uploadImage?: (file: File) => Promise<{ url: string }>;
    draftKey?: string;
  } = {},
): Setup {
  TestBed.resetTestingModule();
  TestBed.configureTestingModule({ imports: [HostComponent] });
  const fixture = TestBed.createComponent(HostComponent);
  const host = fixture.componentInstance;
  if (options.initial !== undefined) host.control.setValue(options.initial);
  if (options.submittable !== undefined) host.submittable.set(options.submittable);
  if (options.minRows !== undefined) host.minRows.set(options.minRows);
  if (options.maxRows !== undefined) host.maxRows.set(options.maxRows);
  if (options.searchUsers) host.searchUsers.set(options.searchUsers);
  if (options.searchIssues) host.searchIssues.set(options.searchIssues);
  if (options.uploadImage) host.uploadImage.set(options.uploadImage);
  if (options.draftKey) host.draftKey.set(options.draftKey);
  fixture.detectChanges();
  return { fixture, host, el: fixture.nativeElement as HTMLElement };
}

function textarea(el: HTMLElement): HTMLTextAreaElement {
  return el.querySelector('textarea') as HTMLTextAreaElement;
}

function button(el: HTMLElement, label: string): HTMLButtonElement {
  const found = Array.from(el.querySelectorAll('button')).find(
    (b) => b.getAttribute('aria-label') === label || b.textContent?.trim() === label,
  );
  if (!found) throw new Error(`button not found: ${label}`);
  return found;
}

function options(el: HTMLElement): HTMLElement[] {
  return Array.from(el.querySelectorAll<HTMLElement>('[role="option"]'));
}

function typeText(fixture: ComponentFixture<HostComponent>, el: HTMLElement, value: string): void {
  const area = textarea(el);
  area.value = value;
  area.dispatchEvent(new Event('input', { bubbles: true }));
  fixture.detectChanges();
}

function keydown(
  fixture: ComponentFixture<HostComponent>,
  target: HTMLElement,
  init: KeyboardEventInit,
): void {
  target.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }));
  fixture.detectChanges();
}

const sleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/** Polls until the check passes, running change detection between attempts. */
async function until(
  fixture: ComponentFixture<HostComponent>,
  check: () => boolean,
  timeout = 1500,
): Promise<void> {
  const start = Date.now();
  fixture.detectChanges();
  while (!check()) {
    if (Date.now() - start > timeout) throw new Error('condition not met in time');
    await sleep(20);
    fixture.detectChanges();
  }
}

afterEach(() => {
  window.localStorage.clear();
});

describe('RichTextEditor toolbar groups', () => {
  it('groups the tools and names every icon-only button', () => {
    const { el } = setup({ initial: '' });
    const groups = Array.from(el.querySelectorAll('[role="group"]')).map((g) =>
      g.getAttribute('aria-label'),
    );
    expect(groups).toEqual(['Encabezados', 'Texto', 'Listas', 'Bloques', 'Insertar']);
    const tools = Array.from(el.querySelectorAll<HTMLButtonElement>('.rich-text-editor__tool'));
    expect(tools.length).toBeGreaterThan(10);
    expect(tools.every((tool) => !!tool.getAttribute('aria-label'))).toBe(true);
  });

  it('offers the image button only when uploads are enabled', () => {
    const without = setup({ initial: '' });
    expect(without.el.querySelector('button[aria-label="Subir imagen"]')).toBeNull();
  });

  it('shows the submit hint only when submittable', () => {
    const plain = setup({ initial: '' });
    expect(plain.el.querySelector('.rich-text-editor__hint')).toBeNull();
    const submittable = setup({ initial: '', submittable: true });
    expect(submittable.el.querySelector('.rich-text-editor__hint')?.textContent).toContain('Enter');
  });
});

describe('RichTextEditor toolbar', () => {
  it('wraps the selection with bold markers', () => {
    const { fixture, el } = setup({ initial: 'hola mundo' });
    const area = textarea(el);
    area.setSelectionRange(5, 10);
    button(el, 'Negrita').click();
    fixture.detectChanges();
    expect(area.value).toBe('hola **mundo**');
  });

  it('inserts a bullet list prefix on the current line', () => {
    const { fixture, el } = setup({ initial: 'uno' });
    const area = textarea(el);
    area.setSelectionRange(0, 0);
    button(el, 'Lista con viñetas').click();
    fixture.detectChanges();
    expect(area.value).toBe('- uno');
  });

  it('inserts a mention trigger and opens the user picker', async () => {
    const searchUsers = vi.fn().mockResolvedValue([{ id: UID, name: 'Ana Pérez' }]);
    const { fixture, el } = setup({ searchUsers });
    button(el, 'Mencionar persona').click();
    fixture.detectChanges();
    expect(textarea(el).value).toBe('@');
    await until(fixture, () => options(el).some((o) => /Ana Pérez/.test(o.textContent ?? '')));
    expect(searchUsers).toHaveBeenCalledWith('');
  });

  it('switches to preview and renders the Markdown', () => {
    const { fixture, el } = setup({ initial: '**fuerte**' });
    button(el, 'Vista previa').click();
    fixture.detectChanges();
    const region = el.querySelector('[role="region"][aria-label="Vista previa"]') as HTMLElement;
    expect(region.querySelector('strong')?.textContent).toBe('fuerte');
    expect(button(el, 'Vista previa').getAttribute('aria-pressed')).toBe('true');
  });
});

describe('RichTextEditor submit', () => {
  it('calls submit on Ctrl+Enter', () => {
    const { fixture, host, el } = setup({ initial: 'texto', submittable: true });
    keydown(fixture, textarea(el), { key: 'Enter', ctrlKey: true });
    expect(host.submitCount).toBe(1);
  });

  it('does not submit on plain Enter', () => {
    const { fixture, host, el } = setup({ initial: 'texto', submittable: true });
    keydown(fixture, textarea(el), { key: 'Enter' });
    expect(host.submitCount).toBe(0);
  });
});

describe('RichTextEditor mention picker', () => {
  it('navigates with arrows, selects with Enter and inserts the token', async () => {
    const searchUsers = vi.fn().mockResolvedValue([
      { id: UID, name: 'Ana Pérez' },
      { id: UID2, name: 'Luis Gómez' },
    ]);
    const { fixture, el } = setup({ searchUsers });
    typeText(fixture, el, 'Hola @an');
    await until(fixture, () => options(el).length === 2);
    const area = textarea(el);

    expect(options(el)[0].getAttribute('aria-selected')).toBe('true');
    keydown(fixture, area, { key: 'ArrowDown' });
    expect(options(el)[1].getAttribute('aria-selected')).toBe('true');
    keydown(fixture, area, { key: 'ArrowDown' });
    expect(options(el)[0].getAttribute('aria-selected')).toBe('true');
    keydown(fixture, area, { key: 'ArrowDown' });

    keydown(fixture, area, { key: 'Enter' });
    expect(area.value).toBe(`Hola @[Luis Gómez](user:${UID2}) `);
    expect(el.querySelector('[role="listbox"]')).toBeNull();
  });

  it('closes with Escape without changing the text', async () => {
    const searchUsers = vi.fn().mockResolvedValue([{ id: UID, name: 'Ana' }]);
    const { fixture, el } = setup({ searchUsers });
    typeText(fixture, el, '@a');
    await until(fixture, () => el.querySelector('[role="listbox"]') !== null);
    keydown(fixture, textarea(el), { key: 'Escape' });
    expect(el.querySelector('[role="listbox"]')).toBeNull();
    expect(textarea(el).value).toBe('@a');
  });

  it('opens the issue picker on a key prefix and inserts the key', async () => {
    const searchIssues = vi.fn().mockResolvedValue([{ key: 'CL-12', summary: 'Error al guardar' }]);
    const { fixture, el } = setup({ searchIssues });
    typeText(fixture, el, 'ver CL-1');
    await until(fixture, () => options(el).some((o) => /CL-12/.test(o.textContent ?? '')));
    expect(searchIssues).toHaveBeenCalledWith('CL-1');
    options(el)[0].dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true }));
    fixture.detectChanges();
    expect(textarea(el).value).toBe('ver CL-12 ');
  });
});

describe('RichTextEditor image upload', () => {
  it('uploads a pasted image and inserts its URL', async () => {
    const uploadImage = vi.fn().mockResolvedValue({ url: 'https://cdn.example.com/a.png' });
    const { fixture, el } = setup({ initial: 'antes ', uploadImage });
    const area = textarea(el);
    area.setSelectionRange(6, 6);
    const file = new File(['png'], 'captura.png', { type: 'image/png' });
    const paste = new Event('paste', { bubbles: true, cancelable: true });
    Object.defineProperty(paste, 'clipboardData', { value: { files: [file], items: [] } });
    area.dispatchEvent(paste);
    await until(fixture, () => area.value === 'antes ![](https://cdn.example.com/a.png)\n');
    expect(uploadImage).toHaveBeenCalledWith(file);
  });

  it('rejects files that are not accepted images', () => {
    const uploadImage = vi.fn();
    const { fixture, el } = setup({ uploadImage });
    const input = el.querySelector('input[type="file"]') as HTMLInputElement;
    Object.defineProperty(input, 'files', {
      value: [new File(['x'], 'a.txt', { type: 'text/plain' })],
      configurable: true,
    });
    input.dispatchEvent(new Event('change'));
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Formato no admitido');
    expect(uploadImage).not.toHaveBeenCalled();
  });

  it('shows an error when the upload fails', async () => {
    const uploadImage = vi.fn().mockRejectedValue(new Error('network'));
    const { fixture, el } = setup({ uploadImage });
    const input = el.querySelector('input[type="file"]') as HTMLInputElement;
    Object.defineProperty(input, 'files', {
      value: [new File(['png'], 'a.png', { type: 'image/png' })],
      configurable: true,
    });
    input.dispatchEvent(new Event('change'));
    await until(
      fixture,
      () =>
        el.querySelector('[role="alert"]')?.textContent?.includes('No se pudo subir la imagen') ===
        true,
    );
  });
});

describe('RichTextEditor draft', () => {
  it('saves the draft after edits, restores it on mount and clears it on submit', async () => {
    const key = 'issue-desc-1';
    const first = setup({ draftKey: key, submittable: true });
    typeText(first.fixture, first.el, 'borrador');
    await until(
      first.fixture,
      () => window.localStorage.getItem(draftStorageKey(key)) === 'borrador',
    );
    first.fixture.destroy();

    const restored = setup({ draftKey: key, submittable: true });
    await until(restored.fixture, () => textarea(restored.el).value === 'borrador');
    keydown(restored.fixture, textarea(restored.el), { key: 'Enter', ctrlKey: true });
    expect(window.localStorage.getItem(draftStorageKey(key))).toBeNull();
    restored.fixture.destroy();
  });

  it('removes the draft when the field is emptied', async () => {
    const key = 'issue-desc-2';
    window.localStorage.setItem(draftStorageKey(key), 'viejo');
    const { fixture, el } = setup({ draftKey: key });
    await until(fixture, () => textarea(el).value === 'viejo');
    typeText(fixture, el, '');
    await until(fixture, () => window.localStorage.getItem(draftStorageKey(key)) === null);
  });
});

describe('RichTextEditor disabled and autosize', () => {
  it('disables the textarea and toolbar', () => {
    const { fixture, host, el } = setup();
    host.control.disable();
    fixture.detectChanges();
    expect(textarea(el).disabled).toBe(true);
    expect(button(el, 'Negrita').disabled).toBe(true);
  });

  it('grows with content between the row limits', () => {
    const { el } = setup({ minRows: 2, maxRows: 6, initial: 'a\nb\nc' });
    expect(textarea(el).style.height).not.toBe('');
  });
});

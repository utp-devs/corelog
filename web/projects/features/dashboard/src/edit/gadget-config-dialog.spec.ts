import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import type { GadgetType } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';

import { GadgetConfigDialog, GadgetConfigValue } from './gadget-config-dialog';

function render(type: GadgetType, config: Record<string, unknown> = {}, title = '') {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  const fixture = TestBed.createComponent(GadgetConfigDialog);
  fixture.componentRef.setInput('open', true);
  fixture.componentRef.setInput('mode', 'add');
  fixture.componentRef.setInput('type', type);
  fixture.componentRef.setInput('title', title);
  fixture.componentRef.setInput('config', config);
  const applied: GadgetConfigValue[] = [];
  fixture.componentInstance.applied.subscribe((value) => applied.push(value));
  fixture.detectChanges();
  TestBed.flushEffects?.();
  fixture.detectChanges();
  return { fixture, applied, backend: TestBed.inject(HttpTestingController) };
}

interface Internals {
  form: { controls: Record<string, { setValue(v: unknown): void }> };
  submit(): void;
}

const internals = (fixture: { componentInstance: GadgetConfigDialog }) =>
  fixture.componentInstance as unknown as Internals;

describe('GadgetConfigDialog', () => {
  it('starts from the defaults of the gadget type and applies them', () => {
    const { fixture, applied } = render('assigned_to_me');
    expect(document.body.textContent).toContain('Añadir: Asignadas a mí');
    internals(fixture).submit();
    expect(applied).toHaveLength(1);
    expect(applied[0]).toEqual({
      title: 'Asignadas a mí',
      config: { limit: 10, include_done: false },
    });
  });

  it('rejects a number outside the limits and does not apply', () => {
    const { fixture, applied } = render('created_vs_resolved');
    internals(fixture).form.controls['days'].setValue(3);
    internals(fixture).submit();
    expect(applied).toEqual([]);
    fixture.detectChanges();
    expect(document.body.textContent).toContain('Entre 7 y 90.');
  });

  it('asks for a saved filter or a query on a filter gadget', () => {
    const { fixture, applied, backend } = render('filter_results', { query: '' });
    backend.match(() => true);
    internals(fixture).form.controls['query'].setValue('   ');
    internals(fixture).submit();
    expect(applied).toEqual([]);
    fixture.detectChanges();
    expect(document.body.textContent).toContain('Elige un filtro guardado o escribe una consulta.');

    internals(fixture).form.controls['query'].setValue('statusCategory != done');
    internals(fixture).submit();
    expect(applied[0].config).toMatchObject({ query: 'statusCategory != done' });
  });

  it('does not allow the same field on both axes of the two-dimensional gadget', () => {
    const { fixture, applied, backend } = render('two_dimensional');
    backend.match(() => true);
    internals(fixture).form.controls['y_axis'].setValue('status');
    internals(fixture).submit();
    expect(applied).toEqual([]);
  });

  it('keeps the text of a text gadget', () => {
    const { fixture, applied } = render('text', { markdown: 'Hola' }, 'Aviso');
    internals(fixture).submit();
    expect(applied[0]).toEqual({ title: 'Aviso', config: { markdown: 'Hola' } });
  });
});

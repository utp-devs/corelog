import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { ErrorPage, type ErrorPageVariant } from './error-page';

@Component({
  imports: [ErrorPage],
  template: `
    <app-error-page
      [variant]="variant()"
      [title]="title()"
      [description]="description()"
      [canRetry]="canRetry()"
      [framed]="framed()"
      (retry)="retries = retries + 1"
    />
  `,
})
class Host {
  readonly variant = signal<ErrorPageVariant>(404);
  readonly title = signal<string | undefined>(undefined);
  readonly description = signal<string | undefined>(undefined);
  readonly canRetry = signal(false);
  readonly framed = signal(false);
  retries = 0;
}

function create(variant: ErrorPageVariant): ComponentFixture<Host> {
  TestBed.configureTestingModule({ imports: [Host], providers: [provideRouter([])] });
  const fixture = TestBed.createComponent(Host);
  fixture.componentInstance.variant.set(variant);
  fixture.detectChanges();
  return fixture;
}

function el(fixture: ComponentFixture<Host>): HTMLElement {
  return fixture.nativeElement as HTMLElement;
}

function buttonNamed(root: HTMLElement, name: string): HTMLButtonElement | undefined {
  return Array.from(root.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === name,
  );
}

describe('ErrorPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('shows the Spanish 403 copy and the status code', () => {
    const root = el(create(403));
    expect(root.querySelector('h1')?.textContent).toBe('No tienes acceso a esta página');
    expect(root.querySelector('.error-page__code')?.textContent).toBe('403');
  });

  it('draws a bordered surface only when framed', () => {
    const fixture = create(404);
    expect(el(fixture).querySelector('.error-page--framed')).toBeNull();
    fixture.componentInstance.framed.set(true);
    fixture.detectChanges();
    expect(el(fixture).querySelector('.error-page--framed')).not.toBeNull();
    expect(el(fixture).querySelector('.error-page__body h1')).not.toBeNull();
  });

  it('shows the Spanish 404 copy', () => {
    const root = el(create(404));
    expect(root.querySelector('h1')?.textContent).toBe('No encontramos esta página');
    expect(root.querySelector('.error-page__code')?.textContent).toBe('404');
  });

  it('shows the Spanish 500 copy', () => {
    const root = el(create(500));
    expect(root.querySelector('h1')?.textContent).toBe('Algo salió mal');
    expect(root.querySelector('.error-page__code')?.textContent).toBe('500');
  });

  it('always offers "Volver al inicio" as a link to /', () => {
    const link = el(create(404)).querySelector('a');
    expect(link?.textContent?.trim()).toBe('Volver al inicio');
    expect(link?.getAttribute('href')).toBe('/');
  });

  it('offers a retry action on 500 when retry is enabled, and emits it', () => {
    const fixture = create(500);
    fixture.componentInstance.canRetry.set(true);
    fixture.detectChanges();
    buttonNamed(el(fixture), 'Reintentar')?.click();
    expect(fixture.componentInstance.retries).toBe(1);
  });

  it('does not offer retry on 403 or 404 even when retry is enabled', () => {
    const fixture = create(403);
    fixture.componentInstance.canRetry.set(true);
    fixture.detectChanges();
    expect(buttonNamed(el(fixture), 'Reintentar')).toBeUndefined();
  });

  it('does not offer retry on 500 without retry enabled', () => {
    expect(buttonNamed(el(create(500)), 'Reintentar')).toBeUndefined();
  });

  it('accepts custom title and description', () => {
    const fixture = create(404);
    fixture.componentInstance.title.set('Incidencia no encontrada');
    fixture.componentInstance.description.set('Puede que la hayan eliminado.');
    fixture.detectChanges();
    const root = el(fixture);
    expect(root.querySelector('h1')?.textContent).toBe('Incidencia no encontrada');
    expect(root.querySelector('.error-page__description')?.textContent).toBe(
      'Puede que la hayan eliminado.',
    );
  });
});

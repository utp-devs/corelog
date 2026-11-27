import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ErrorState } from './error-state';

@Component({
  imports: [ErrorState],
  template: `
    <app-error-state
      [message]="message()"
      [status]="status()"
      [canRetry]="canRetry()"
      (retry)="retries = retries + 1"
    />
  `,
})
class Host {
  readonly message = signal('Revisa tu conexión');
  readonly status = signal<number | null>(null);
  readonly canRetry = signal(false);
  retries = 0;
}

function create(): ComponentFixture<Host> {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return fixture;
}

function retryButton(el: HTMLElement): HTMLButtonElement | undefined {
  return Array.from(el.querySelectorAll('button')).find(
    (button) => button.textContent?.trim() === 'Reintentar',
  );
}

describe('ErrorState', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('announces the message as an alert with the default title', () => {
    const fixture = create();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('[role="alert"]')).not.toBeNull();
    expect(el.textContent).toContain('No se pudo cargar la información');
    expect(el.textContent).toContain('Revisa tu conexión');
  });

  it('replaces a raw English API message with friendly Spanish copy', () => {
    const fixture = create();
    fixture.componentInstance.message.set('Internal Server Error: request failed');
    fixture.componentInstance.status.set(500);
    fixture.detectChanges();
    const text = (fixture.nativeElement as HTMLElement).textContent ?? '';
    expect(text).not.toContain('Internal Server Error');
    expect(text).toContain('Algo salió mal');
  });

  it('shows the forbidden copy for a 403 and the connection copy for status 0', () => {
    const fixture = create();
    fixture.componentInstance.status.set(403);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('No tienes acceso');
    fixture.componentInstance.status.set(0);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('Sin conexión');
  });

  it('calls retry when the retry button is pressed', () => {
    const fixture = create();
    fixture.componentInstance.canRetry.set(true);
    fixture.detectChanges();
    retryButton(fixture.nativeElement as HTMLElement)?.click();
    expect(fixture.componentInstance.retries).toBe(1);
  });

  it('does not render a retry button without retry enabled', () => {
    const fixture = create();
    expect(retryButton(fixture.nativeElement as HTMLElement)).toBeUndefined();
  });
});

describe('ErrorState with a state photograph', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('draws the photograph instead of the icon tile', () => {
    TestBed.configureTestingModule({ imports: [ErrorState] });
    const fixture = TestBed.createComponent(ErrorState);
    fixture.componentRef.setInput('status', 500);
    fixture.componentRef.setInput('image', 'error-500');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('img')?.getAttribute('src')).toBe('/images/states/error-500.webp');
    expect(el.querySelector('.error-state__icon')).toBeNull();
  });
});

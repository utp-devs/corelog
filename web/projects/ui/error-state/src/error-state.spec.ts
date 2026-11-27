import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ErrorState } from './error-state';

@Component({
  imports: [ErrorState],
  template: `
    <app-error-state
      [message]="message()"
      [canRetry]="canRetry()"
      (retry)="retries = retries + 1"
    />
  `,
})
class Host {
  readonly message = signal('Revisa tu conexión');
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

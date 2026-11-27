import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { PasswordStrength } from './password-strength';

@Component({
  imports: [PasswordStrength],
  template: `<app-password-strength [password]="password()" />`,
})
class StrengthHost {
  readonly password = signal('');
}

function setup(): { el: HTMLElement; host: StrengthHost } {
  TestBed.configureTestingModule({ imports: [StrengthHost] });
  const fixture = TestBed.createComponent(StrengthHost);
  fixture.detectChanges();
  return { el: fixture.nativeElement as HTMLElement, host: fixture.componentInstance };
}

describe('PasswordStrength', () => {
  it('shows no message and no active segments for an empty password', () => {
    const { el } = setup();
    expect(el.querySelector('.password-strength__message')?.textContent?.trim()).toBe('');
    expect(el.querySelectorAll('.password-strength__segment--on').length).toBe(0);
  });

  it('announces the level in Spanish and lights the matching segments', () => {
    const { el, host } = setup();
    host.password.set('Secreta123!456');
    TestBed.tick();
    expect(el.querySelector('.password-strength__message')?.textContent?.trim()).toBe(
      'Seguridad de la contraseña: Fuerte',
    );
    expect(el.querySelector('[data-level]')?.getAttribute('data-level')).toBe('4');
    expect(el.querySelectorAll('.password-strength__segment--on').length).toBe(4);
  });

  it('reports a short password as weak with one segment', () => {
    const { el, host } = setup();
    host.password.set('abc');
    TestBed.tick();
    expect(el.querySelector('.password-strength__message')?.textContent?.trim()).toBe(
      'Seguridad de la contraseña: Débil',
    );
    expect(el.querySelectorAll('.password-strength__segment--on').length).toBe(1);
  });
});

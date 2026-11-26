import { ChangeDetectionStrategy, Component } from '@angular/core';

/** Placeholder sign-in page. The real form is pending. */
@Component({
  selector: 'app-login-page',
  template: `
    <main class="login">
      <h1>CoreLog</h1>
      <p>El inicio de sesión estará disponible próximamente.</p>
    </main>
  `,
  styles: `
    .login {
      min-height: 100vh;
      display: grid;
      place-content: center;
      gap: var(--cl-space-2);
      text-align: center;
      color: var(--cl-text);
      background: var(--cl-bg-page);
    }
    h1 {
      margin: 0;
      font-size: var(--cl-font-h1);
      color: var(--cl-brand);
    }
    p {
      margin: 0;
      color: var(--cl-text-subtle);
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class LoginPage {}

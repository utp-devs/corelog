import { ChangeDetectionStrategy, Component } from '@angular/core';

/** Shown for unknown paths inside an app. Placeholder for now. */
@Component({
  selector: 'app-not-found',
  template: `
    <section class="estado">
      <h1>Página no encontrada</h1>
      <p>La dirección que escribiste no existe.</p>
    </section>
  `,
  styles: `
    .estado {
      padding: var(--cl-space-10) var(--cl-space-4);
      text-align: center;
      color: var(--cl-text);
    }
    h1 {
      font-size: var(--cl-font-h2);
      margin: 0 0 var(--cl-space-2);
    }
    p {
      margin: 0;
      color: var(--cl-text-subtle);
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NotFound {}

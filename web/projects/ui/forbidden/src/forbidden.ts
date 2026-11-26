import { ChangeDetectionStrategy, Component } from '@angular/core';

/** Shown when the signed-in user has no permission for the page. Placeholder for now. */
@Component({
  selector: 'app-forbidden',
  template: `
    <section class="estado" role="alert">
      <h1>Acceso denegado</h1>
      <p>No tienes permisos para ver esta página.</p>
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
export class Forbidden {}

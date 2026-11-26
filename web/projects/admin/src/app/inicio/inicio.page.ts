import { ChangeDetectionStrategy, Component } from '@angular/core';

import { PageHeader } from '@corelog/ui/page-header';

/** Home of the organization console. Placeholder until the admin pages. */
@Component({
  selector: 'app-inicio-page',
  imports: [PageHeader],
  template: `<app-page-header title="Inicio" subtitle="Administración de la organización" />`,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class InicioPage {}

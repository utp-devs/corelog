import { ChangeDetectionStrategy, Component } from '@angular/core';

import { PageHeader } from '@corelog/ui/page-header';

/** Home of the workspace. Placeholder until the dashboard. */
@Component({
  selector: 'app-inicio-page',
  imports: [PageHeader],
  template: `<app-page-header title="Inicio" subtitle="Tu espacio de trabajo en CoreLog" />`,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class InicioPage {}

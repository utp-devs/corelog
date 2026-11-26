import { ApplicationConfig } from '@angular/core';

import { provideCorelog } from '@corelog/core';
import { environment } from '@corelog/environment';
import { routes } from './app.routes';

export const appConfig: ApplicationConfig = {
  providers: [provideCorelog({ app: 'workspace', apiUrl: environment.apiUrl, routes })],
};

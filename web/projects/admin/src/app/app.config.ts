import { ApplicationConfig } from '@angular/core';

import { provideCorelog } from '@corelog/core';
import { provideUnsavedChangesDialog } from '@corelog/ui/confirm-modal';
import { environment } from '@corelog/environment';
import { routes } from './app.routes';

export const appConfig: ApplicationConfig = {
  providers: [
    provideCorelog({ app: 'admin', apiUrl: environment.apiUrl, routes }),
    provideUnsavedChangesDialog(),
  ],
};

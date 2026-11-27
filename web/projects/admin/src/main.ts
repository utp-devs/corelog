import { bootstrapApplication } from '@angular/platform-browser';

import { appConfig } from './app/app.config';
import { AppHost } from './app/app-host';

bootstrapApplication(AppHost, appConfig).catch((error: unknown) => console.error(error));

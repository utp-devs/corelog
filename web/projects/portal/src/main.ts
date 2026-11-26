import { bootstrapApplication } from '@angular/platform-browser';

import { AppRoot } from '@corelog/ui/app-root';
import { appConfig } from './app/app.config';

bootstrapApplication(AppRoot, appConfig).catch((error: unknown) => console.error(error));

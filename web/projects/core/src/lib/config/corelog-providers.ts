import { registerLocaleData } from '@angular/common';
import { provideHttpClient, withFetch, withInterceptors } from '@angular/common/http';
import localeEsPe from '@angular/common/locales/es-PE';
import {
  EnvironmentProviders,
  LOCALE_ID,
  Provider,
  makeEnvironmentProviders,
  provideBrowserGlobalErrorListeners,
} from '@angular/core';
import {
  Route,
  Routes,
  TitleStrategy,
  provideRouter,
  withComponentInputBinding,
  withInMemoryScrolling,
} from '@angular/router';
import { MessageService } from 'primeng/api';
import { providePrimeNG } from 'primeng/config';

import { authInterceptor } from '../interceptors/auth.interceptor';
import { errorInterceptor } from '../interceptors/error.interceptor';
import { loadingInterceptor } from '../interceptors/loading.interceptor';
import { API_URL, CORELOG_APP } from './api.config';
import { AppTitleStrategy } from './app-title.strategy';
import { PRIMENG_TRANSLATION_ES, CorelogPreset } from './theme';

export interface CorelogOptions {
  /** Application name: portal, workspace or admin. */
  readonly app: string;
  /** Base URL of the API, e.g. /api/v1. */
  readonly apiUrl: string;
  /** Routes of the application. */
  readonly routes: Routes;
}

/** Application-wide providers shared by portal, workspace and admin. */
export function provideCorelog(options: CorelogOptions): EnvironmentProviders {
  registerLocaleData(localeEsPe);
  const providers: (Provider | EnvironmentProviders)[] = [
    provideBrowserGlobalErrorListeners(),
    provideRouter(
      options.routes as Route[],
      withComponentInputBinding(),
      withInMemoryScrolling({ scrollPositionRestoration: 'top', anchorScrolling: 'enabled' }),
    ),
    { provide: TitleStrategy, useClass: AppTitleStrategy },
    { provide: LOCALE_ID, useValue: 'es-PE' },
    { provide: API_URL, useValue: options.apiUrl },
    { provide: CORELOG_APP, useValue: options.app },
    provideHttpClient(
      withFetch(),
      withInterceptors([loadingInterceptor, errorInterceptor, authInterceptor]),
    ),
    providePrimeNG({
      ripple: true,
      translation: PRIMENG_TRANSLATION_ES,
      theme: {
        preset: CorelogPreset,
        options: { darkModeSelector: "[data-theme='dark']", cssLayer: false },
      },
    }),
    MessageService,
  ];
  return makeEnvironmentProviders(providers);
}

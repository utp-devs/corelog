import { registerLocaleData } from '@angular/common';
import { provideHttpClient, withFetch, withInterceptors } from '@angular/common/http';
import localeEsPe from '@angular/common/locales/es-PE';
import {
  EnvironmentProviders,
  LOCALE_ID,
  inject,
  Provider,
  makeEnvironmentProviders,
  provideAppInitializer,
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
import { map } from 'rxjs';
import { MessageService } from 'primeng/api';
import { providePrimeNG } from 'primeng/config';

import { OrganizationApiService } from '../services/api/organization-api.service';
import { AuthStore } from '../state/auth.store';
import { MEMBER_NAMES_SOURCE } from '../state/issue.store';
import { ORGANIZATION_ID_PROVIDER } from '../utils/org-id-provider';
import { getOrgId } from '../utils/org-context';
import { authInterceptor } from '../interceptors/auth.interceptor';
import { errorInterceptor } from '../interceptors/error.interceptor';
import { loadingInterceptor } from '../interceptors/loading.interceptor';
import { API_URL, CORELOG_APP } from './api.config';
import { AppTitleStrategy } from './app-title.strategy';
import { normalizePrimeAutofocus } from './primeng-autofocus';
import { PRIMENG_TRANSLATION_ES, CL_Z_INDEX, CorelogPreset } from './theme';

export interface CorelogOptions {
  /** Application name: portal, workspace or admin. */
  readonly app: string;
  /** Base URL of the API, e.g. /api/v1. */
  readonly apiUrl: string;
  /** Routes of the application. */
  readonly routes: Routes;
}

/** Wires the domain stores to the active organization and to the organization member list. */
export function provideCorelogDomain(): Provider[] {
  return [
    { provide: ORGANIZATION_ID_PROVIDER, useValue: getOrgId },
    {
      provide: MEMBER_NAMES_SOURCE,
      useFactory: () => {
        const api = inject(OrganizationApiService);
        return {
          listMembers: (organizationId: string) =>
            api
              .listMembers(organizationId)
              .pipe(
                map((members) =>
                  members.map((m) => ({ userId: m.userId, name: m.displayName || m.name })),
                ),
              ),
        };
      },
    },
  ];
}

/** Application-wide providers shared by portal, workspace and admin. */
export function provideCorelog(options: CorelogOptions): EnvironmentProviders {
  registerLocaleData(localeEsPe);
  normalizePrimeAutofocus();
  const providers: (Provider | EnvironmentProviders)[] = [
    provideBrowserGlobalErrorListeners(),
    // Validates a stored token before the first navigation, so guards and shells see the account.
    provideAppInitializer(() => inject(AuthStore).restoreSession()),
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
      zIndex: {
        modal: CL_Z_INDEX.modal,
        overlay: CL_Z_INDEX.overlay,
        menu: CL_Z_INDEX.menu,
        tooltip: CL_Z_INDEX.tooltip,
      },
      translation: PRIMENG_TRANSLATION_ES,
      theme: {
        preset: CorelogPreset,
        options: { darkModeSelector: "[data-theme='dark']", cssLayer: false },
      },
    }),
    MessageService,
    ...provideCorelogDomain(),
  ];
  return makeEnvironmentProviders(providers);
}

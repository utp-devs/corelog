import { EnvironmentProviders, Provider } from '@angular/core';
import { provideRouter } from '@angular/router';
import { MessageService } from 'primeng/api';

import { API_URL } from '../lib/config/api.config';
import { LOCATION_ASSIGN } from '../lib/config/navigation';

/** Records full-page navigations instead of performing them (jsdom cannot navigate). */
export interface LocationRecorder {
  readonly urls: string[];
  readonly assign: (url: string) => void;
}

export function recordLocation(): LocationRecorder {
  const urls: string[] = [];
  return { urls, assign: (url: string) => urls.push(url) };
}

/** Providers for unit tests: empty router, toast service, fixed API URL and a recorded location. */
export function provideCorelogTesting(
  recorder: LocationRecorder = recordLocation(),
): (Provider | EnvironmentProviders)[] {
  return [
    provideRouter([]),
    MessageService,
    { provide: API_URL, useValue: '/api/v1' },
    { provide: LOCATION_ASSIGN, useValue: recorder.assign },
  ];
}

export * from './console-guard';
export * from './viewport';

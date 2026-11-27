import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { errorInterceptor } from '../interceptors/error.interceptor';
import { OrgLogoService, OrgLogoWatch } from './org-logo.service';

const LOGO_URL = '/api/v1/organizations/org-1/logo';
let urlCount = 0;
const originalCreate = URL.createObjectURL;
const originalRevoke = URL.revokeObjectURL;

function png(): Blob {
  return new Blob(['png'], { type: 'image/png' });
}

function setup(): { service: OrgLogoService; http: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(withInterceptors([errorInterceptor])),
      provideHttpClientTesting(),
    ],
  });
  return {
    service: TestBed.inject(OrgLogoService),
    http: TestBed.inject(HttpTestingController),
  };
}

function flushLogo(http: HttpTestingController, blob: Blob = png()): void {
  http.expectOne(LOGO_URL).flush(blob);
}

describe('OrgLogoService', () => {
  beforeEach(() => {
    urlCount = 0;
    URL.createObjectURL = vi.fn(() => `blob:org-logo-${++urlCount}`);
    URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    URL.createObjectURL = originalCreate;
    URL.revokeObjectURL = originalRevoke;
  });

  it('makes no request and no URL when the organization has no logo', () => {
    const { service, http } = setup();

    const watch: OrgLogoWatch = service.watch('org-1', false);

    expect(watch.url()).toBeNull();
    http.expectNone(() => true);
    watch.release();
  });

  it('fetches the logo once and exposes an object URL', () => {
    const { service, http } = setup();

    const watch = service.watch('org-1', true, 0);
    expect(watch.url()).toBeNull();
    flushLogo(http);

    expect(watch.url()).toBe('blob:org-logo-1');
    http.verify();
  });

  it('shares one fetch between readers of the same organization and revision', () => {
    const { service, http } = setup();

    const first = service.watch('org-1', true, 0);
    const second = service.watch('org-1', true, 0);
    flushLogo(http);

    expect(first.url()).toBe('blob:org-logo-1');
    expect(second.url()).toBe('blob:org-logo-1');
    http.verify();
  });

  it('revokes the object URL only after the last reader releases it', () => {
    const { service, http } = setup();
    const first = service.watch('org-1', true, 0);
    const second = service.watch('org-1', true, 0);
    flushLogo(http);

    first.release();
    expect(URL.revokeObjectURL).not.toHaveBeenCalled();

    second.release();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:org-logo-1');
  });

  it('fetches again when the revision changes and releases the old URL', () => {
    const { service, http } = setup();
    const r0 = service.watch('org-1', true, 0);
    flushLogo(http);
    expect(r0.url()).toBe('blob:org-logo-1');

    r0.release();
    const r1 = service.watch('org-1', true, 1);
    flushLogo(http);

    expect(r1.url()).toBe('blob:org-logo-2');
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:org-logo-1');
    http.verify();
  });

  it('returns null when the fetch fails so the caller shows initials', () => {
    const { service, http } = setup();

    const watch = service.watch('org-1', true, 0);
    http.expectOne(LOGO_URL).flush(null, { status: 500, statusText: 'Server Error' });

    expect(watch.url()).toBeNull();
    expect(URL.createObjectURL).not.toHaveBeenCalled();
  });

  it('keeps no URL when the organization has no logo on the server (404)', () => {
    const { service, http } = setup();

    const watch = service.watch('org-1', true, 0);
    http.expectOne(LOGO_URL).flush(null, { status: 404, statusText: 'Not Found' });

    expect(watch.url()).toBeNull();
    expect(URL.createObjectURL).not.toHaveBeenCalled();
  });

  it('creates no URL when the reader goes away before the blob arrives', () => {
    const { service, http } = setup();

    const watch = service.watch('org-1', true, 0);
    watch.release();
    flushLogo(http);

    expect(URL.createObjectURL).not.toHaveBeenCalled();
    expect(watch.url()).toBeNull();
  });
});

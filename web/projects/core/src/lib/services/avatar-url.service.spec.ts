import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';

import { AvatarApiService } from './api/avatar-api.service';

import { provideCorelogTesting } from '../../testing';
import { errorInterceptor } from '../interceptors/error.interceptor';
import { AvatarUrlService, AvatarUrlWatch } from './avatar-url.service';

let urlCount = 0;
const originalCreate = URL.createObjectURL;
const originalRevoke = URL.revokeObjectURL;

function png(): Blob {
  return new Blob(['png'], { type: 'image/png' });
}

function setup(): { service: AvatarUrlService; http: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(withInterceptors([errorInterceptor])),
      provideHttpClientTesting(),
    ],
  });
  return {
    service: TestBed.inject(AvatarUrlService),
    http: TestBed.inject(HttpTestingController),
  };
}

function flushAvatar(http: HttpTestingController, userId: string, blob: Blob = png()): void {
  http.expectOne(`/api/v1/users/${userId}/avatar`).flush(blob);
}

describe('AvatarUrlService', () => {
  beforeEach(() => {
    urlCount = 0;
    URL.createObjectURL = vi.fn(() => `blob:avatar-${++urlCount}`);
    URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    URL.createObjectURL = originalCreate;
    URL.revokeObjectURL = originalRevoke;
  });

  it('does not fetch when the user has no avatar', () => {
    const { service, http } = setup();

    const watch: AvatarUrlWatch = service.watch('user-none', false);

    expect(watch.url()).toBeNull();
    http.expectNone(() => true);
    watch.release();
  });

  it('fetches the avatar once and returns an object URL', () => {
    const { service, http } = setup();

    const watch = service.watch('user-a', true, 'v1');
    expect(watch.url()).toBeNull();
    flushAvatar(http, 'user-a');

    expect(watch.url()).toBe('blob:avatar-1');
    http.verify();
  });

  it('shares one fetch between readers showing the same user and version', () => {
    const { service, http } = setup();

    const first = service.watch('user-b', true, 'v1');
    const second = service.watch('user-b', true, 'v1');
    flushAvatar(http, 'user-b');

    expect(second.url()).toBe('blob:avatar-1');
    expect(first.url()).toBe('blob:avatar-1');
    http.verify();
  });

  it('revokes the object URL only after the last reader releases it', () => {
    const { service, http } = setup();
    const first = service.watch('user-c', true, 'v1');
    const second = service.watch('user-c', true, 'v1');
    flushAvatar(http, 'user-c');

    first.release();
    expect(URL.revokeObjectURL).not.toHaveBeenCalled();

    second.release();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:avatar-1');
  });

  it('fetches again when the version changes and releases the old URL', () => {
    const { service, http } = setup();
    const v1 = service.watch('user-d', true, 'v1');
    flushAvatar(http, 'user-d');
    expect(v1.url()).toBe('blob:avatar-1');

    v1.release();
    const v2 = service.watch('user-d', true, 'v2');
    flushAvatar(http, 'user-d');

    expect(v2.url()).toBe('blob:avatar-2');
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:avatar-1');
    http.verify();
  });

  it('returns null when the fetch fails so the caller shows initials', () => {
    const { service, http } = setup();

    const watch = service.watch('user-e', true, 'v1');
    http
      .expectOne('/api/v1/users/user-e/avatar')
      .flush(null, { status: 500, statusText: 'Server Error' });

    expect(watch.url()).toBeNull();
    expect(URL.createObjectURL).not.toHaveBeenCalled();
  });

  it('keeps no URL when the user has no avatar on the server (404)', () => {
    const { service, http } = setup();

    const watch = service.watch('user-f', true, 'v1');
    http
      .expectOne('/api/v1/users/user-f/avatar')
      .flush(null, { status: 404, statusText: 'Not Found' });

    expect(watch.url()).toBeNull();
    expect(URL.createObjectURL).not.toHaveBeenCalled();
  });

  it('releases the URL when the reader goes away before the blob arrives', () => {
    const { service, http } = setup();

    const watch = service.watch('user-g', true, 'v1');
    watch.release();
    flushAvatar(http, 'user-g');

    expect(URL.createObjectURL).not.toHaveBeenCalled();
    expect(watch.url()).toBeNull();
  });

  it('ignores a second release of the same reader', () => {
    const { service, http } = setup();
    const keep = service.watch('user-h', true, 'v1');
    const watch = service.watch('user-h', true, 'v1');
    flushAvatar(http, 'user-h');

    watch.release();
    watch.release();

    expect(URL.revokeObjectURL).not.toHaveBeenCalled();
    expect(keep.url()).toBe('blob:avatar-1');
  });

  it('shows the URL when the source answers synchronously', () => {
    const getAvatar = vi.fn(() => of(png()));
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), { provide: AvatarApiService, useValue: { getAvatar } }],
    });
    const service = TestBed.inject(AvatarUrlService);

    const watch = service.watch('user-sync', true, 'v1');

    expect(getAvatar).toHaveBeenCalledWith('user-sync');
    expect(watch.url()).toBe('blob:avatar-1');
    watch.release();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:avatar-1');
  });

  it('revokes the remaining object URLs when the service is destroyed', () => {
    const { service, http } = setup();
    service.watch('user-i', true, 'v1');
    flushAvatar(http, 'user-i');

    service.ngOnDestroy();

    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:avatar-1');
  });
});

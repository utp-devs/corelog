import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { errorInterceptor } from '../../interceptors/error.interceptor';
import { AvatarApiService } from './avatar-api.service';

function setup(): { service: AvatarApiService; http: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(withInterceptors([errorInterceptor])),
      provideHttpClientTesting(),
    ],
  });
  return {
    service: TestBed.inject(AvatarApiService),
    http: TestBed.inject(HttpTestingController),
  };
}

describe('AvatarApiService', () => {
  it('requests the avatar of a user as a blob', () => {
    const { service, http } = setup();
    const blob = new Blob(['png'], { type: 'image/png' });
    let result: Blob | null | undefined;

    service.getAvatar('u-1').subscribe((value) => (result = value));

    const request = http.expectOne('/api/v1/users/u-1/avatar');
    expect(request.request.method).toBe('GET');
    expect(request.request.responseType).toBe('blob');
    request.flush(blob);
    expect(result).toBe(blob);
    http.verify();
  });

  it('emits null when the user has no avatar', () => {
    const { service, http } = setup();
    const emitted: (Blob | null)[] = [];

    service.getAvatar('u-2').subscribe((value) => emitted.push(value));
    http
      .expectOne('/api/v1/users/u-2/avatar')
      .flush(null, { status: 404, statusText: 'Not Found' });

    expect(emitted).toEqual([null]);
    http.verify();
  });

  it('fails with the normalized error for other statuses', () => {
    const { service, http } = setup();
    let failure: unknown;

    service.getAvatar('u-3').subscribe({ error: (error: unknown) => (failure = error) });
    http
      .expectOne('/api/v1/users/u-3/avatar')
      .flush(null, { status: 500, statusText: 'Server Error' });

    expect(failure).toMatchObject({ status: 500, code: 'http.500' });
  });

  it('uploads the file as multipart field "file" and returns the avatar URL', () => {
    const { service, http } = setup();
    const file = new File(['png'], 'foto.png', { type: 'image/png' });
    let url: string | undefined;

    service.uploadMyAvatar(file).subscribe((value) => (url = value));

    const request = http.expectOne({ method: 'PUT', url: '/api/v1/users/me/avatar' });
    expect(request.request.body).toBeInstanceOf(FormData);
    expect((request.request.body as FormData).get('file')).toBeInstanceOf(File);
    request.flush({ data: { avatar_url: '/users/u-1/avatar' } });
    expect(url).toBe('/users/u-1/avatar');
    http.verify();
  });

  it('deletes the own avatar', () => {
    const { service, http } = setup();

    service.deleteMyAvatar().subscribe();

    http.expectOne({ method: 'DELETE', url: '/api/v1/users/me/avatar' }).flush(null);
    http.verify();
  });
});

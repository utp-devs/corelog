import { TestBed } from '@angular/core/testing';
import { AvatarApiService } from '@corelog/core';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { from, Observable, of, throwError } from 'rxjs';

import { UserAvatar } from './user-avatar';

function setup(getAvatar: (userId: string) => Observable<Blob | null>) {
  const getAvatarSpy = vi.fn(getAvatar);
  TestBed.configureTestingModule({
    providers: [{ provide: AvatarApiService, useValue: { getAvatar: getAvatarSpy } }],
  });
  return { getAvatarSpy };
}

function create(inputs: {
  id: string;
  name: string;
  hasAvatar?: boolean;
  avatarKey?: string | null;
}) {
  const fixture = TestBed.createComponent(UserAvatar);
  fixture.componentRef.setInput('id', inputs.id);
  fixture.componentRef.setInput('name', inputs.name);
  if (inputs.hasAvatar !== undefined) fixture.componentRef.setInput('hasAvatar', inputs.hasAvatar);
  if (inputs.avatarKey !== undefined) fixture.componentRef.setInput('avatarKey', inputs.avatarKey);
  fixture.detectChanges();
  return fixture;
}

describe('UserAvatar', () => {
  beforeEach(() => {
    URL.createObjectURL = vi.fn(() => 'blob:avatar-1');
    URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows initials and requests no image when the user has no photo', () => {
    const { getAvatarSpy } = setup(() => of(null));
    const fixture = create({ id: 'u-1', name: 'Ana Torres', hasAvatar: false });

    const avatar = fixture.nativeElement.querySelector('[role="img"]') as HTMLElement;
    expect(avatar.textContent).toContain('AT');
    expect(getAvatarSpy).not.toHaveBeenCalled();
  });

  it('fetches the photo and shows it when the user has one', async () => {
    const { getAvatarSpy } = setup(() =>
      from(Promise.resolve(new Blob(['png'], { type: 'image/png' }))),
    );
    const fixture = create({ id: 'u-2', name: 'Luis Pardo', hasAvatar: true, avatarKey: 'k-1' });

    expect(getAvatarSpy).toHaveBeenCalledWith('u-2');
    await vi.waitFor(() => {
      fixture.detectChanges();
      expect(fixture.nativeElement.querySelector('img')).not.toBeNull();
    });
    expect(fixture.nativeElement.querySelector('img').getAttribute('src')).toBe('blob:avatar-1');
  });

  it('falls back to initials when the photo cannot be fetched', async () => {
    setup(() => throwError(() => new Error('boom')));
    const fixture = create({ id: 'u-3', name: 'Marta Ruiz', hasAvatar: true });

    await vi.waitFor(() => {
      fixture.detectChanges();
      expect(fixture.nativeElement.querySelector('[role="img"]')).not.toBeNull();
    });
    expect(fixture.nativeElement.querySelector('img')).toBeNull();
    expect(fixture.nativeElement.textContent).toContain('MR');
  });

  it('uses the avatar key as the photo presence when hasAvatar is not given', () => {
    const { getAvatarSpy } = setup(() => of(null));
    create({ id: 'u-4', name: 'Carla Vega', avatarKey: 'k-9' });
    expect(getAvatarSpy).toHaveBeenCalledWith('u-4');
  });
});

import { signal } from '@angular/core';
import { TestBed, type ComponentFixture } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { AuthStore, AvatarApiService, type User } from '@corelog/core';
import { map, timer } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { UserMenu } from './user-menu';

const user: User = {
  id: 'u1',
  name: 'Ana Pérez',
  email: 'ana@example.com',
  role: 'user',
  teamId: null,
  displayName: '',
  jobTitle: '',
  department: '',
  avatarKey: null,
  timezone: 'UTC',
  createdAt: '2026-01-01T00:00:00Z',
  lastLoginAt: null,
  emailVerifiedAt: null,
};

describe('UserMenu avatar', () => {
  let getAvatar: ReturnType<typeof vi.fn>;
  let current: ReturnType<typeof signal<User | null>>;

  beforeEach(() => {
    window.localStorage.clear();
    URL.createObjectURL = vi.fn(() => 'blob:ana-avatar');
    URL.revokeObjectURL = vi.fn();
    // Asynchronous like the HTTP client: the avatar cache registers its entry after subscribing.
    getAvatar = vi.fn(() => timer(0).pipe(map(() => new Blob(['png'], { type: 'image/png' }))));
    current = signal<User | null>(user);
    TestBed.configureTestingModule({
      imports: [UserMenu],
      providers: [
        provideRouter([]),
        { provide: AvatarApiService, useValue: { getAvatar } },
        { provide: AuthStore, useValue: { user: current, logout: vi.fn() } },
      ],
    });
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('shows the initials of the current user when there is no photo', () => {
    const fixture: ComponentFixture<UserMenu> = TestBed.createComponent(UserMenu);
    fixture.detectChanges();
    const avatar = fixture.nativeElement.querySelector('app-avatar') as HTMLElement;
    expect(avatar.textContent).toContain('AP');
    expect(getAvatar).not.toHaveBeenCalled();
  });

  it('loads the photo with the session when the user has an avatar', async () => {
    current.set({ ...user, avatarKey: 'k1' });
    const fixture = TestBed.createComponent(UserMenu);
    fixture.detectChanges();
    await new Promise((resolve) => setTimeout(resolve, 10));
    fixture.detectChanges();
    expect(document.querySelector('img[src="blob:ana-avatar"]')).not.toBeNull();
    expect(getAvatar).toHaveBeenCalledWith('u1');
  });
});

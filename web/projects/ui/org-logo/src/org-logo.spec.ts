import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideCorelogTesting } from '@corelog/core/testing';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { OrgLogo } from './org-logo';

const LOGO_URL = '/api/v1/organizations/o1/logo';
const png = () => new Blob(['png'], { type: 'image/png' });

function create(inputs: { hasLogo: boolean; revision?: number; name?: string; orgId?: string }) {
  const fixture = TestBed.createComponent(OrgLogo);
  fixture.componentRef.setInput('orgId', inputs.orgId ?? 'o1');
  fixture.componentRef.setInput('name', inputs.name ?? 'Acme Corp');
  fixture.componentRef.setInput('hasLogo', inputs.hasLogo);
  fixture.componentRef.setInput('revision', inputs.revision ?? 0);
  fixture.detectChanges();
  return fixture;
}

describe('OrgLogo', () => {
  beforeEach(() => {
    URL.createObjectURL = vi.fn(() => 'blob:org-logo');
    URL.revokeObjectURL = vi.fn();
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('makes no request and shows initials when the organization has no logo', () => {
    const http = TestBed.inject(HttpTestingController);
    const fixture = create({ hasLogo: false });
    const fallback = fixture.nativeElement.querySelector('[data-testid="org-logo-fallback"]');
    expect(fallback.textContent.trim()).toBe('AC');
    http.expectNone(() => true);
  });

  it('falls back to initials when the request resolves with no logo', () => {
    const http = TestBed.inject(HttpTestingController);
    const fixture = create({ hasLogo: true });
    http.expectOne(LOGO_URL).flush(null, { status: 404, statusText: 'Not Found' });
    fixture.detectChanges();
    expect(
      fixture.nativeElement.querySelector('[data-testid="org-logo-fallback"]').textContent.trim(),
    ).toBe('AC');
    expect(fixture.nativeElement.querySelector('[data-testid="org-logo"]')).toBeNull();
  });

  it('renders the fetched logo image when the organization has one', () => {
    const http = TestBed.inject(HttpTestingController);
    const fixture = create({ hasLogo: true, name: 'Acme' });
    http.expectOne(LOGO_URL).flush(png());
    fixture.detectChanges();
    const img = fixture.nativeElement.querySelector('[data-testid="org-logo"]') as HTMLImageElement;
    expect(img.getAttribute('src')).toBe('blob:org-logo');
    expect(fixture.nativeElement.querySelector('[data-testid="org-logo-fallback"]')).toBeNull();
  });

  it('fetches again when the revision changes after a logo change', () => {
    const http = TestBed.inject(HttpTestingController);
    const fixture = create({ hasLogo: true, revision: 0 });
    http.expectOne(LOGO_URL).flush(png());
    fixture.componentRef.setInput('revision', 1);
    fixture.detectChanges();
    http.expectOne(LOGO_URL).flush(png());
    http.verify();
  });

  it('shows initials again once the logo is removed (hasLogo false, no request)', () => {
    const http = TestBed.inject(HttpTestingController);
    const fixture = create({ hasLogo: true });
    http.expectOne(LOGO_URL).flush(png());
    fixture.detectChanges();
    fixture.componentRef.setInput('hasLogo', false);
    fixture.componentRef.setInput('revision', 1);
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('[data-testid="org-logo-fallback"]')).not.toBeNull();
    http.expectNone(() => true);
  });

  it('revokes the object URL when the logo is destroyed', () => {
    const http = TestBed.inject(HttpTestingController);
    const fixture = create({ hasLogo: true });
    http.expectOne(LOGO_URL).flush(png());
    fixture.detectChanges();
    fixture.destroy();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:org-logo');
  });
});

import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { vi } from 'vitest';

import { focusFirstInvalid, goAfterAuth } from './auth-navigation';

describe('goAfterAuth', () => {
  let router: Router;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    router = TestBed.inject(Router);
    vi.spyOn(router, 'navigateByUrl').mockResolvedValue(true);
  });

  it('uses the router for portal targets', () => {
    const assign = vi.fn();
    goAfterAuth('/invite/tok?a=1', router, assign);
    expect(router.navigateByUrl).toHaveBeenCalledWith('/invite/tok?a=1');
    expect(assign).not.toHaveBeenCalled();
  });

  it('uses a full-page navigation for other apps', () => {
    const assign = vi.fn();
    goAfterAuth('/workspace/inicio', router, assign);
    expect(assign).toHaveBeenCalledWith('/workspace/inicio');
    expect(router.navigateByUrl).not.toHaveBeenCalled();
  });
});

describe('focusFirstInvalid', () => {
  it('focuses the first element marked as invalid', () => {
    const root = document.createElement('form');
    root.innerHTML =
      '<input id="a" /><input id="b" aria-invalid="true" /><input id="c" aria-invalid="true" />';
    document.body.appendChild(root);
    focusFirstInvalid(root);
    expect(document.activeElement?.id).toBe('b');
    root.remove();
  });
});

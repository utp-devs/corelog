import { TestBed } from '@angular/core/testing';

import { LoadingSkeleton, LoadingSkeletonVariant } from './loading-skeleton';

function render(variant: LoadingSkeletonVariant, rows?: number): HTMLElement {
  TestBed.configureTestingModule({ imports: [LoadingSkeleton] });
  const fixture = TestBed.createComponent(LoadingSkeleton);
  fixture.componentRef.setInput('variant', variant);
  if (rows !== undefined) fixture.componentRef.setInput('rows', rows);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('LoadingSkeleton', () => {
  it('announces the loading state to assistive technology', () => {
    const el = render('table');
    const status = el.querySelector('[role="status"]');
    expect(status?.getAttribute('aria-label')).toBe('Cargando');
    expect(status?.getAttribute('aria-busy')).toBe('true');
  });

  it('renders every variant without errors', () => {
    expect(render('table', 3).querySelectorAll('.loading-skeleton__bone--line')).toHaveLength(3);
    TestBed.resetTestingModule();
    expect(render('card').querySelector('.loading-skeleton__bone--title')).not.toBeNull();
    TestBed.resetTestingModule();
    const detail = render('detail');
    expect(detail.querySelectorAll('.loading-skeleton__bone--title')).toHaveLength(2);
    expect(detail.querySelector('[role="status"]')).not.toBeNull();
  });
});

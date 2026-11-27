import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { CatalogIcon } from './catalog-icon';
import { CATALOG_ICON_PATHS } from './catalog-icon-paths';

describe('CatalogIcon', () => {
  it('has a glyph for every key of the palette', () => {
    expect(Object.keys(CATALOG_ICON_PATHS)).toHaveLength(22);
  });

  it('draws the paths of the key and exposes the color key', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(CatalogIcon);
    fixture.componentRef.setInput('icon', 'chevrons-up');
    fixture.componentRef.setInput('color', 'red');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelectorAll('path')).toHaveLength(CATALOG_ICON_PATHS['chevrons-up'].length);
    expect(el.getAttribute('data-color')).toBe('red');
    expect(el.getAttribute('aria-hidden')).toBe('true');
  });
});

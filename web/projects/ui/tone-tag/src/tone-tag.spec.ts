import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { ToneTag } from './tone-tag';

describe('ToneTag', () => {
  it('renders the label with the class of its tone', () => {
    TestBed.configureTestingModule({});
    const fixture = TestBed.createComponent(ToneTag);
    fixture.componentRef.setInput('label', 'Alta');
    fixture.componentRef.setInput('tone', 'orange');
    fixture.detectChanges();
    const tag = (fixture.nativeElement as HTMLElement).querySelector('.tag');
    expect(tag?.textContent?.trim()).toBe('Alta');
    expect(tag?.classList.contains('orange')).toBe(true);
  });
});

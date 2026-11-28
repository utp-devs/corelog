import { TestBed } from '@angular/core/testing';

import { MarkText, markSegments } from './mark-text';

describe('markSegments', () => {
  it('keeps only the mark tags as highlights', () => {
    expect(markSegments('Actualizar <mark>dep</mark>endencias')).toEqual([
      { text: 'Actualizar ', mark: false },
      { text: 'dep', mark: true },
      { text: 'endencias', mark: false },
    ]);
  });

  it('decodes the entities of the escaped text', () => {
    expect(markSegments('a &lt;b&gt; &amp; &quot;c&quot;')).toEqual([
      { text: 'a <b> & "c"', mark: false },
    ]);
  });

  it.each([
    '<script>alert(1)</script>',
    '<img src=x onerror=alert(1)>',
    '<a href="javascript:alert(1)">x</a>',
    '<mark onclick="alert(1)">x</mark>',
    '<svg onload=alert(1)>',
    '&lt;script&gt;alert(1)&lt;/script&gt;',
  ])('never produces markup other than marks and plain spans: %s', (input) => {
    const fixture = TestBed.createComponent(MarkText);
    fixture.componentRef.setInput('highlighted', input);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    const tags = Array.from(root.querySelectorAll('*')).map((node) => node.tagName.toLowerCase());
    expect(tags.every((tag) => tag === 'mark' || tag === 'span')).toBe(true);
    for (const node of Array.from(root.querySelectorAll('mark'))) {
      expect(node.attributes.length).toBe(1);
      expect(node.getAttribute('class')).toBe('mark-text__mark');
    }
  });

  it('uses the fallback when there is no highlight', () => {
    const fixture = TestBed.createComponent(MarkText);
    fixture.componentRef.setInput('highlighted', '');
    fixture.componentRef.setInput('fallback', 'Sin título');
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toBe('Sin título');
  });
});

import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { buildScale, dayNumber } from '@corelog/core';

import { Timeline, TimelineBand, TimelineDependency, TimelineRow } from './timeline';

const day = (iso: string) => dayNumber(iso) as number;

function row(id: string, start: string | null, end: string | null): TimelineRow {
  return {
    id,
    title: `Epica ${id}`,
    key: id.toUpperCase(),
    category: 'in_progress',
    percent: 40,
    start,
    end,
    description: 'En curso, 40 % completado',
  };
}

@Component({
  imports: [Timeline],
  template: `
    <app-timeline
      [rows]="rows()"
      [scale]="scale"
      [bands]="bands()"
      [dependencies]="dependencies()"
      [reorderable]="reorderable()"
      (rowActivated)="activated.push($event)"
      (reordered)="orders.push($event)"
    />
  `,
})
class Host {
  readonly rows = signal<readonly TimelineRow[]>([
    row('a', '2026-11-03', '2026-11-20'),
    row('b', '2026-11-25', '2026-12-20'),
    row('c', null, null),
  ]);
  readonly bands = signal<readonly TimelineBand[]>([
    { id: 'v1', label: 'v1.0', kind: 'version', start: '2026-11-10', end: '2026-11-30' },
  ]);
  readonly dependencies = signal<readonly TimelineDependency[]>([
    { from: 'a', to: 'b', kind: 'blocks' },
  ]);
  readonly reorderable = signal(true);
  readonly scale = buildScale(day('2026-11-01'), day('2026-12-31'), 'month', day('2026-11-15'));
  readonly activated: string[] = [];
  readonly orders: Array<readonly string[]> = [];
}

function render() {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

describe('Timeline', () => {
  it('draws the axis, a bar for each dated row and none for undated ones', () => {
    const { el } = render();
    expect([...el.querySelectorAll('.timeline__tick')].map((t) => t.textContent?.trim())).toEqual([
      'nov 2026',
      'dic 2026',
    ]);
    expect(el.querySelector('[data-testid="timeline-bar-a"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="timeline-bar-b"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="timeline-bar-c"]')).toBeNull();
    expect(el.querySelectorAll('.timeline__title')).toHaveLength(3);
  });

  it('draws the today line, the version band and the dependency arrow', () => {
    const { el } = render();
    expect(el.querySelector('[data-testid="timeline-today"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="timeline-band"]')?.textContent).toContain('v1.0');
    expect(el.querySelectorAll('[data-testid="timeline-dependency"]')).toHaveLength(1);
  });

  it('colors the bar by status category and scales its progress strip', () => {
    const { el } = render();
    const bar = el.querySelector('[data-testid="timeline-bar-a"]') as HTMLElement;
    expect(bar.classList.contains('timeline__bar--progress')).toBe(true);
    expect(bar.style.getPropertyValue('--timeline-progress')).toBe('40%');
  });

  it('gives each row an accessible name with its description', () => {
    const { el } = render();
    const title = el.querySelector('[data-testid="timeline-row-a"]') as HTMLElement;
    expect(title.getAttribute('aria-label')).toBe('Epica a. En curso, 40 % completado');
  });

  it('emits the row id when its title or bar is chosen', () => {
    const { fixture, el } = render();
    (el.querySelector('[data-testid="timeline-row-b"]') as HTMLElement).click();
    (el.querySelector('[data-testid="timeline-bar-a"]') as HTMLElement).click();
    expect(fixture.componentInstance.activated).toEqual(['b', 'a']);
  });

  it('reorders with the move buttons and disables the ends', () => {
    const { fixture, el } = render();
    const down = el.querySelector('[aria-label="Bajar Epica a"]') as HTMLButtonElement;
    const up = el.querySelector('[aria-label="Subir Epica a"]') as HTMLButtonElement;
    expect(up.disabled).toBe(true);
    down.click();
    expect(fixture.componentInstance.orders).toEqual([['b', 'a', 'c']]);
  });

  it('hides the drag handle and move buttons when not reorderable', () => {
    const { fixture, el } = render();
    fixture.componentInstance.reorderable.set(false);
    fixture.detectChanges();
    expect(el.querySelector('.timeline__handle')).toBeNull();
    expect(el.querySelector('.timeline__move')).toBeNull();
  });

  it('omits the band area when there are no bands', () => {
    const { fixture, el } = render();
    fixture.componentInstance.bands.set([]);
    fixture.detectChanges();
    expect(el.querySelector('.timeline__bands-label')).toBeNull();
  });
});

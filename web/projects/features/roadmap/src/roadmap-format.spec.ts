import type { RoadmapBar } from '@corelog/core';

import {
  describeBar,
  formatDay,
  formatRange,
  statusLabel,
  toBand,
  toTimelineRow,
} from './roadmap-format';

function bar(overrides: Partial<RoadmapBar> = {}): RoadmapBar {
  return {
    id: 'e-1',
    kind: 'epic',
    key: 'CL-1',
    title: 'Portal',
    status: 'in_progress',
    statusCategory: 'in_progress',
    assignee: '',
    start: '2026-11-03',
    end: '2026-12-20',
    startSource: 'own',
    endSource: 'own',
    dated: true,
    complete: false,
    progress: {
      total: 4,
      todo: 1,
      inProgress: 1,
      done: 2,
      percent: 50,
      pointsTotal: 0,
      pointsDone: 0,
    },
    ...overrides,
  };
}

describe('roadmap format', () => {
  it('formats days and ranges in Spanish', () => {
    expect(formatDay('2026-12-03')).toBe('3 dic 2026');
    expect(formatDay(null)).toBe('Sin fecha');
    expect(formatRange('2026-11-03', '2026-12-20')).toBe('3 nov 2026 - 20 dic 2026');
    expect(formatRange('2026-11-03', '2026-11-03')).toBe('3 nov 2026');
    expect(formatRange(null, null)).toBe('Sin fechas');
  });

  it('names the status by kind', () => {
    expect(statusLabel(bar())).toBe('En curso');
    expect(statusLabel(bar({ kind: 'sprint', status: 'active' }))).toBe('Activo');
    expect(statusLabel(bar({ kind: 'version', status: 'released' }))).toBe('Publicada');
  });

  it('describes a bar with its progress', () => {
    expect(describeBar(bar())).toBe('En curso, 3 nov 2026 - 20 dic 2026, 50 % completado (2 de 4)');
  });

  it('maps bars to timeline rows and bands', () => {
    expect(toTimelineRow(bar())).toMatchObject({
      id: 'e-1',
      key: 'CL-1',
      category: 'in_progress',
      percent: 50,
    });
    expect(toTimelineRow(bar({ key: '' })).key).toBeUndefined();
    expect(toBand(bar({ kind: 'sprint' })).kind).toBe('sprint');
    expect(toBand(bar({ kind: 'version' })).kind).toBe('version');
  });
});

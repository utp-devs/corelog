import type { GadgetData } from '@corelog/core';

import { isChartGadget, toChartModel } from './gadget-chart-model';
import { activityVerb, categoryTone, formatShortDay, groupLabel, isPastDay } from './gadget-format';

const STATS: GadgetData = {
  type: 'issue_statistics',
  data: {
    groupBy: 'assignee',
    total: 7,
    groups: [
      { key: 'u1', label: 'Ana', count: 4 },
      { key: 'u2', label: '', count: 2 },
    ],
    other: 1,
  },
};

describe('chart model', () => {
  it('knows which gadget types are charts', () => {
    expect(
      ['issue_statistics', 'pie', 'created_vs_resolved', 'sprint_burndown'].every(isChartGadget),
    ).toBe(true);
    expect(isChartGadget('assigned_to_me')).toBe(false);
  });

  it('maps statistics to progress rows with an "others" row and a table', () => {
    const model = toChartModel(STATS);
    expect(model?.chartType).toBe('progress');
    expect(model?.labels).toEqual(['Ana', 'Sin valor', 'Otros']);
    expect(model?.series[0].data).toEqual([4, 2, 1]);
    expect(model?.table.rows).toEqual([
      ['Ana', 4],
      ['Sin valor', 2],
      ['Otros', 1],
    ]);
    expect(model?.isEmpty).toBe(false);
  });

  it('draws the pie as a doughnut and colors statuses by category', () => {
    const model = toChartModel({
      type: 'pie',
      data: {
        groupBy: 'status',
        total: 2,
        groups: [
          { key: 'todo', label: 'Por hacer', count: 1 },
          { key: 'done', label: 'Hecho', count: 1 },
        ],
        other: 0,
      },
    });
    expect(model?.chartType).toBe('doughnut');
    expect(model?.series[0].colors).toEqual(['var(--cl-cat-todo)', 'var(--cl-cat-done)']);
  });

  it('is empty without incidencias', () => {
    const model = toChartModel({
      type: 'pie',
      data: { groupBy: 'status', total: 0, groups: [], other: 0 },
    });
    expect(model?.isEmpty).toBe(true);
  });

  it('maps created against resolved to two lines', () => {
    const model = toChartModel({
      type: 'created_vs_resolved',
      data: {
        days: 7,
        series: [
          { date: '2026-11-26', created: 2, resolved: 1 },
          { date: '2026-11-27', created: 0, resolved: 3 },
        ],
        totalCreated: 2,
        totalResolved: 4,
        netOpenChange: -2,
      },
    });
    expect(model?.chartType).toBe('line');
    expect(model?.labels).toEqual(['26 nov', '27 nov']);
    expect(model?.series.map((s) => s.label)).toEqual(['Creadas', 'Resueltas']);
  });

  it('maps the burndown with a dashed ideal line and gaps after today', () => {
    const model = toChartModel({
      type: 'sprint_burndown',
      data: {
        sprint: { id: 's', name: 'S1', projectKey: 'CL', startDate: null, endDate: null },
        totalPoints: 10,
        points: [
          { date: '2026-11-26', remaining: 10, ideal: 10 },
          { date: '2026-11-27', remaining: null, ideal: 5 },
        ],
      },
    });
    expect(model?.series[1].dashed).toBe(true);
    expect(Number.isNaN(model?.series[0].data[1])).toBe(true);
    expect(model?.table.rows[1]).toEqual(['27 nov', '-', 5]);
    expect(model?.isEmpty).toBe(false);
  });

  it('is empty when there is no sprint to show and gives no model to other types', () => {
    const empty = toChartModel({
      type: 'sprint_burndown',
      data: { sprint: null, totalPoints: 0, points: [] },
    });
    expect(empty?.isEmpty).toBe(true);
    expect(toChartModel({ type: 'text', data: { markdown: '' } })).toBeNull();
  });
});

describe('gadget format helpers', () => {
  it('formats days and detects past days', () => {
    expect(formatShortDay('2026-12-03')).toBe('3 dic');
    expect(formatShortDay(null)).toBe('');
    expect(isPastDay('2026-11-26', new Date(2026, 10, 27))).toBe(true);
    expect(isPastDay('2026-11-27', new Date(2026, 10, 27))).toBe(false);
  });

  it('maps status categories to tones and names empty groups', () => {
    expect(categoryTone('done')).toBe('green');
    expect(categoryTone('in_progress')).toBe('blue');
    expect(categoryTone('todo')).toBe('neutral');
    expect(groupLabel(' ')).toBe('Sin valor');
  });

  it('describes activity in Spanish', () => {
    const base = {
      id: '1',
      issueId: 'i',
      issueKey: 'CL-1',
      summary: 's',
      actor: 'Ana',
      field: '',
      oldValue: '',
      newValue: '',
      occurredAt: '',
    };
    expect(activityVerb({ ...base, action: 'created' })).toBe('creó');
    expect(activityVerb({ ...base, action: 'status_changed', newValue: 'Hecho' })).toBe(
      'movió a "Hecho"',
    );
    expect(activityVerb({ ...base, action: 'other' })).toBe('actualizó');
  });
});

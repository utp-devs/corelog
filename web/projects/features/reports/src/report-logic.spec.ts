import { convertToParamMap } from '@angular/router';
import type { Report, ReportColumn } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { buildReportChart, describeSeries, isReportEmpty } from './report-charts';
import { isReportParamError, reportErrorMessage, REPORT_LOAD_FAILED } from './report-errors';
import { formatCell, formatDate, sortRows } from './report-format';
import {
  activePreset,
  addDays,
  effectiveRange,
  inclusiveDays,
  queryFromParams,
  queryToParams,
  rangeForDays,
} from './report-query';
import { summarySubject, summaryTiles } from './report-summary';

function report(kind: string, series: Report['series'], summary: Report['summary'] = {}): Report {
  return {
    kind,
    title: `Informe ${kind}`,
    generatedAt: '2026-11-27T10:00:00Z',
    params: {},
    series,
    table: { columns: [], rows: series.length ? [['x']] : [] },
    summary,
  };
}

function points(name: string, values: number[]): Report['series'][number] {
  return { name, points: values.map((y, index) => ({ x: `d${index}`, y })) };
}

describe('report formatting', () => {
  it('formats dates as dd/mm/aaaa and keeps other text', () => {
    expect(formatDate('2026-11-05')).toBe('05/11/2026');
    expect(formatDate('2026-11-05T10:00:00Z')).toBe('05/11/2026');
    expect(formatDate('Semana 3')).toBe('Semana 3');
  });

  it('formats cells by column type', () => {
    expect(formatCell(1234.5, 'number')).toBe('1234,5');
    expect(formatCell(66.67, 'percent')).toBe('66,67 %');
    expect(formatCell('2026-01-02', 'date')).toBe('02/01/2026');
    expect(formatCell(null, 'number')).toBe('–');
    expect(formatCell('Sí', 'text')).toBe('Sí');
    expect(formatCell(true, 'text')).toBe('Sí');
  });

  it('sorts rows by type with empty cells last in both directions', () => {
    const column: ReportColumn = { key: 'n', label: 'N', type: 'number' };
    const rows = [[10], [null], [2], [33]];
    expect(sortRows(rows, 0, column, 'asc').map((row) => row[0])).toEqual([2, 10, 33, null]);
    expect(sortRows(rows, 0, column, 'desc').map((row) => row[0])).toEqual([33, 10, 2, null]);
    const text: ReportColumn = { key: 't', label: 'T', type: 'text' };
    expect(sortRows([['b-10'], ['b-2'], ['a']], 0, text, 'asc').map((row) => row[0])).toEqual([
      'a',
      'b-2',
      'b-10',
    ]);
    expect(rows[0][0]).toBe(10);
  });
});

describe('report errors', () => {
  it('maps the report codes to Spanish messages', () => {
    expect(reportErrorMessage({ code: 'report.range_too_large' })).toContain('366');
    expect(reportErrorMessage({ code: 'report.too_many_issues' })).toContain('demasiadas');
    expect(reportErrorMessage({ code: 'report.version_not_found' })).toContain('versión');
    expect(reportErrorMessage({ code: 'http.500' })).toBe(REPORT_LOAD_FAILED);
    expect(reportErrorMessage(null, 'otro')).toBe('otro');
  });

  it('tells parameter problems from outages', () => {
    expect(isReportParamError({ code: 'report.filter_invalid' })).toBe(true);
    expect(isReportParamError({ code: 'network.unreachable' })).toBe(false);
    expect(isReportParamError({ code: 'report.kind_unknown' })).toBe(false);
  });
});

describe('report query', () => {
  it('reads and writes the URL parameters', () => {
    const query = queryFromParams(
      convertToParamMap({ from: '2026-11-01', group_by: 'issues', limit: '10', junk: 'x' }),
    );
    expect(query).toEqual({ from: '2026-11-01', groupBy: 'issues', limit: 10 });
    const params = queryToParams({ from: '2026-11-01', limit: 6 });
    expect(params['from']).toBe('2026-11-01');
    expect(params['group_by']).toBeNull();
    expect(params['limit']).toBe(6);
  });

  it('ignores an invalid limit', () => {
    expect(queryFromParams(convertToParamMap({ limit: 'abc' })).limit).toBeUndefined();
  });

  it('computes presets and the default range', () => {
    expect(addDays('2026-11-01', -1)).toBe('2026-10-31');
    expect(inclusiveDays('2026-11-01', '2026-11-30')).toBe(30);
    expect(rangeForDays(7, '2026-11-27')).toEqual({ from: '2026-11-21', to: '2026-11-27' });
    expect(effectiveRange({}, '2026-11-27')).toEqual({ from: '2026-10-29', to: '2026-11-27' });
    expect(activePreset({}, '2026-11-27')).toBe(30);
    expect(activePreset(rangeForDays(90, '2026-11-27'), '2026-11-27')).toBe(90);
    expect(activePreset({ from: '2026-11-01', to: '2026-11-27' }, '2026-11-27')).toBeNull();
    expect(activePreset({ ...rangeForDays(7, '2026-11-26') }, '2026-11-27')).toBeNull();
  });
});

describe('report summary', () => {
  it('builds tiles with units and skips keys the report did not send', () => {
    const tiles = summaryTiles('control', { count: 3, average_days: 1, p85_days: 2.5 });
    expect(tiles.map((tile) => [tile.label, tile.value])).toEqual([
      ['Incidencias resueltas', '3'],
      ['Promedio', '1 día'],
      ['Percentil 85', '2,5 días'],
    ]);
  });

  it('shows the unit of the work on sprint progress and a dash for null values', () => {
    const tiles = summaryTiles('burndown', { committed: 10, metric: 'points', remaining: null });
    expect(tiles[0]).toMatchObject({ value: '10', hint: 'puntos' });
    expect(tiles[1].value).toBe('–');
    expect(summaryTiles('unknown', { a: 1 })).toEqual([]);
  });

  it('formats percent and hours', () => {
    const tiles = summaryTiles('time-tracking', { spent_hours: 12.5, accuracy_percent: 80 });
    expect(tiles.map((tile) => tile.value)).toEqual(['12,5 h', '80 %']);
  });

  it('names the sprint or version of the report', () => {
    expect(summarySubject({ sprint: { name: 'Sprint 2' } })).toBe('Sprint 2');
    expect(summarySubject({ version: { name: '1.0' } })).toBe('1.0');
    expect(summarySubject({})).toBeNull();
  });
});

describe('report chart builders', () => {
  it('draws the burndown with a dashed ideal line from the chart tokens', () => {
    const spec = buildReportChart(
      report('burndown', [points('Restante', [5, 3]), points('Ideal', [5, 2])]),
    );
    expect(spec?.chartType).toBe('line');
    expect(spec?.labels).toEqual(['d0', 'd1']);
    const ideal = spec?.series.find((series) => series.label === 'Ideal');
    expect(ideal).toMatchObject({ dashed: true, data: [5, 2] });
    expect(spec?.series[0].color).toBe('var(--cl-chart-1)');
  });

  it('draws the velocity as grouped bars', () => {
    const spec = buildReportChart(
      report('velocity', [points('Comprometido', [8]), points('Completado', [6])]),
    );
    expect(spec?.chartType).toBe('bar');
    expect(spec?.series.map((series) => series.color)).toEqual([
      'var(--cl-chart-1)',
      'var(--cl-chart-3)',
    ]);
  });

  it('stacks the cumulative flow with finished work at the bottom', () => {
    const spec = buildReportChart(
      report('cumulative-flow', [
        points('Por hacer', [1]),
        points('En curso', [2]),
        points('Hecho', [3]),
      ]),
    );
    expect(spec?.stacked).toBe(true);
    expect(spec?.series.map((series) => series.label)).toEqual(['Hecho', 'En curso', 'Por hacer']);
    expect(spec?.series.every((series) => series.fill)).toBe(true);
  });

  it('draws the control chart as points with a rolling average and a dashed band', () => {
    const spec = buildReportChart(
      report('control', [
        points('Tiempo de ciclo (días)', [1, 2]),
        points('Media móvil', [1, 1.5]),
        points('Límite superior', [2, 2]),
        points('Límite inferior', [0, 1]),
      ]),
    );
    expect(spec?.series[0]).toMatchObject({ pointsOnly: true, pointRadius: 4 });
    expect(spec?.series[1].color).toBe('var(--cl-chart-2)');
    expect(spec?.series[2].dashed).toBe(true);
  });

  it('draws created versus resolved as bars with an optional cumulative line', () => {
    const series = [
      points('Creadas', [2]),
      points('Resueltas', [1]),
      points('Diferencia acumulada', [1]),
    ];
    const withLine = buildReportChart(report('created-vs-resolved', series));
    expect(withLine?.series.map((entry) => entry.type)).toEqual(['bar', 'bar', 'line']);
    const withoutLine = buildReportChart(report('created-vs-resolved', series), {
      cumulative: false,
    });
    expect(withoutLine?.series.map((entry) => entry.label)).toEqual(['Creadas', 'Resueltas']);
  });

  it('draws the workload as horizontal bars', () => {
    const spec = buildReportChart(report('workload', [points('Incidencias abiertas', [4])]));
    expect(spec?.horizontal).toBe(true);
  });

  it('colors every bar of the sprint summary by outcome', () => {
    const spec = buildReportChart(
      report('sprint-report', [
        {
          name: 'Puntos',
          points: [
            { x: 'Comprometido', y: 5 },
            { x: 'Completado', y: 3 },
          ],
        },
      ]),
    );
    expect(spec?.series[0].colors).toEqual(['var(--cl-chart-1)', 'var(--cl-chart-3)']);
  });

  it('returns null without points and flags the report as empty', () => {
    const empty = report('burndown', []);
    expect(buildReportChart(empty)).toBeNull();
    expect(isReportEmpty(empty)).toBe(true);
    expect(isReportEmpty(report('burndown', [points('Restante', [1])]))).toBe(false);
  });

  it('summarizes the series for screen readers', () => {
    const text = describeSeries(report('burndown', [points('Restante', [5, 1, 3])]));
    expect(text).toContain('Restante: 3 valores, de 1 a 5, último 3');
    expect(text).toContain('tabla');
    expect(describeSeries(report('burndown', []))).toContain('Sin datos');
  });
});

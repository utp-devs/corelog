import { Provider, signal } from '@angular/core';
import {
  CurrentProjectStore,
  ReportApiService,
  SprintV2ApiService,
  VersionApiService,
  type Project,
  type Report,
  type ReportDescriptor,
} from '@corelog/core';
import { MessageService } from 'primeng/api';
import { Observable, of } from 'rxjs';
import { Mock, vi } from 'vitest';

export const CATALOG: ReportDescriptor[] = [
  {
    kind: 'burndown',
    title: 'Burndown del sprint',
    description: 'Trabajo restante frente a la línea ideal.',
    params: ['sprint', 'group_by', 'filter'],
  },
  {
    kind: 'velocity',
    title: 'Velocidad',
    description: 'Puntos por sprint.',
    params: ['limit'],
  },
  {
    kind: 'created-vs-resolved',
    title: 'Creadas frente a resueltas',
    description: 'Creadas y resueltas.',
    params: ['from', 'to', 'interval', 'filter'],
  },
  {
    kind: 'version',
    title: 'Versión',
    description: 'Progreso de una versión.',
    params: ['version', 'filter'],
  },
  {
    kind: 'workload',
    title: 'Carga de trabajo',
    description: 'Abiertas por persona.',
    params: ['filter'],
  },
];

export function reportOf(kind: string, overrides: Partial<Report> = {}): Report {
  return {
    kind,
    title: CATALOG.find((entry) => entry.kind === kind)?.title ?? kind,
    generatedAt: '2026-11-27T10:00:00Z',
    params: {},
    series: [
      {
        name: 'Incidencias abiertas',
        points: [
          { x: 'Ana', y: 4 },
          { x: 'Bea', y: 2 },
        ],
      },
    ],
    table: {
      columns: [
        { key: 'assignee', label: 'Persona', type: 'text' },
        { key: 'open', label: 'Abiertas', type: 'number' },
      ],
      rows: [
        ['Ana', 4],
        ['Bea', 2],
      ],
    },
    summary: { open: 6, unassigned: 0 },
    ...overrides,
  };
}

export interface ReportsTestOptions {
  readonly canViewReports?: boolean;
  readonly archived?: boolean;
  readonly list?: () => Observable<ReportDescriptor[]>;
  readonly get?: (projectId: string, kind: string, query: unknown) => Observable<Report>;
  readonly csv?: () => Observable<Blob>;
  readonly versions?: { id: string; name: string; status: string }[];
}

export interface ReportsTestEnvironment {
  readonly providers: Provider[];
  readonly list: Mock;
  readonly get: Mock;
  readonly csv: Mock;
}

/** Fake project in view and fake report services; the real ReportStore runs on top of them. */
export function reportsTestEnvironment(options: ReportsTestOptions = {}): ReportsTestEnvironment {
  const project = {
    id: 'p-1',
    key: 'SCRUM',
    name: 'Scrum',
    archivedAt: options.archived ? '2026-11-01T00:00:00Z' : null,
    template: 'scrum',
    type: 'scrum',
  } as unknown as Project;
  const list = vi.fn(options.list ?? (() => of(CATALOG)));
  const get = vi.fn(options.get ?? ((_: string, kind: string) => of(reportOf(kind))));
  const csv = vi.fn(options.csv ?? (() => of(new Blob(['﻿a;b']))));
  const current = {
    project: signal(project),
    projectId: signal('p-1'),
    key: signal('SCRUM'),
    permissionsLoaded: signal(true),
    can: () => options.canViewReports ?? true,
  };
  return {
    list,
    get,
    csv,
    providers: [
      { provide: CurrentProjectStore, useValue: current },
      { provide: ReportApiService, useValue: { list, get, csv } },
      {
        provide: SprintV2ApiService,
        useValue: { list: () => of([{ id: 's-1', name: 'Sprint 1', status: 'active' }]) },
      },
      { provide: VersionApiService, useValue: { list: () => of(options.versions ?? []) } },
      MessageService,
    ],
  };
}

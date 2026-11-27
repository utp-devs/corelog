import type { ProjectTemplate, ProjectType } from '@corelog/core';

/** Where a status of a template sits in the flow; drives the color of its preview chip. */
export type TemplateStatusCategory = 'todo' | 'in_progress' | 'done';

export interface TemplateStatus {
  readonly name: string;
  readonly category: TemplateStatusCategory;
}

/** What a template gives a new project. Mirrors the templates the server applies at creation. */
export interface ProjectTemplateInfo {
  readonly value: ProjectTemplate;
  readonly label: string;
  readonly description: string;
  /** Legacy project type sent with the template. */
  readonly type: ProjectType;
  readonly sprints: boolean;
  readonly icon: string;
  readonly statuses: readonly TemplateStatus[];
  readonly issueTypes: readonly string[];
}

const STANDARD_STATUSES: readonly TemplateStatus[] = [
  { name: 'Por hacer', category: 'todo' },
  { name: 'En curso', category: 'in_progress' },
  { name: 'En revisión', category: 'in_progress' },
  { name: 'Hecho', category: 'done' },
];

const STANDARD_ISSUE_TYPES = ['Épica', 'Historia', 'Tarea', 'Error', 'Subtarea'];

export const PROJECT_TEMPLATES: readonly ProjectTemplateInfo[] = [
  {
    value: 'scrum',
    label: 'Scrum',
    description: 'Backlog, sprints y tablero para equipos que planifican por iteraciones.',
    type: 'scrum',
    sprints: true,
    icon: 'pi pi-sync',
    statuses: STANDARD_STATUSES,
    issueTypes: STANDARD_ISSUE_TYPES,
  },
  {
    value: 'kanban',
    label: 'Kanban',
    description: 'Tablero de flujo continuo, sin sprints.',
    type: 'kanban',
    sprints: false,
    icon: 'pi pi-table',
    statuses: STANDARD_STATUSES,
    issueTypes: STANDARD_ISSUE_TYPES,
  },
  {
    value: 'task_tracking',
    label: 'Seguimiento de tareas',
    description: 'Flujo simple de tres estados para listas de tareas, sin sprints.',
    type: 'kanban',
    sprints: false,
    icon: 'pi pi-check-square',
    statuses: [
      { name: 'Por hacer', category: 'todo' },
      { name: 'En curso', category: 'in_progress' },
      { name: 'Hecho', category: 'done' },
    ],
    issueTypes: ['Tarea', 'Subtarea'],
  },
  {
    value: 'bug_tracking',
    label: 'Seguimiento de errores',
    description: 'Tipos centrados en errores y un flujo de cinco estados.',
    type: 'kanban',
    sprints: false,
    icon: 'pi pi-exclamation-triangle',
    statuses: [
      { name: 'Nuevo', category: 'todo' },
      { name: 'Confirmado', category: 'todo' },
      { name: 'En curso', category: 'in_progress' },
      { name: 'Resuelto', category: 'done' },
      { name: 'Cerrado', category: 'done' },
    ],
    issueTypes: ['Error', 'Regresión', 'Incidencia', 'Mejora'],
  },
];

export const TEMPLATE_LABELS: Readonly<Record<ProjectTemplate, string>> = {
  scrum: 'Scrum',
  kanban: 'Kanban',
  task_tracking: 'Seguimiento de tareas',
  bug_tracking: 'Seguimiento de errores',
};

export function templateInfo(template: ProjectTemplate): ProjectTemplateInfo {
  return PROJECT_TEMPLATES.find((item) => item.value === template) ?? PROJECT_TEMPLATES[0];
}

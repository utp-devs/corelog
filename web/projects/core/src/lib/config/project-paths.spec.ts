import { PROJECTS_PATH, PROJECT_SECTIONS, projectPath } from './project-paths';
import { buildProjectNav, hasSprints } from './project-nav';

describe('projectPath', () => {
  it('uses the key when it is known', () => {
    expect(projectPath({ id: 'uuid-1', key: 'CORE' })).toBe('/projects/CORE');
    expect(projectPath({ id: 'uuid-1', key: 'CORE' }, 'board')).toBe('/projects/CORE/board');
  });

  it('falls back to the id when the key is empty or missing', () => {
    expect(projectPath({ id: 'uuid-1', key: '' }, 'sprints')).toBe('/projects/uuid-1/sprints');
    expect(projectPath({ id: 'uuid-1' })).toBe('/projects/uuid-1');
    expect(projectPath({ id: 'uuid-1', key: null })).toBe('/projects/uuid-1');
  });

  it('takes a plain string as the identifier', () => {
    expect(projectPath('CORE', 'calendar')).toBe('/projects/CORE/calendar');
  });

  it('encodes the identifier', () => {
    expect(projectPath('A/B')).toBe('/projects/A%2FB');
  });

  it('exposes the base path and the sections', () => {
    expect(PROJECTS_PATH).toBe('/projects');
    expect(PROJECT_SECTIONS).toContain('board');
  });
});

describe('hasSprints', () => {
  it('follows the template, and the legacy type when there is none', () => {
    expect(hasSprints({ template: 'scrum', type: 'scrum' })).toBe(true);
    expect(hasSprints({ template: 'bug_tracking', type: 'kanban' })).toBe(false);
    expect(hasSprints({ type: 'scrum' })).toBe(true);
    expect(hasSprints({ type: 'kanban' })).toBe(false);
  });
});

describe('buildProjectNav', () => {
  const scrum = { id: 'u', key: 'CORE', template: 'scrum', type: 'scrum' } as const;
  const kanban = { id: 'u', key: 'CORE', template: 'kanban', type: 'kanban' } as const;
  const everything = (): boolean => true;

  it('lists every section of a scrum project with key based paths', () => {
    const items = buildProjectNav(scrum, everything);

    expect(items.map((item) => item.key)).toEqual([
      'board',
      'backlog',
      'sprints',
      'calendar',
      'settings',
    ]);
    expect(items.map((item) => item.path)).toEqual([
      '/projects/CORE/board',
      '/projects/CORE/backlog',
      '/projects/CORE/sprints',
      '/projects/CORE/calendar',
      '/projects/CORE/settings',
    ]);
    expect(items[0].label).toBe('Tablero');
  });

  it('hides backlog and sprints for non scrum templates', () => {
    expect(buildProjectNav(kanban, everything).map((item) => item.key)).toEqual([
      'board',
      'calendar',
      'settings',
    ]);
  });

  it('hides the entries whose permission is missing', () => {
    expect(buildProjectNav(scrum, () => false)).toEqual([]);
  });

  it('asks for browse_project and edit_project (settings)', () => {
    const asked: string[] = [];
    buildProjectNav(kanban, (permission) => {
      asked.push(permission);
      return true;
    });
    expect(new Set(asked)).toEqual(new Set(['browse_project', 'edit_project']));
  });
});

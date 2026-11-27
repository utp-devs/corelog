import { PROJECT_COVERS, projectCover } from './project-cover';

describe('projectCover', () => {
  it('is deterministic for the same color', () => {
    expect(projectCover('#0052cc', 'CL')).toBe(projectCover('#0052cc', 'OTHER'));
  });

  it('falls back to the project key when there is no color', () => {
    expect(projectCover(undefined, 'CL')).toBe(projectCover('', 'cl'));
  });

  it('always returns one of the six covers', () => {
    for (const key of ['A', 'CL', 'WEB', 'API', 'OPS', 'DATA', 'MOBILE', 'QA']) {
      expect(PROJECT_COVERS).toContain(projectCover(undefined, key));
    }
  });

  it('spreads different keys over several covers', () => {
    const used = new Set(
      ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J'].map((k) => projectCover(null, k)),
    );
    expect(used.size).toBeGreaterThan(2);
  });
});

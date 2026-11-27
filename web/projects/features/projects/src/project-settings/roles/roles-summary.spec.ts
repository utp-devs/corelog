import { describe, expect, it } from 'vitest';

import { SAMPLE_GRANTS, schemeOf } from '../settings-test-kit';
import { summarizeRole } from './roles-summary';

describe('summarizeRole', () => {
  const scheme = schemeOf(SAMPLE_GRANTS);

  it('reads what each role can do from the scheme', () => {
    const viewer = summarizeRole(scheme, 'viewer');
    expect(viewer.total).toBe(1);
    expect(viewer.groups[0].abilities[0].label).toBe('Ver el proyecto');
    expect(summarizeRole(scheme, 'lead').total).toBe(6);
  });

  it('follows a customized scheme', () => {
    const custom = schemeOf(
      { ...SAMPLE_GRANTS, comment_issue: ['lead', 'contributor', 'viewer'] },
      false,
    );
    const labels = summarizeRole(custom, 'viewer').groups.flatMap((group) =>
      group.abilities.map((ability) => ability.label),
    );
    expect(labels).toContain('Comentar');
  });
});

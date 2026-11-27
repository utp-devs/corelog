import { FormControl } from '@angular/forms';
import { describe, expect, it } from 'vitest';

import { teamNameValidator } from './team-form';

describe('teamNameValidator', () => {
  const check = (value: string) => teamNameValidator(new FormControl(value));

  it('accepts names of 2 to 120 characters after trimming', () => {
    expect(check('Ab')).toBeNull();
    expect(check('  Ab  ')).toBeNull();
    expect(check('a'.repeat(120))).toBeNull();
  });

  it('rejects short, blank and long names', () => {
    expect(check('A')).toEqual({ length: true });
    expect(check('   ')).toEqual({ length: true });
    expect(check('a'.repeat(121))).toEqual({ length: true });
  });
});

import { DestroyRef } from '@angular/core';
import { FormControl, FormGroup } from '@angular/forms';

import {
  normalizeName,
  passwordMatchValidator,
  personNameValidator,
  revalidateOnPasswordChange,
} from './identity.validators';

describe('personNameValidator', () => {
  const validator = personNameValidator(10);
  const check = (value: string) => validator(new FormControl(value));

  it('requires a value once trimmed', () => {
    expect(check('')).toEqual({ required: true });
    expect(check('   ')).toEqual({ required: true });
  });

  it('accepts letters with accents, spaces, hyphens and apostrophes', () => {
    expect(check('María José')).toBeNull();
    expect(check("O'Brien")).toBeNull();
    expect(check('Anne-Marie')).toBeNull();
    expect(check('  Ñandú  ')).toBeNull();
  });

  it('rejects digits and symbols', () => {
    expect(check('Ada1')).toEqual({ pattern: true });
    expect(check('Ada@')).toEqual({ pattern: true });
    expect(check('-Ada')).toEqual({ pattern: true });
  });

  it('limits the length after normalizing the spaces', () => {
    expect(check('a'.repeat(10))).toBeNull();
    expect(check('a'.repeat(11))?.['maxlength']).toEqual({ requiredLength: 10, actualLength: 11 });
    expect(check('ab    cd')).toBeNull();
  });

  it('collapses inner spaces', () => {
    expect(normalizeName('  Ada    María ')).toBe('Ada María');
  });
});

describe('password confirmation', () => {
  function form(): FormGroup<{ password: FormControl<string>; confirm: FormControl<string> }> {
    return new FormGroup({
      password: new FormControl('', { nonNullable: true }),
      confirm: new FormControl('', {
        nonNullable: true,
        validators: [passwordMatchValidator('password')],
      }),
    });
  }

  it('flags a confirmation that differs and clears it when it matches', () => {
    const group = form();
    group.controls.password.setValue('Abcdef12!');
    group.controls.confirm.setValue('Abcdef12');
    expect(group.controls.confirm.errors).toEqual({ mismatch: true });
    group.controls.confirm.setValue('Abcdef12!');
    expect(group.controls.confirm.errors).toBeNull();
  });

  it('leaves an empty confirmation to the required validator', () => {
    const group = form();
    group.controls.password.setValue('Abcdef12!');
    expect(group.controls.confirm.errors).toBeNull();
  });

  it('re-evaluates the confirmation when the password changes afterwards', () => {
    const group = form();
    const callbacks: (() => void)[] = [];
    revalidateOnPasswordChange(group, 'password', 'confirm', {
      onDestroy: (fn: () => void) => callbacks.push(fn),
    } as unknown as DestroyRef);

    group.controls.password.setValue('Abcdef12!');
    group.controls.confirm.setValue('Abcdef12!');
    expect(group.controls.confirm.valid).toBe(true);

    group.controls.password.setValue('Otra-clave-1');
    expect(group.controls.confirm.errors).toEqual({ mismatch: true });
    group.controls.password.setValue('Abcdef12!');
    expect(group.controls.confirm.valid).toBe(true);

    callbacks.forEach((fn) => fn());
    group.controls.password.setValue('x');
    expect(group.controls.confirm.valid).toBe(true);
  });
});

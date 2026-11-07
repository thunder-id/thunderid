// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {
  createVariable,
  defaultValueFor,
  findProblems,
  formatOptions,
  isValidIdentifier,
  parseOptions,
} from '../variableDefinitions';

describe('variableDefinitions', () => {
  describe('isValidIdentifier', () => {
    it('starts with a letter and holds letters, digits and underscores', () => {
      expect(isValidIdentifier('supportEmail')).toBe(true);
      expect(isValidIdentifier('otp_ttl_2')).toBe(true);
      expect(isValidIdentifier('2fa')).toBe(false);
      expect(isValidIdentifier('has space')).toBe(false);
      expect(isValidIdentifier('')).toBe(false);
    });
  });

  describe('createVariable', () => {
    it('gives each new variable an identifier no other has', () => {
      const first = createVariable([]);
      const second = createVariable([first, {identifier: 'variable3', type: 'TEXT_INPUT'}]);

      expect(first.identifier).toBe('variable1');
      expect(second.identifier).toBe('variable4');
      expect(second.type).toBe('TEXT_INPUT');
    });
  });

  describe('defaultValueFor', () => {
    it('starts every type with a value that is valid for it', () => {
      expect(defaultValueFor('NUMBER_INPUT')).toBe(0);
      expect(defaultValueFor('BOOLEAN_INPUT')).toBe(false);
      expect(defaultValueFor('TEXT_INPUT')).toBe('');
      expect(
        findProblems([{defaultValue: defaultValueFor('EMAIL_INPUT'), identifier: 'e', type: 'EMAIL_INPUT'}], 0),
      ).toEqual([]);
      expect(
        findProblems([{defaultValue: defaultValueFor('DATE_INPUT'), identifier: 'd', type: 'DATE_INPUT'}], 0),
      ).toEqual([]);
    });
  });

  describe('parseOptions', () => {
    it('reads one option per line and drops blanks', () => {
      expect(parseOptions('SMS_OTP\n\n  EMAIL_OTP  \n')).toEqual(['SMS_OTP', 'EMAIL_OTP']);
    });

    it('makes all numbers a number and all booleans a boolean', () => {
      expect(parseOptions('30\n60')).toEqual([30, 60]);
      expect(parseOptions('true\nfalse')).toEqual([true, false]);
      expect(parseOptions('30\nabc')).toEqual(['30', 'abc']);
    });

    it('has no options for empty text', () => {
      expect(parseOptions('')).toEqual([]);
    });
  });

  describe('formatOptions', () => {
    it('writes one option per line', () => {
      expect(formatOptions(['a', 2, true])).toBe('a\n2\ntrue');
      expect(formatOptions(undefined)).toBe('');
    });
  });

  describe('findProblems', () => {
    it('flags a bad or repeated identifier', () => {
      const defs = [
        {defaultValue: 'x', identifier: 'a b', type: 'TEXT_INPUT' as const},
        {defaultValue: 'x', identifier: 'same', type: 'TEXT_INPUT' as const},
        {defaultValue: 'x', identifier: 'same', type: 'TEXT_INPUT' as const},
      ];

      expect(findProblems(defs, 0)).toContain('identifier');
      expect(findProblems(defs, 1)).toContain('duplicate');
    });

    it('flags a choice with no options', () => {
      expect(findProblems([{defaultValue: '', identifier: 's', type: 'SELECT'}], 0)).toContain('options');
    });

    it('lets a text variable start blank, as the server does', () => {
      expect(findProblems([{defaultValue: '', identifier: 't', type: 'TEXT_INPUT'}], 0)).toEqual([]);
      expect(
        findProblems(
          [{defaultValue: '', identifier: 't', type: 'TEXT_INPUT', validation: [{type: 'maxLength', value: 10}]}],
          0,
        ),
      ).toEqual([]);
    });

    it('still flags a blank text default that a rule would break', () => {
      expect(
        findProblems(
          [{defaultValue: '', identifier: 't', type: 'TEXT_INPUT', validation: [{type: 'minLength', value: 3}]}],
          0,
        ),
      ).toEqual(['default']);
      expect(
        findProblems(
          [{defaultValue: '', identifier: 't', type: 'TEXT_INPUT', validation: [{type: 'pattern', value: '^a'}]}],
          0,
        ),
      ).toEqual(['default']);
    });

    it('still flags a blank default for the types that have no blank value', () => {
      expect(findProblems([{defaultValue: '', identifier: 'e', type: 'EMAIL_INPUT'}], 0)).toEqual(['default']);
      expect(findProblems([{defaultValue: '', identifier: 'p', type: 'PHONE_INPUT'}], 0)).toEqual(['default']);
      expect(findProblems([{defaultValue: '', identifier: 'd', type: 'DATE_INPUT'}], 0)).toEqual(['default']);
      expect(findProblems([{defaultValue: '', identifier: 's', options: ['a'], type: 'SELECT'}], 0)).toEqual([
        'default',
      ]);
    });

    it('lets a connection variable start with no connection', () => {
      expect(
        findProblems(
          [{category: 'IDENTITY_PROVIDER', defaultValue: '', identifier: 'c', type: 'CONNECTION_SELECT'}],
          0,
        ),
      ).toEqual([]);
    });

    it('flags a default that does not fit the type or its rules', () => {
      expect(findProblems([{defaultValue: 'nope', identifier: 'e', type: 'EMAIL_INPUT'}], 0)).toEqual(['default']);
      expect(
        findProblems(
          [{defaultValue: 'abcdef', identifier: 't', type: 'TEXT_INPUT', validation: [{type: 'maxLength', value: 3}]}],
          0,
        ),
      ).toEqual(['default']);
    });

    it('accepts a valid variable', () => {
      expect(findProblems([{defaultValue: 'ok', identifier: 't', type: 'TEXT_INPUT'}], 0)).toEqual([]);
    });
  });
});

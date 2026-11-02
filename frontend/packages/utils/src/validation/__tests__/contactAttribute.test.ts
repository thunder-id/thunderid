// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {
  CONTACT_ATTRIBUTE_REGEX,
  getContactTypesMissingRegex,
  isContactAttributeType,
  resolveAttributeRegex,
} from '../contactAttribute';

describe('CONTACT_ATTRIBUTE_REGEX', () => {
  it.each(['User+tag@example.com', 'a.b@sub.example.org'])('should accept email %s', (value) => {
    expect(new RegExp(CONTACT_ATTRIBUTE_REGEX.email).test(value)).toBe(true);
  });

  it.each(['\u00a0', '\u000b', '\u2003', '\u2028', '\u2029', '\ufeff'])(
    'should reject email whitespace %j consistently with the backend',
    (space) => {
      expect(new RegExp(CONTACT_ATTRIBUTE_REGEX.email).test(`person${space}@example.com`)).toBe(false);
    },
  );

  it.each([' ', '\t', '\n', '\f', '\r'])('should reject Go whitespace %j in email', (space) => {
    expect(new RegExp(CONTACT_ATTRIBUTE_REGEX.email).test(`person${space}@example.com`)).toBe(false);
  });

  it.each(['Name <user@example.com>', 'a b@example.com', 'a@b.com\n', ''])('should reject email "%s"', (value) => {
    expect(new RegExp(CONTACT_ATTRIBUTE_REGEX.email).test(value)).toBe(false);
  });

  it.each(['+12', '+94771234567', '+123456789012345'])('should accept mobile %s', (value) => {
    expect(new RegExp(CONTACT_ATTRIBUTE_REGEX.mobile).test(value)).toBe(true);
  });

  it.each(['0771234567', '+0123', '+1', '+1234567890123456', '+12 345', '+12\n', ''])(
    'should reject mobile "%s"',
    (value) => {
      expect(new RegExp(CONTACT_ATTRIBUTE_REGEX.mobile).test(value)).toBe(false);
    },
  );
});

describe('isContactAttributeType', () => {
  it.each(['email', 'mobile'])('should return true for %s', (type) => {
    expect(isContactAttributeType(type)).toBe(true);
  });

  it.each(['string', 'number', 'boolean', 'object', 'array', undefined])('should return false for %s', (type) => {
    expect(isContactAttributeType(type)).toBe(false);
  });
});

describe('resolveAttributeRegex', () => {
  it('should return the saved regex', () => {
    expect(resolveAttributeRegex('email', '@example.com$')).toBe('@example.com$');
  });

  it.each(['email', 'mobile'] as const)('should return the default %s regex when none is saved', (type) => {
    expect(resolveAttributeRegex(type, '')).toBe(CONTACT_ATTRIBUTE_REGEX[type]);
    expect(resolveAttributeRegex(type)).toBe(CONTACT_ATTRIBUTE_REGEX[type]);
  });

  it('should return an empty string for other types without a saved regex', () => {
    expect(resolveAttributeRegex('string')).toBe('');
  });
});

describe('getContactTypesMissingRegex', () => {
  it('should return contact types with a named attribute that has no regex', () => {
    expect(
      getContactTypesMissingRegex([
        {name: 'work', type: 'email', regex: ' '},
        {name: 'phone', type: 'mobile', regex: ''},
        {name: 'nickname', type: 'string', regex: ''},
      ]),
    ).toEqual(['email', 'mobile']);
  });

  it('should ignore unnamed attributes and attributes with a regex', () => {
    expect(
      getContactTypesMissingRegex([
        {name: ' ', type: 'email', regex: ''},
        {name: 'phone', type: 'mobile', regex: '^\\+\\d+$'},
      ]),
    ).toEqual([]);
  });
});

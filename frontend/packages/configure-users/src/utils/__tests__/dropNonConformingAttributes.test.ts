// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {PropertyDefinition, UserTypeDefinition} from '../../models/users';
import {attributeConformsToSchema, dropNonConformingOptionalAttributes} from '../dropNonConformingAttributes';

describe('attributeConformsToSchema', () => {
  it('checks primitive types', () => {
    expect(attributeConformsToSchema('hi', {type: 'string'})).toBe(true);
    expect(attributeConformsToSchema(5, {type: 'string'})).toBe(false);
    expect(attributeConformsToSchema(5, {type: 'number'})).toBe(true);
    expect(attributeConformsToSchema('5', {type: 'number'})).toBe(false);
    expect(attributeConformsToSchema(true, {type: 'boolean'})).toBe(true);
    expect(attributeConformsToSchema('true', {type: 'boolean'})).toBe(false);
    expect(attributeConformsToSchema([1], {type: 'array', items: {type: 'number'}})).toBe(true);
    expect(attributeConformsToSchema('x', {type: 'array', items: {type: 'number'}})).toBe(false);
    expect(attributeConformsToSchema({a: 1}, {type: 'object', properties: {}})).toBe(true);
    expect(attributeConformsToSchema([1], {type: 'object', properties: {}})).toBe(false);
  });

  it.each([
    ['email', 'User+tag@example.com', true],
    ['email', 'a@localhost', false],
    ['email', 'a@[127.0.0.1]', true],
    ['email', 'a@[::1]', false],
    ['email', 'not-an-email', false],
    ['email', 'Name <a@b.com>', false],
    ['email', ' a@b.com', false],
    ['email', 'a@b.com\n', false],
    ['email', 'ü@b.com', true],
    ['email', 'a@例.com', true],
    ['email', 'a..b@example.com', true],
    ['email', 'a@b..com', true],
    ['email', 'a@[256.0.0.1]', true],
    ['email', 'a@[not:ip]', false],
    ['mobile', '+12', true],
    ['mobile', '+123456789012345', true],
    ['mobile', '+1234567890123456', false],
    ['mobile', '555-1234', false],
    ['mobile', '+01', false],
    ['mobile', '+1', false],
    ['mobile', '+12 345', false],
    ['mobile', '+12\n', false],
  ] as const)('checks %s contact format for %j', (type, value, valid) => {
    expect(attributeConformsToSchema(value, {type})).toBe(valid);
  });

  it.each(['email', 'mobile'] as const)('uses the saved %s regex instead of the default', (type) => {
    const customValue = type === 'email' ? 'a@localhost' : '0771234567';
    const regex = type === 'email' ? '^.+@localhost$' : '^0[0-9]{9}$';
    const defaultValue = type === 'email' ? 'a@example.com' : '+94771234567';
    expect(attributeConformsToSchema(customValue, {type, regex})).toBe(true);
    expect(attributeConformsToSchema(defaultValue, {type, regex})).toBe(false);
    expect(attributeConformsToSchema(customValue, {type})).toBe(false);
    expect(attributeConformsToSchema(customValue, {type, regex: ''})).toBe(false);
    expect(attributeConformsToSchema(defaultValue, {type, regex: ''})).toBe(true);
    expect(attributeConformsToSchema(customValue, {type, regex: '('})).toBe(true);
    expect(attributeConformsToSchema('', {type, regex: '.*'})).toBe(false);
    expect(attributeConformsToSchema(7, {type, regex: '.*'})).toBe(false);
  });

  it('checks enum membership', () => {
    const def: PropertyDefinition = {type: 'string', enum: ['ACTIVE', 'INACTIVE']};
    expect(attributeConformsToSchema('ACTIVE', def)).toBe(true);
    expect(attributeConformsToSchema('PENDING', def)).toBe(false);
  });

  it('checks regex and tolerates an unparseable pattern', () => {
    expect(attributeConformsToSchema('abc', {type: 'string', regex: '^[a-z]+$'})).toBe(true);
    expect(attributeConformsToSchema('ABC', {type: 'string', regex: '^[a-z]+$'})).toBe(false);
    // An invalid schema regex can't judge the value, so it is not dropped.
    expect(attributeConformsToSchema('abc', {type: 'string', regex: '('})).toBe(true);
  });
});

describe('dropNonConformingOptionalAttributes', () => {
  const schema: UserTypeDefinition = {
    age: {type: 'number'},
    nickname: {type: 'string'},
    email: {type: 'string', required: true},
  };

  it('drops an optional attribute whose value no longer matches the schema', () => {
    const result = dropNonConformingOptionalAttributes({age: 'not-a-number', nickname: 'jo'}, schema);
    expect(result).toEqual({nickname: 'jo'});
  });

  it('keeps a required attribute even when its value no longer matches', () => {
    const result = dropNonConformingOptionalAttributes({email: 12345}, schema);
    expect(result).toEqual({email: 12345});
  });

  it('keeps conforming values', () => {
    const result = dropNonConformingOptionalAttributes({age: 30, nickname: 'jo'}, schema);
    expect(result).toEqual({age: 30, nickname: 'jo'});
  });

  it('leaves undeclared keys untouched (backend strips those)', () => {
    const result = dropNonConformingOptionalAttributes({stale: 'x', age: 30}, schema);
    expect(result).toEqual({stale: 'x', age: 30});
  });

  it('returns the input unchanged when no schema is available', () => {
    const attrs = {age: 'not-a-number'};
    expect(dropNonConformingOptionalAttributes(attrs, undefined)).toBe(attrs);
  });
  it('drops stale optional contact strings and keeps invalid required contacts', () => {
    const contactSchema: UserTypeDefinition = {
      email: {type: 'email'},
      phone: {type: 'mobile'},
      requiredEmail: {type: 'email', required: true},
      requiredPhone: {type: 'mobile', required: true},
      nickname: {type: 'string'},
    };
    expect(
      dropNonConformingOptionalAttributes(
        {email: 'not-an-email', phone: '555-1234', requiredEmail: 'bad', requiredPhone: 'bad', nickname: 'jo'},
        contactSchema,
      ),
    ).toEqual({requiredEmail: 'bad', requiredPhone: 'bad', nickname: 'jo'});
  });

  it('preserves optional contacts accepted by the saved custom regex', () => {
    expect(
      dropNonConformingOptionalAttributes(
        {email: 'a@localhost', phone: '0771234567'},
        {email: {type: 'email', regex: '^.+@localhost$'}, phone: {type: 'mobile', regex: '^0[0-9]{9}$'}},
      ),
    ).toEqual({email: 'a@localhost', phone: '0771234567'});
  });

  it('omits cleared optional contacts while retaining required values for validation', () => {
    const contactSchema: UserTypeDefinition = {
      email: {type: 'email'},
      phone: {type: 'mobile'},
      requiredEmail: {type: 'email', required: true},
    };
    expect(dropNonConformingOptionalAttributes({email: '', phone: '', requiredEmail: ''}, contactSchema)).toEqual({
      requiredEmail: '',
    });
    expect(dropNonConformingOptionalAttributes({email: 'User@example.com', phone: '+123'}, contactSchema)).toEqual({
      email: 'User@example.com',
      phone: '+123',
    });
  });
});

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {SchemaPropertyInput} from '../../models/property-definition';
import convertPropertiesToSchema from '../convertPropertiesToSchema';

const property = (overrides: Partial<SchemaPropertyInput>): SchemaPropertyInput => ({
  id: '0',
  name: 'field',
  displayName: '',
  type: 'string',
  required: false,
  unique: false,
  credential: false,
  enum: [],
  regex: '',
  ...overrides,
});

describe('convertPropertiesToSchema', () => {
  it('should skip properties without a name and trim the names it keeps', () => {
    expect(convertPropertiesToSchema([property({name: '  '}), property({name: ' model '})])).toEqual({
      model: {type: 'string', required: false},
    });
  });

  it('should keep the trimmed display name, unique, and credential flags', () => {
    expect(
      convertPropertiesToSchema([
        property({name: 'name', displayName: ' Name ', required: true, unique: true}),
        property({name: 'secret', type: 'number', credential: true}),
      ]),
    ).toEqual({
      name: {type: 'string', required: true, displayName: 'Name', unique: true},
      secret: {type: 'number', required: false, credential: true},
    });
  });

  it('should store an enum as a string with its allowed values', () => {
    expect(convertPropertiesToSchema([property({name: 'tier', type: 'enum', enum: ['a', 'b']})])).toEqual({
      tier: {type: 'string', required: false, enum: ['a', 'b']},
    });
  });

  it('should keep the regex for strings and always for contact types', () => {
    expect(
      convertPropertiesToSchema([
        property({name: 'code', regex: '^[A-Z]+$'}),
        property({name: 'plain'}),
        property({name: 'mail', type: 'email', regex: ''}),
      ]),
    ).toEqual({
      code: {type: 'string', required: false, regex: '^[A-Z]+$'},
      plain: {type: 'string', required: false},
      mail: {type: 'email', required: false, regex: ''},
    });
  });

  it('should not mark booleans as credentials', () => {
    expect(convertPropertiesToSchema([property({name: 'active', type: 'boolean', credential: true})])).toEqual({
      active: {type: 'boolean', required: false},
    });
  });

  it('should keep array items and object properties, defaulting when absent', () => {
    expect(
      convertPropertiesToSchema([
        property({name: 'tags', type: 'array'}),
        property({name: 'meta', type: 'object', properties: {team: {type: 'string'}}}),
      ]),
    ).toEqual({
      tags: {type: 'array', required: false, items: {type: 'string'}},
      meta: {type: 'object', required: false, properties: {team: {type: 'string'}}},
    });
  });
});

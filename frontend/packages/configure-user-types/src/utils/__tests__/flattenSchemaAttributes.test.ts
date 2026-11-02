// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {UserTypeDefinition} from '../../types/user-types';
import flattenSchemaAttributes from '../flattenSchemaAttributes';

describe('flattenSchemaAttributes', () => {
  it('returns an empty list for an undefined schema', () => {
    expect(flattenSchemaAttributes(undefined)).toEqual([]);
  });

  it('flattens primitive attributes', () => {
    const schema: UserTypeDefinition = {
      username: {type: 'string'},
      age: {type: 'number'},
      active: {type: 'boolean'},
    };

    expect(flattenSchemaAttributes(schema)).toEqual([
      {attribute: 'username', credential: false},
      {attribute: 'age', credential: false},
      {attribute: 'active', credential: false},
    ]);
  });

  it('flattens nested object attributes using dot notation', () => {
    const schema: UserTypeDefinition = {
      address: {
        type: 'object',
        properties: {
          city: {type: 'string'},
          geo: {type: 'object', properties: {lat: {type: 'number'}}},
        },
      },
    };

    expect(flattenSchemaAttributes(schema).map((attribute) => attribute.attribute)).toEqual([
      'address.city',
      'address.geo.lat',
    ]);
  });

  it('skips array attributes', () => {
    const schema: UserTypeDefinition = {
      username: {type: 'string'},
      roles: {type: 'array', items: {type: 'string'}},
    };

    expect(flattenSchemaAttributes(schema).map((attribute) => attribute.attribute)).toEqual(['username']);
  });

  it('tags credential attributes instead of dropping them', () => {
    const schema: UserTypeDefinition = {
      username: {type: 'string'},
      password: {type: 'string', credential: true},
    };

    expect(flattenSchemaAttributes(schema)).toEqual([
      {attribute: 'username', credential: false},
      {attribute: 'password', credential: true},
    ]);
  });

  it('applies the prefix to top level attributes', () => {
    const schema: UserTypeDefinition = {city: {type: 'string'}};

    expect(flattenSchemaAttributes(schema, 'address.')).toEqual([{attribute: 'address.city', credential: false}]);
  });
});

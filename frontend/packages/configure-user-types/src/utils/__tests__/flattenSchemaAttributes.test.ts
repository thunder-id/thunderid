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
      {attribute: 'username', credential: false, unique: false},
      {attribute: 'age', credential: false, unique: false},
      {attribute: 'active', credential: false, unique: false},
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
      password: {type: 'string', credential: true, unique: false},
    };

    expect(flattenSchemaAttributes(schema)).toEqual([
      {attribute: 'username', credential: false, unique: false},
      {attribute: 'password', credential: true, unique: false},
    ]);
  });

  it('tags unique attributes', () => {
    const schema: UserTypeDefinition = {
      email: {type: 'string', unique: true},
      given_name: {type: 'string'},
    };

    expect(flattenSchemaAttributes(schema)).toEqual([
      {attribute: 'email', credential: false, unique: true},
      {attribute: 'given_name', credential: false, unique: false},
    ]);
  });

  it('applies the prefix to top level attributes', () => {
    const schema: UserTypeDefinition = {city: {type: 'string'}};

    expect(flattenSchemaAttributes(schema, 'address.')).toEqual([
      {attribute: 'address.city', credential: false, unique: false},
    ]);
  });
});

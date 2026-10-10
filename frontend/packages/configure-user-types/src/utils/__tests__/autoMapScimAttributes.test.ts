// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {SchemaPropertyInput} from '../../types/user-types';
import autoMapScimAttributes from '../autoMapScimAttributes';

const property = (name: string, overrides: Partial<SchemaPropertyInput> = {}): SchemaPropertyInput => ({
  id: name,
  name,
  displayName: '',
  type: 'string',
  required: false,
  unique: false,
  credential: false,
  enum: [],
  regex: '',
  ...overrides,
});

describe('autoMapScimAttributes', () => {
  it('maps attribute library properties to their SCIM targets', () => {
    const result = autoMapScimAttributes(
      [property('username'), property('email'), property('given_name'), property('family_name'), property('city')],
      {},
    );

    expect(result).toEqual({
      username: 'userName',
      email: 'emails',
      given_name: 'name.givenName',
      family_name: 'name.familyName',
      city: 'addresses.locality',
    });
  });

  it('leaves properties that are not in the library unmapped', () => {
    expect(autoMapScimAttributes([property('favoriteColor'), property('gender')], {})).toEqual({});
  });

  it('never changes an existing mapping or takes a target already in use', () => {
    const result = autoMapScimAttributes([property('email'), property('username')], {email: 'userName'});

    expect(result['email']).toBe('userName');
    expect(result['username']).toBeUndefined();
  });

  it('maps the password and mobile number attributes of the default user types', () => {
    const result = autoMapScimAttributes(
      [property('password', {credential: true}), property('mobile_number'), property('sub')],
      {},
    );

    expect(result).toEqual({password: 'password', mobile_number: 'phoneNumbers'});
  });

  it('skips non-scalar properties', () => {
    const result = autoMapScimAttributes([property('country', {type: 'object', properties: {}})], {});

    expect(result).toEqual({});
  });
});

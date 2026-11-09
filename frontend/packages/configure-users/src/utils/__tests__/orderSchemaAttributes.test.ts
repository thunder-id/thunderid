// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {PropertyDefinition} from '../../models/users';
import orderSchemaAttributes from '../orderSchemaAttributes';

const names = (schema: Record<string, PropertyDefinition>, includeOptional = false): string[] =>
  orderSchemaAttributes(schema, {includeOptional}).map(([name]) => name);

describe('orderSchemaAttributes', () => {
  it('orders leading attributes, other required attributes, required credentials and the password', () => {
    expect(
      names({
        email: {type: 'email', required: true},
        department: {type: 'string', required: true},
        family_name: {type: 'string'},
        given_name: {type: 'string'},
        mobile_number: {type: 'mobile'},
        password: {type: 'string', credential: true},
        pin: {type: 'string', required: true, credential: true},
        username: {type: 'string', required: true},
      }),
    ).toEqual(['username', 'given_name', 'family_name', 'email', 'mobile_number', 'department', 'pin', 'password']);
  });

  it('places a required password first among the required credentials', () => {
    expect(
      names({
        pin: {type: 'string', required: true, credential: true},
        password: {type: 'string', required: true, credential: true},
      }),
    ).toEqual(['password', 'pin']);
  });

  it('drops optional attributes other than the leading ones and the password', () => {
    expect(
      names({
        username: {type: 'string', required: true},
        sub: {type: 'string'},
        picture: {type: 'string'},
        recovery_code: {type: 'string', credential: true},
      }),
    ).toEqual(['username']);
  });

  it('skips leading attributes the schema does not define', () => {
    expect(names({email: {type: 'email', required: true}, password: {type: 'string', credential: true}})).toEqual([
      'email',
      'password',
    ]);
  });

  it('keeps other optional attributes after the required ones when included', () => {
    expect(
      names(
        {
          sub: {type: 'string'},
          email: {type: 'email', required: true},
          password: {type: 'string', credential: true},
          department: {type: 'string', required: true},
          recovery_code: {type: 'string', credential: true},
          username: {type: 'string', required: true},
          picture: {type: 'string'},
        },
        true,
      ),
    ).toEqual(['username', 'email', 'department', 'sub', 'picture', 'password', 'recovery_code']);
  });

  it('keeps optional attributes when none other than credentials would be shown', () => {
    expect(
      names({
        nickname: {type: 'string'},
        password: {type: 'string', credential: true},
        department: {type: 'string'},
      }),
    ).toEqual(['nickname', 'department', 'password']);
  });
});

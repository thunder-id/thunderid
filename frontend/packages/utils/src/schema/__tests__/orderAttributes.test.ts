// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {AttributeDisplay} from '../attributeDisplay';
import orderAttributes from '../orderAttributes';

interface Definition {
  type: string;
  required?: boolean;
  credential?: boolean;
  display?: AttributeDisplay;
}

const names = (schema: Record<string, Definition>, options?: {onCreate?: boolean; leading?: string}): string[] =>
  orderAttributes(schema, options).map(([name]) => name);

describe('orderAttributes', () => {
  it('orders attributes without a display order as required, optional, then credentials, each by name', () => {
    expect(
      names({
        pin: {type: 'string', credential: true},
        nickname: {type: 'string'},
        username: {type: 'string', required: true},
        password: {type: 'string', required: true, credential: true},
        email: {type: 'email', required: true},
        address: {type: 'string'},
      }),
    ).toEqual(['email', 'username', 'address', 'nickname', 'password', 'pin']);
  });

  it('puts attributes with a display order first, lowest first and by name for the same order', () => {
    expect(
      names({
        email: {type: 'email', required: true},
        nickname: {type: 'string', display: {order: 1}},
        alias: {type: 'string', display: {order: 1}},
        password: {type: 'string', credential: true, display: {order: 0}},
        address: {type: 'string'},
      }),
    ).toEqual(['password', 'alias', 'nickname', 'email', 'address']);
  });

  it('keeps every attribute unless ordering for creation', () => {
    const schema = {
      email: {type: 'email', required: true, display: {onCreate: {hidden: true}}},
      picture: {type: 'string', display: {order: 0, onCreate: {hidden: true}}},
      nickname: {type: 'string', display: {onCreate: {hidden: false}}},
    };

    expect(names(schema)).toEqual(['picture', 'email', 'nickname']);
    // A required attribute is asked for on create even when the schema hides it.
    expect(names(schema, {onCreate: true})).toEqual(['email', 'nickname']);
  });

  it('leads the attributes without a display order with the leading attribute', () => {
    const schema = {
      model: {type: 'string'},
      environment: {type: 'string', required: true},
      label: {type: 'string'},
      provider: {type: 'string', display: {order: 0}},
    };

    expect(names(schema, {leading: 'label'})).toEqual(['provider', 'label', 'environment', 'model']);
  });

  it('places a leading attribute with a display order by its order', () => {
    const schema = {
      label: {type: 'string', display: {order: 2}},
      provider: {type: 'string', display: {order: 0}},
      environment: {type: 'string', required: true},
    };

    expect(names(schema, {leading: 'label'})).toEqual(['provider', 'label', 'environment']);
  });
});

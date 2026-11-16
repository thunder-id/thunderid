// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import orderAgentAttributes from '../orderAgentAttributes';

const names = (entries: [string, unknown][]): string[] => entries.map(([name]) => name);

describe('orderAgentAttributes', () => {
  const schema = {
    apiKey: {type: 'string', credential: true},
    region: {type: 'string'},
    environment: {type: 'string', required: true},
    secret: {type: 'string', required: true, credential: true},
    team: {type: 'string'},
    label: {type: 'string', required: true},
  };

  it('puts the display attribute first, then required, optional and credentials, each in schema order', () => {
    expect(names(orderAgentAttributes(schema, 'label'))).toEqual([
      'label',
      'environment',
      'region',
      'team',
      'secret',
      'apiKey',
    ]);
  });

  it('leads with the display attribute even when it is optional', () => {
    expect(names(orderAgentAttributes(schema, 'region'))).toEqual([
      'region',
      'environment',
      'label',
      'team',
      'secret',
      'apiKey',
    ]);
  });

  it('assumes no display attribute when the agent type names none', () => {
    expect(names(orderAgentAttributes({region: {type: 'string'}, name: {type: 'string', required: true}}))).toEqual([
      'name',
      'region',
    ]);
  });

  it('keeps optional attributes in schema order, with none placed ahead of the others', () => {
    const stored = {
      name: {type: 'string', required: true},
      region: {type: 'string'},
      modelProvider: {type: 'string'},
    };

    expect(names(orderAgentAttributes(stored, 'name'))).toEqual(['name', 'region', 'modelProvider']);
  });

  it('ignores a display attribute the schema does not hold, and drops nothing', () => {
    expect(names(orderAgentAttributes(schema, 'missing'))).toHaveLength(Object.keys(schema).length);
  });

  it('keeps each definition with its attribute', () => {
    expect(orderAgentAttributes(schema, 'label')[0]).toEqual(['label', schema.label]);
  });
});

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import orderAgentAttributes from '../orderAgentAttributes';

const names = (entries: [string, unknown][]): string[] => entries.map(([name]) => name);

describe('orderAgentAttributes', () => {
  const schema = {
    apiKey: {type: 'string', credential: true},
    model: {type: 'string'},
    environment: {type: 'string', required: true},
    secret: {type: 'string', required: true, credential: true},
    function: {type: 'string'},
    label: {type: 'string', required: true},
  };

  it('puts the display attribute first, then required, optional and credentials, each in schema order', () => {
    expect(names(orderAgentAttributes(schema, 'label'))).toEqual([
      'label',
      'environment',
      'model',
      'function',
      'secret',
      'apiKey',
    ]);
  });

  it('leads with the display attribute even when it is optional', () => {
    expect(names(orderAgentAttributes(schema, 'model'))).toEqual([
      'model',
      'environment',
      'label',
      'function',
      'secret',
      'apiKey',
    ]);
  });

  it('assumes no display attribute when the agent type names none', () => {
    expect(names(orderAgentAttributes({model: {type: 'string'}, name: {type: 'string', required: true}}))).toEqual([
      'name',
      'model',
    ]);
  });

  it('ignores a display attribute the schema does not hold, and drops nothing', () => {
    expect(names(orderAgentAttributes(schema, 'missing'))).toHaveLength(Object.keys(schema).length);
  });

  it('keeps each definition with its attribute', () => {
    expect(orderAgentAttributes(schema, 'label')[0]).toEqual(['label', schema.label]);
  });
});

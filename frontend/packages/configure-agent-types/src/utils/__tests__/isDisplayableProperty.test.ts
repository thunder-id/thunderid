// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {SchemaPropertyInput, UIPropertyType} from '../../models/property-definition';
import isDisplayableProperty from '../isDisplayableProperty';

const property = (overrides: Partial<SchemaPropertyInput>): SchemaPropertyInput => ({
  id: '0',
  name: 'name',
  displayName: '',
  type: 'string',
  required: false,
  unique: false,
  credential: false,
  enum: [],
  regex: '',
  ...overrides,
});

describe('isDisplayableProperty', () => {
  it.each<UIPropertyType>(['string', 'email', 'mobile', 'number', 'enum'])('should accept a %s property', (type) => {
    expect(isDisplayableProperty(property({type}))).toBe(true);
  });

  it.each<UIPropertyType>(['boolean', 'array', 'object'])('should reject a %s property', (type) => {
    expect(isDisplayableProperty(property({type}))).toBe(false);
  });

  it('should reject a credential or an unnamed property', () => {
    expect(isDisplayableProperty(property({credential: true}))).toBe(false);
    expect(isDisplayableProperty(property({name: '  '}))).toBe(false);
  });
});

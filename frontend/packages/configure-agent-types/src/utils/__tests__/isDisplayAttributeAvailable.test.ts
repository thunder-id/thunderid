// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {SchemaPropertyInput} from '../../models/property-definition';
import isDisplayAttributeAvailable from '../isDisplayAttributeAvailable';

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

describe('isDisplayAttributeAvailable', () => {
  it('should keep a display attribute that names a displayable property', () => {
    expect(isDisplayAttributeAvailable('name', [property({})])).toBe(true);
  });

  it('should drop a display attribute whose property was renamed or removed', () => {
    expect(isDisplayAttributeAvailable('name', [property({name: 'title'})])).toBe(false);
    expect(isDisplayAttributeAvailable('name', [])).toBe(false);
  });

  it('should drop a display attribute whose property became a credential or a boolean', () => {
    expect(isDisplayAttributeAvailable('name', [property({credential: true})])).toBe(false);
    expect(isDisplayAttributeAvailable('name', [property({type: 'boolean'})])).toBe(false);
  });

  it('should keep a nested path while its top-level object property exists', () => {
    expect(isDisplayAttributeAvailable('profile.alias', [property({name: 'profile', type: 'object'})])).toBe(true);
    expect(isDisplayAttributeAvailable('profile.alias', [property({name: 'profile'})])).toBe(false);
    expect(isDisplayAttributeAvailable('profile.alias', [])).toBe(false);
  });
});

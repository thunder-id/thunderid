// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import getUserTypeLabel from '../getUserTypeLabel';

describe('getUserTypeLabel', () => {
  const types = [
    {handle: 'customer', displayName: 'Customer'},
    {handle: 'retail-customer', displayName: 'Customer'},
    {handle: 'employee', displayName: 'Employee'},
  ];

  it('returns the display name when it is unique', () => {
    expect(getUserTypeLabel(types, 'employee')).toBe('Employee');
  });

  it('appends the handle when another type has the same display name', () => {
    expect(getUserTypeLabel(types, 'customer')).toBe('Customer (customer)');
    expect(getUserTypeLabel(types, 'retail-customer')).toBe('Customer (retail-customer)');
  });

  it('returns the handle when the type is not found', () => {
    expect(getUserTypeLabel(types, 'unknown')).toBe('unknown');
    expect(getUserTypeLabel([], 'unknown')).toBe('unknown');
  });
});

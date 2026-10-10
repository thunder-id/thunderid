// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {SCIM_CORE_TARGETS, SCIM_ENTERPRISE_TARGETS, SCIM_TARGET_GROUP_ORDER} from '../scimTargets';

describe('scimTargets', () => {
  it('has no duplicate values across core and enterprise targets', () => {
    const values = [...SCIM_CORE_TARGETS, ...SCIM_ENTERPRISE_TARGETS].map((t) => t.value);
    expect(new Set(values).size).toBe(values.length);
  });

  it('includes userName as a core target', () => {
    expect(SCIM_CORE_TARGETS.some((t) => t.value === 'userName')).toBe(true);
  });

  it('includes password as a core target', () => {
    expect(SCIM_CORE_TARGETS.some((t) => t.value === 'password')).toBe(true);
  });

  it('includes employeeNumber as an enterprise target', () => {
    expect(SCIM_ENTERPRISE_TARGETS.some((t) => t.value === 'employeeNumber')).toBe(true);
  });

  it('collapses emails, phoneNumbers, and photos to a single target each', () => {
    expect(SCIM_CORE_TARGETS.filter((t) => t.value.startsWith('emails'))).toEqual([
      {value: 'emails', label: 'emails', group: 'Emails'},
    ]);
    expect(SCIM_CORE_TARGETS.filter((t) => t.value.startsWith('phoneNumbers'))).toEqual([
      {value: 'phoneNumbers', label: 'phoneNumbers', group: 'Phone Numbers'},
    ]);
    expect(SCIM_CORE_TARGETS.filter((t) => t.value.startsWith('photos'))).toEqual([
      {value: 'photos', label: 'photos', group: 'Photos'},
    ]);
  });

  it('collapses manager to a single target', () => {
    expect(SCIM_ENTERPRISE_TARGETS.filter((t) => t.value.startsWith('manager'))).toEqual([
      {value: 'manager', label: 'manager', group: 'Enterprise'},
    ]);
  });

  it('keeps addresses sub-fields separate, without type/primary', () => {
    const addressValues = SCIM_CORE_TARGETS.filter((t) => t.group === 'Address').map((t) => t.value);
    expect(addressValues).toEqual([
      'addresses.formatted',
      'addresses.streetAddress',
      'addresses.locality',
      'addresses.region',
      'addresses.postalCode',
      'addresses.country',
    ]);
  });

  it('every target belongs to a group present in the declared group order', () => {
    const values = [...SCIM_CORE_TARGETS, ...SCIM_ENTERPRISE_TARGETS];
    expect(values.every((t) => (SCIM_TARGET_GROUP_ORDER as readonly string[]).includes(t.group))).toBe(true);
  });
});

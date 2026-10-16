// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {PropertyDefinition} from '@thunderid/configure-user-types';
import {describe, expect, it} from 'vitest';
import type {AttributeConfiguration} from '../../models/connection';
import {
  type AttributeMappingFormState,
  flattenUserTypeAttributes,
  fromAttributeConfiguration,
  toAttributeConfiguration,
} from '../attributeConfiguration';

const emptyState: AttributeMappingFormState = {
  defaultUserType: '',
  resolveDynamic: false,
  externalAttribute: '',
  valueMapping: [],
  groups: [],
};

describe('toAttributeConfiguration', () => {
  it('returns undefined when the whole configuration is empty', () => {
    expect(toAttributeConfiguration(emptyState)).toBeUndefined();
  });

  it('builds a static default-only config', () => {
    expect(toAttributeConfiguration({...emptyState, defaultUserType: ' Person '})).toEqual({
      userTypeResolution: {default: 'Person'},
    });
  });

  it('builds per-user-type mappings, dropping incomplete rows and empty groups, trimming values', () => {
    const cfg = toAttributeConfiguration({
      ...emptyState,
      defaultUserType: 'Person',
      groups: [
        {
          userType: ' Person ',
          rows: [
            {externalAttribute: ' given_name ', localAttribute: ' firstName '},
            {externalAttribute: 'email', localAttribute: ''},
          ],
        },
        {userType: 'Employee', rows: [{externalAttribute: '', localAttribute: ''}]},
      ],
    });
    expect(cfg).toEqual({
      userTypeResolution: {default: 'Person'},
      userTypeAttributeMappings: [
        {userType: 'Person', attributes: [{externalAttribute: 'given_name', localAttribute: 'firstName'}]},
      ],
    });
  });

  it('includes the external attribute and drops incomplete value-mapping entries', () => {
    const cfg = toAttributeConfiguration({
      ...emptyState,
      defaultUserType: 'Person',
      resolveDynamic: true,
      externalAttribute: ' user_type ',
      valueMapping: [
        {value: ' staff ', userType: ' Employee '},
        {value: 'incomplete', userType: ''},
      ],
    });
    expect(cfg).toEqual({
      userTypeResolution: {default: 'Person', externalAttribute: 'user_type', valueMapping: {staff: 'Employee'}},
    });
  });

  it('includes the external attribute alone when no value mappings are configured', () => {
    const cfg = toAttributeConfiguration({
      ...emptyState,
      defaultUserType: 'Person',
      resolveDynamic: true,
      externalAttribute: 'user_type',
      valueMapping: [],
    });
    expect(cfg).toEqual({userTypeResolution: {default: 'Person', externalAttribute: 'user_type'}});
  });

  it('omits dynamic fields when the toggle is off even if value mappings linger in state', () => {
    const cfg = toAttributeConfiguration({
      ...emptyState,
      defaultUserType: 'Person',
      resolveDynamic: false,
      externalAttribute: 'user_type',
      valueMapping: [{value: 'staff', userType: 'Employee'}],
    });
    expect(cfg).toEqual({userTypeResolution: {default: 'Person'}});
  });
});

describe('fromAttributeConfiguration', () => {
  it('returns empty state for undefined config', () => {
    expect(fromAttributeConfiguration(undefined)).toEqual(emptyState);
  });

  it('hydrates all three sections and infers the dynamic toggle', () => {
    const cfg: AttributeConfiguration = {
      userTypeResolution: {
        default: 'Person',
        externalAttribute: 'user_type',
        valueMapping: {staff: 'Employee'},
      },
      userTypeAttributeMappings: [
        {userType: 'Person', attributes: [{externalAttribute: 'given_name', localAttribute: 'firstName'}]},
      ],
    };
    expect(fromAttributeConfiguration(cfg)).toEqual({
      defaultUserType: 'Person',
      resolveDynamic: true,
      externalAttribute: 'user_type',
      valueMapping: [{value: 'staff', userType: 'Employee'}],
      groups: [{userType: 'Person', rows: [{externalAttribute: 'given_name', localAttribute: 'firstName'}]}],
    });
  });

  it('round-trips a rich state with toAttributeConfiguration', () => {
    const state: AttributeMappingFormState = {
      defaultUserType: 'Person',
      resolveDynamic: true,
      externalAttribute: 'user_type',
      valueMapping: [{value: 'staff', userType: 'Employee'}],
      groups: [
        {userType: 'Person', rows: [{externalAttribute: 'email', localAttribute: 'email'}]},
        {userType: 'Employee', rows: [{externalAttribute: 'emp_id', localAttribute: 'employeeNumber'}]},
      ],
    };
    expect(fromAttributeConfiguration(toAttributeConfiguration(state))).toEqual(state);
  });
});

describe('flattenUserTypeAttributes', () => {
  it('skips credential + array attributes and recurses objects with dot-notation', () => {
    const schema = {
      firstName: {type: 'string'},
      password: {type: 'string', credential: true},
      roles: {type: 'array'},
      address: {type: 'object', properties: {email: {type: 'string'}, zip: {type: 'number'}}},
    } as unknown as Record<string, PropertyDefinition>;

    expect(flattenUserTypeAttributes(schema).sort()).toEqual(['address.email', 'address.zip', 'firstName']);
  });

  it('returns an empty list for undefined schema', () => {
    expect(flattenUserTypeAttributes(undefined)).toEqual([]);
  });
});

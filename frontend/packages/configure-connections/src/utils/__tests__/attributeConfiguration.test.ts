/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
  linking: [],
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

  it('includes account linking only for non-empty trimmed attributes', () => {
    const cfg = toAttributeConfiguration({...emptyState, defaultUserType: 'Person', linking: [' email ', '', '  ']});
    expect(cfg).toEqual({userTypeResolution: {default: 'Person'}, accountLinking: {attributes: ['email']}});
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
      accountLinking: {attributes: ['email']},
    };
    expect(fromAttributeConfiguration(cfg)).toEqual({
      defaultUserType: 'Person',
      resolveDynamic: true,
      externalAttribute: 'user_type',
      valueMapping: [{value: 'staff', userType: 'Employee'}],
      groups: [{userType: 'Person', rows: [{externalAttribute: 'given_name', localAttribute: 'firstName'}]}],
      linking: ['email'],
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
      linking: ['email'],
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

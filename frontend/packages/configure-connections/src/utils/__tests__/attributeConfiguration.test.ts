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
  flattenUserTypeAttributes,
  fromAttributeConfiguration,
  toAttributeConfiguration,
} from '../attributeConfiguration';

describe('toAttributeConfiguration', () => {
  it('returns undefined when no user type is selected', () => {
    expect(
      toAttributeConfiguration({userType: '', rows: [{externalAttribute: 'a', localAttribute: 'b'}]}),
    ).toBeUndefined();
  });

  it('builds the config and drops incomplete rows, trimming values', () => {
    const cfg = toAttributeConfiguration({
      userType: ' Person ',
      rows: [
        {externalAttribute: ' given_name ', localAttribute: ' firstName '},
        {externalAttribute: 'email', localAttribute: ''},
        {externalAttribute: '', localAttribute: 'x'},
      ],
    });
    expect(cfg).toEqual({
      userTypeResolution: {default: 'Person'},
      userTypeAttributeMappings: [
        {userType: 'Person', attributes: [{externalAttribute: 'given_name', localAttribute: 'firstName'}]},
      ],
    });
  });

  it('omits the mappings entry when a user type is set but no complete rows exist', () => {
    const cfg = toAttributeConfiguration({userType: 'Person', rows: [{externalAttribute: 'a', localAttribute: ''}]});
    expect(cfg).toEqual({userTypeResolution: {default: 'Person'}});
    expect(cfg).not.toHaveProperty('userTypeAttributeMappings');
  });
});

describe('fromAttributeConfiguration', () => {
  it('returns empty state for undefined config', () => {
    expect(fromAttributeConfiguration(undefined)).toEqual({userType: '', rows: []});
  });

  it('extracts the default user type and its mapping rows', () => {
    const cfg: AttributeConfiguration = {
      userTypeResolution: {default: 'Person'},
      userTypeAttributeMappings: [
        {userType: 'Person', attributes: [{externalAttribute: 'given_name', localAttribute: 'firstName'}]},
        {userType: 'Other', attributes: [{externalAttribute: 'x', localAttribute: 'y'}]},
      ],
    };
    expect(fromAttributeConfiguration(cfg)).toEqual({
      userType: 'Person',
      rows: [{externalAttribute: 'given_name', localAttribute: 'firstName'}],
    });
  });

  it('round-trips with toAttributeConfiguration', () => {
    const state = {userType: 'Person', rows: [{externalAttribute: 'email', localAttribute: 'email'}]};
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

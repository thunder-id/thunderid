// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {ValueReference} from '../../models/deploy';
import {fieldLabel, valuesByResource} from '../valueLabels';

const t = ((key: string, options?: string | {defaultValue?: string}) =>
  typeof options === 'string' ? options : (options?.defaultValue ?? key)) as unknown as Parameters<
  typeof fieldLabel
>[0];

describe('fieldLabel', () => {
  it('spells a field out for a person', () => {
    expect(fieldLabel(t, 'clientSecret')).toBe('Client secret');
    expect(fieldLabel(t, 'clientId')).toBe('Client ID');
    expect(fieldLabel(t, 'redirectUris')).toBe('Redirect URIs');
    expect(fieldLabel(t, 'password')).toBe('Password');
  });

  it('calls a value without a field just a value', () => {
    expect(fieldLabel(t, undefined)).toBe('Value');
  });
});

describe('valuesByResource', () => {
  const reference = (overrides: Partial<ValueReference>): ValueReference => ({
    name: 'APP_ONE_CLIENT_SECRET',
    kind: 'secret',
    resourceType: 'application',
    resourceId: 'app-1',
    resourceName: 'One',
    field: 'clientSecret',
    ...overrides,
  });

  it('groups the values by the resource that uses them, once each', () => {
    const groups = valuesByResource([
      reference({}),
      reference({name: 'APP_ONE_CLIENT_ID', kind: 'variable', field: 'clientId'}),
      reference({}),
      reference({
        name: 'USER_ADMIN_PASSWORD',
        resourceType: 'user',
        resourceId: 'u-1',
        resourceName: 'admin',
        field: 'password',
      }),
    ]);

    expect(groups).toHaveLength(2);
    expect(groups[0]).toMatchObject({key: 'application/app-1', resourceName: 'One'});
    expect(groups[0].values.map((value) => value.name)).toEqual(['APP_ONE_CLIENT_SECRET', 'APP_ONE_CLIENT_ID']);
    expect(groups[1].values).toEqual([{kind: 'secret', name: 'USER_ADMIN_PASSWORD', field: 'password', list: false}]);
  });

  it('marks a value a list when any place it stands is a list', () => {
    const [group] = valuesByResource([
      reference({name: 'APP_ONE_REDIRECT_URIS', kind: 'variable', field: 'redirectUris'}),
      reference({name: 'APP_ONE_REDIRECT_URIS', kind: 'variable', field: 'redirectUris', list: true}),
    ]);

    expect(group.values).toEqual([
      {kind: 'variable', name: 'APP_ONE_REDIRECT_URIS', field: 'redirectUris', list: true},
    ]);
  });
});

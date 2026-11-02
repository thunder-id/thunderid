// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {SchemaPropertyInput} from '../../types/user-types';
import {getMissingPrimaryContactTypes, normalizePrimaryContacts, primaryContactMappings} from '../primaryContacts';

const contact = (id: string, type: 'email' | 'mobile', primary = false): SchemaPropertyInput => ({
  id,
  name: id,
  type,
  primary,
  displayName: '',
  required: false,
  unique: false,
  credential: false,
  enum: [],
  regex: '',
});

describe('normalizePrimaryContacts', () => {
  it('should select sole email and mobile independently', () => {
    const properties = normalizePrimaryContacts([contact('work', 'email'), contact('phone', 'mobile')]);
    expect(primaryContactMappings(properties)).toEqual({primaryEmail: 'work', primaryMobile: 'phone'});
  });

  it.each([
    ['email', '', false],
    ['email', ' ', true],
    ['mobile', '', false],
    ['mobile', ' ', true],
  ] as const)(
    'should ignore an unnamed %s row (%j, selected: %s) when selecting a sole primary',
    (type, name, primary) => {
      const properties = normalizePrimaryContacts([{...contact('draft', type, primary), name}, contact('named', type)]);

      expect(properties.map((property) => property.primary)).toEqual([false, true]);
      expect(primaryContactMappings(properties)).toEqual(
        type === 'email' ? {primaryEmail: 'named'} : {primaryMobile: 'named'},
      );
      expect(getMissingPrimaryContactTypes(properties)).toEqual([]);
    },
  );

  it('should retain selection when another attribute appears', () => {
    const [first] = normalizePrimaryContacts([contact('work', 'email')]);
    const properties = normalizePrimaryContacts([first, contact('recovery', 'email')]);
    expect(primaryContactMappings(properties)).toEqual({primaryEmail: 'work'});
  });

  it('should select the sole remaining contact after removal or a type change', () => {
    expect(primaryContactMappings(normalizePrimaryContacts([contact('recovery', 'email')]))).toEqual({
      primaryEmail: 'recovery',
    });
    expect(
      primaryContactMappings(
        normalizePrimaryContacts([{...contact('work', 'email', true), type: 'string'}, contact('recovery', 'email')]),
      ),
    ).toEqual({primaryEmail: 'recovery'});
    expect(primaryContactMappings(normalizePrimaryContacts([]))).toEqual({});
  });

  it('should leave several contacts without a primary when none is selected', () => {
    const properties = normalizePrimaryContacts([contact('work', 'email'), contact('recovery', 'email')]);
    expect(properties.some((property) => property.primary)).toBe(false);
    expect(primaryContactMappings(properties)).toEqual({});
  });

  it('should keep only one selected primary per type', () => {
    const properties = normalizePrimaryContacts([contact('work', 'email', true), contact('recovery', 'email', true)]);
    expect(properties.map((property) => property.primary)).toEqual([true, false]);
  });

  it('should clear stale primary state on non-contact properties', () => {
    expect(normalizePrimaryContacts([{...contact('old', 'email', true), type: 'string'}])[0].primary).toBe(false);
  });
});

describe('getMissingPrimaryContactTypes', () => {
  it('should require a choice for multiple contacts', () => {
    const properties = [contact('work', 'email'), contact('recovery', 'email')];
    expect(getMissingPrimaryContactTypes(properties)).toEqual(['email']);
    properties[1].primary = true;
    expect(getMissingPrimaryContactTypes(properties)).toEqual([]);
  });

  it('should report missing primary email and mobile independently', () => {
    const properties = [
      contact('work', 'email'),
      contact('recovery', 'email'),
      contact('phone', 'mobile'),
      contact('backup', 'mobile'),
    ];
    expect(getMissingPrimaryContactTypes(properties)).toEqual(['email', 'mobile']);
    properties[0].primary = true;
    expect(getMissingPrimaryContactTypes(properties)).toEqual(['mobile']);
    properties[2].primary = true;
    expect(getMissingPrimaryContactTypes(properties)).toEqual([]);
  });

  it('should ignore unnamed contact rows', () => {
    expect(
      getMissingPrimaryContactTypes([
        {...contact('work', 'email', true), name: ''},
        {...contact('phone', 'mobile'), name: ' '},
      ]),
    ).toEqual([]);
  });

  it('should not accept an unnamed primary for a named contact', () => {
    expect(
      getMissingPrimaryContactTypes([contact('work', 'email'), {...contact('other', 'email', true), name: ' '}]),
    ).toEqual(['email']);
  });
});

describe('primaryContactMappings', () => {
  it('should map only selected contacts', () => {
    expect(primaryContactMappings([contact('work', 'email'), contact('recovery', 'email')])).toEqual({});
    expect(primaryContactMappings([contact('work', 'email'), contact('recovery', 'email', true)])).toEqual({
      primaryEmail: 'recovery',
    });
  });

  it('should follow renames of the primary contact', () => {
    const properties = [contact('work', 'email', true)];
    properties[0].name = 'renamed';
    expect(primaryContactMappings(properties)).toEqual({primaryEmail: 'renamed'});
  });
});

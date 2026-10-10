// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {
  addDefaultMultiValuedTypes,
  getUsedMultiValuedTypes,
  pickDefaultMultiValuedType,
} from '../multiValuedTypeDefaults';

describe('getUsedMultiValuedTypes', () => {
  it('collects the types of the other properties mapped to the same target', () => {
    const used = getUsedMultiValuedTypes(
      'emails',
      'personalEmail',
      {workEmail: 'emails', personalEmail: 'emails', mobile: 'phoneNumbers'},
      {workEmail: {type: 'work', primary: false}, mobile: {type: 'mobile', primary: false}},
    );

    expect([...used]).toEqual(['work']);
  });

  it('ignores the property itself and properties without a type', () => {
    const used = getUsedMultiValuedTypes(
      'emails',
      'workEmail',
      {workEmail: 'emails', otherEmail: 'emails'},
      {workEmail: {type: 'work', primary: false}, otherEmail: {type: '', primary: false}},
    );

    expect(used.size).toBe(0);
  });
});

describe('pickDefaultMultiValuedType', () => {
  it('starts with the first predefined type of the target', () => {
    expect(pickDefaultMultiValuedType('emails', 'email', {email: 'emails'}, {})).toBe('work');
    expect(pickDefaultMultiValuedType('phoneNumbers', 'phone', {phone: 'phoneNumbers'}, {})).toBe('work');
    expect(pickDefaultMultiValuedType('photos', 'avatar', {avatar: 'photos'}, {})).toBe('photo');
  });

  it('skips the types other properties of the target already use', () => {
    const mapping = {workEmail: 'emails', homeEmail: 'emails', newEmail: 'emails'};
    const meta = {workEmail: {type: 'work', primary: false}, homeEmail: {type: 'home', primary: false}};

    expect(pickDefaultMultiValuedType('emails', 'newEmail', mapping, meta)).toBe('other');
  });

  it('returns an empty string when every predefined type is taken', () => {
    const mapping = {a: 'photos', b: 'photos', c: 'photos'};
    const meta = {a: {type: 'photo', primary: false}, b: {type: 'thumbnail', primary: false}};

    expect(pickDefaultMultiValuedType('photos', 'c', mapping, meta)).toBe('');
  });

  it('returns an empty string for a target without predefined types', () => {
    expect(pickDefaultMultiValuedType('displayName', 'name', {name: 'displayName'}, {})).toBe('');
  });
});

describe('addDefaultMultiValuedTypes', () => {
  it('gives properties sharing a target different types, in order', () => {
    const result = addDefaultMultiValuedTypes({a: 'emails', b: 'emails', c: 'phoneNumbers'}, {}, ['a', 'b', 'c']);

    expect(result).toEqual({
      a: {type: 'work', primary: false},
      b: {type: 'home', primary: false},
      c: {type: 'work', primary: false},
    });
  });

  it('leaves properties that already have metadata and single-valued targets alone', () => {
    const meta = {a: {type: '', primary: true}};
    const result = addDefaultMultiValuedTypes({a: 'emails', b: 'userName'}, meta, ['a', 'b']);

    expect(result).toEqual(meta);
  });

  it('does not change the metadata it was given', () => {
    const meta = {};
    addDefaultMultiValuedTypes({a: 'emails'}, meta, ['a']);

    expect(meta).toEqual({});
  });
});

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {User} from '@thunderid/types';
import {describe, expect, it} from 'vitest';
import type {UserTypeDefinition} from '../../models/users';
import getVerifiableAttributes from '../getVerifiableAttributes';

const schema: UserTypeDefinition = {
  email: {type: 'email'},
  workEmail: {type: 'email'},
  mobile: {type: 'mobile'},
  username: {type: 'string'},
};

function userWith(attributes: Record<string, unknown>, isReadOnly = false): User {
  return {id: 'user-1', ouId: 'ou-1', type: 'employee', attributes, isReadOnly, verifiedAttributes: {}};
}

describe('getVerifiableAttributes', () => {
  it('returns each stored email and mobile attribute', () => {
    const user = userWith({
      email: 'alex@example.com',
      workEmail: 'alex@work.example.com',
      mobile: '+94771234567',
      username: 'alex',
    });

    expect(getVerifiableAttributes(user, schema)).toEqual(
      new Map([
        ['email', undefined],
        ['workEmail', undefined],
        ['mobile', undefined],
      ]),
    );
  });

  it("maps each contact to its stored value's verification time and ignores entries for other attributes", () => {
    const user: User = {
      ...userWith({email: 'alex@example.com', mobile: '+94771234567', username: 'alex'}),
      verifiedAttributes: {email: {verifiedAt: '2026-10-16T08:30:00Z'}, username: {verifiedAt: '2026-10-16T08:30:00Z'}},
    };

    expect(getVerifiableAttributes(user, schema)).toEqual(
      new Map([
        ['email', '2026-10-16T08:30:00Z'],
        ['mobile', undefined],
      ]),
    );
  });

  it('leaves out a contact with no stored value', () => {
    const user = userWith({email: '', mobile: '   ', workEmail: undefined});

    expect(getVerifiableAttributes(user, schema)).toEqual(new Map());
  });

  it('leaves out a contact whose stored value is not a single string', () => {
    const user = userWith({email: ['a@example.com'], mobile: {number: '+94771234567'}});

    expect(getVerifiableAttributes(user, schema)).toEqual(new Map());
  });

  it('leaves out a contact attribute marked as a credential', () => {
    const user = userWith({email: 'alex@example.com', mobile: '+94771234567'});

    expect(getVerifiableAttributes(user, {email: {type: 'email', credential: true}, mobile: {type: 'mobile'}})).toEqual(
      new Map([['mobile', undefined]]),
    );
  });

  it('leaves out an attribute the schema does not type as a contact', () => {
    const user = userWith({
      contact: {email: 'alex@example.com'},
      emails: ['alex@example.com'],
      username: 'alex@example.com',
    });

    expect(
      getVerifiableAttributes(user, {
        contact: {type: 'object', properties: {email: {type: 'email'}}},
        emails: {type: 'array', items: {type: 'email'}},
        username: {type: 'string'},
      }),
    ).toEqual(new Map());
  });

  it('returns nothing for a read-only user', () => {
    const user = userWith({email: 'alex@example.com', mobile: '+94771234567'}, true);

    expect(getVerifiableAttributes(user, schema)).toEqual(new Map());
  });

  it('returns nothing when the verification was not read', () => {
    const user: User = {
      ...userWith({email: 'alex@example.com', mobile: '+94771234567'}),
      verifiedAttributes: undefined,
    };

    expect(getVerifiableAttributes(user, schema)).toEqual(new Map());
  });

  it('returns nothing while the schema is unknown', () => {
    expect(getVerifiableAttributes(userWith({email: 'alex@example.com'}), undefined)).toEqual(new Map());
  });
});

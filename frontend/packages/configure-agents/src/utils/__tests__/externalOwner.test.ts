// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {getExternalOwnerIdError, isInvalidExternalOwner} from '../externalOwner';

describe('getExternalOwnerIdError', () => {
  it.each([
    ['jane@example.com', 'email'],
    ['jane@localhost', 'email'],
    ['+94771234567', 'mobile'],
    ['sub-7f3a', 'uuid'],
    ['  jane@example.com  ', 'email'],
  ] as const)('accepts %s as %s', (id, idType) => {
    expect(getExternalOwnerIdError(id, idType)).toBeUndefined();
  });

  it.each([
    ['', 'uuid', 'empty'],
    ['   ', 'email', 'empty'],
    ['jane', 'email', 'format'],
    ['Jane <jane@example.com>', 'email', 'format'],
    ['0771234567', 'mobile', 'format'],
    ['+0771234567', 'mobile', 'format'],
    ['+123456', 'mobile', 'format'],
    ['é'.repeat(257), 'uuid', 'tooLong'],
  ] as const)('refuses %j as %s (%s)', (id, idType, reason) => {
    expect(getExternalOwnerIdError(id, idType)).toBe(reason);
  });

  it('counts characters, not bytes, against the length limit', () => {
    expect(getExternalOwnerIdError('é'.repeat(256), 'uuid')).toBeUndefined();
  });
});

describe('isInvalidExternalOwner', () => {
  it('is false for a local or absent owner', () => {
    expect(isInvalidExternalOwner(undefined)).toBe(false);
    expect(isInvalidExternalOwner({id: '', idType: 'uuid', isLocal: true})).toBe(false);
  });

  it('checks an external owner against its type, defaulting to uuid', () => {
    expect(isInvalidExternalOwner({id: 'jane', idType: 'email', isLocal: false})).toBe(true);
    expect(isInvalidExternalOwner({id: 'jane', isLocal: false})).toBe(false);
  });
});

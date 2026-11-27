// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {getExternalOwnerIdError, isInvalidExternalOwner, isVerifiableOwner} from '../externalOwner';

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

describe('owner verification', () => {
  it.each([
    [{id: 'jane@example.com', idType: 'email', isLocal: false, verified: false}, true],
    [{id: '+94771234567', idType: 'mobile', isLocal: false, verified: true, verifiedAt: '2026-11-17T06:19:41Z'}, true],
    // A control plane, or a declarative agent, states no verification for an owner of these types.
    [{id: 'jane@example.com', idType: 'email', isLocal: false}, false],
    [{id: 'sub-1', idType: 'uuid', isLocal: false}, false],
    [{id: 'user-1', idType: 'uuid', isLocal: true}, false],
    [{id: 'user-1'}, false],
    [undefined, false],
  ] as const)('%j is verifiable: %s', (owner, verifiable) => {
    expect(isVerifiableOwner(owner)).toBe(verifiable);
  });
});

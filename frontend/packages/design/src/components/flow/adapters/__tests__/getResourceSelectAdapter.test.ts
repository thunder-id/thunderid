// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import AuthFlowSelectAdapter from '../AuthFlowSelectAdapter';
import getResourceSelectAdapter from '../getResourceSelectAdapter';
import UserSelectAdapter from '../UserSelectAdapter';
import UserTypeSelectAdapter from '../UserTypeSelectAdapter';

describe('getResourceSelectAdapter', () => {
  it('returns the user adapter for USER_SELECT', () => {
    expect(getResourceSelectAdapter('USER_SELECT')).toBe(UserSelectAdapter);
  });

  it('returns the authentication flow adapter for AUTH_FLOW_SELECT', () => {
    expect(getResourceSelectAdapter('AUTH_FLOW_SELECT')).toBe(AuthFlowSelectAdapter);
  });

  it('returns the user type adapter for USER_TYPE_SELECT', () => {
    expect(getResourceSelectAdapter('USER_TYPE_SELECT')).toBe(UserTypeSelectAdapter);
  });

  it.each(['SELECT', 'OU_SELECT', 'TEXT_INPUT', '', 'constructor', '__proto__'])('returns undefined for %s', (type) => {
    expect(getResourceSelectAdapter(type)).toBeUndefined();
  });
});

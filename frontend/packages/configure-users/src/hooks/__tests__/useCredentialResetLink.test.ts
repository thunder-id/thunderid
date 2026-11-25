// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes} from '@thunderid/contexts';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import useCredentialResetLink from '../useCredentialResetLink';

let administrationOperation: unknown = AdministrationModes.FLOW;
let canUpdate = true;
let canReadServerConfig = true;
let configuredHandle: string | undefined = 'custom-reset-flow';
const mockUseGetAdministrationFlowHandle = vi.fn<(configKey: string, enabled: boolean) => void>();

vi.mock('@/api/useGetAdministrationFlowHandle', () => ({
  default: (configKey: string, enabled: boolean) => {
    mockUseGetAdministrationFlowHandle(configKey, enabled);
    return {data: enabled ? configuredHandle : undefined};
  },
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useAdministrationOperation: () => administrationOperation,
    useCapabilitiesContext: () => ({
      can: (resource: string, action = 'view') =>
        (canUpdate && resource === 'user' && action === 'update') ||
        (canReadServerConfig && resource === 'serverconfig' && action === 'view'),
    }),
  };
});

describe('useCredentialResetLink', () => {
  beforeEach(() => {
    administrationOperation = AdministrationModes.FLOW;
    canUpdate = true;
    canReadServerConfig = true;
    configuredHandle = 'custom-reset-flow';
    mockUseGetAdministrationFlowHandle.mockReset();
  });

  it('offers the flow the server configures', () => {
    const {result} = renderHook(() => useCredentialResetLink());

    expect(result.current.handle).toBe('custom-reset-flow');
    expect(mockUseGetAdministrationFlowHandle).toHaveBeenCalledWith('adminUserCredentialResetFlow', true);
  });

  it.each([
    {name: 'no flow is configured', setUp: () => (configuredHandle = '')},
    {name: 'the handle is still loading', setUp: () => (configuredHandle = undefined)},
  ])('offers nothing while $name', ({setUp}) => {
    setUp();
    const {result} = renderHook(() => useCredentialResetLink());

    expect(result.current.handle).toBe('');
  });

  it.each([
    {name: 'the operator cannot update users', setUp: () => (canUpdate = false)},
    {name: 'the operator cannot read the server configuration', setUp: () => (canReadServerConfig = false)},
    {name: 'user administration runs natively', setUp: () => (administrationOperation = AdministrationModes.NATIVE)},
    {name: 'the console supplies its own operation', setUp: () => (administrationOperation = vi.fn())},
  ])('neither offers nor reads the flow when $name', ({setUp}) => {
    setUp();
    const {result} = renderHook(() => useCredentialResetLink());

    expect(result.current.handle).toBe('');
    expect(mockUseGetAdministrationFlowHandle).toHaveBeenCalledWith('adminUserCredentialResetFlow', false);
  });

  it.each([
    {status: 'SUSPENDED', reason: 'Unsuspend the user to send a reset link.'},
    {status: 'EXPIRED', reason: "This user's account has expired, so a reset link cannot be sent."},
    {status: 'LOCKED', reason: ''},
    {status: 'ACTIVE', reason: ''},
    {status: undefined, reason: ''},
  ] as const)('holds a $status user with "$reason"', ({status, reason}) => {
    const {result} = renderHook(() => useCredentialResetLink(status));

    expect(result.current.heldReason).toBe(reason);
  });
});

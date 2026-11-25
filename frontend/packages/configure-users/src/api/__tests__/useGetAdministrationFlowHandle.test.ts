// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {AdministrationFlowConfigKey} from '@thunderid/utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import useGetAdministrationFlowHandle from '../useGetAdministrationFlowHandle';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
  };
});

describe('useGetAdministrationFlowHandle', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it.each([
    {
      name: 'the merged handle',
      section: {
        readOnly: {adminAttributeVerificationFlow: {defaultHandle: 'default-admin-attribute-verification-flow'}},
        merged: {adminAttributeVerificationFlow: {defaultHandle: 'custom-verification-flow'}},
      },
      expected: 'custom-verification-flow',
    },
    {name: 'an empty handle when none is set', section: {merged: {}}, expected: ''},
  ])('reads $name from the flow section', async ({section, expected}) => {
    mockHttpRequest.mockResolvedValue({data: section});

    const {result} = renderHook(() =>
      useGetAdministrationFlowHandle(AdministrationFlowConfigKey.ADMIN_ATTRIBUTE_VERIFICATION, true),
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toBe(expected);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://api.test.com/server-config/flow', method: 'GET'}),
    );
  });

  it('reads the flow section once for every action on the page', async () => {
    mockHttpRequest.mockResolvedValue({
      data: {
        merged: {
          adminAttributeVerificationFlow: {defaultHandle: 'verification-flow'},
          adminUserCredentialResetFlow: {defaultHandle: 'reset-flow'},
        },
      },
    });

    const {result} = renderHook(() => ({
      verification: useGetAdministrationFlowHandle(AdministrationFlowConfigKey.ADMIN_ATTRIBUTE_VERIFICATION, true),
      reset: useGetAdministrationFlowHandle(AdministrationFlowConfigKey.ADMIN_USER_CREDENTIAL_RESET, true),
    }));

    await waitFor(() => expect(result.current.reset.isSuccess).toBe(true));
    expect(result.current.verification.data).toBe('verification-flow');
    expect(result.current.reset.data).toBe('reset-flow');
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
  });

  it('reads nothing when disabled', () => {
    renderHook(() => useGetAdministrationFlowHandle(AdministrationFlowConfigKey.ADMIN_ATTRIBUTE_VERIFICATION, false));

    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});

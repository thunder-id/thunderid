// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import useGetAttributeVerificationFlow from '../useGetAttributeVerificationFlow';

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

describe('useGetAttributeVerificationFlow', () => {
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

    const {result} = renderHook(() => useGetAttributeVerificationFlow(true));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toBe(expected);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://api.test.com/server-config/flow', method: 'GET'}),
    );
  });

  it('reads nothing when disabled', () => {
    renderHook(() => useGetAttributeVerificationFlow(false));

    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});

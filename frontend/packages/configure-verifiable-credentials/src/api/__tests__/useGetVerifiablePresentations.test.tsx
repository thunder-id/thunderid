// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useGetVerifiablePresentations from '../useGetVerifiablePresentations';

const {mockHttpRequest} = vi.hoisted(() => ({mockHttpRequest: vi.fn()}));

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => 'https://api.test.com'})};
});

describe('useGetVerifiablePresentations', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockHttpRequest.mockResolvedValue({data: [{id: 'vp-1', handle: 'pid', ouId: 'ou-1', vct: 'v', format: 'f'}]});
  });

  it('lists every presentation definition without a project', async () => {
    const {result} = renderHook(() => useGetVerifiablePresentations());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://api.test.com/openid4vp/presentation-definitions'}),
    );
  });

  it("lists only a project's presentation definitions when given one", async () => {
    const {result} = renderHook(() => useGetVerifiablePresentations('project-1'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://api.test.com/openid4vp/presentation-definitions?projectId=project-1'}),
    );
  });
});

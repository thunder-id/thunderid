// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderHook, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AuthorizationDetailTypeQueryKeys from '../../constants/authorization-detail-type-query-keys';
import type {AuthorizationDetailType, AuthorizationDetailTypeRequest} from '../../models/authorization-detail-type';
import useCreateAuthorizationDetailType from '../useCreateAuthorizationDetailType';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

const mockShowToast = vi.fn();
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
    useToast: () => ({showToast: mockShowToast}),
  };
});

const request: AuthorizationDetailTypeRequest = {
  type: 'payment_initiation',
  name: 'Payment initiation',
  schema: {type: 'object', properties: {amount: {type: 'number'}}},
  reuse: false,
};

const created: AuthorizationDetailType = {id: 'type-1', version: 1, ...request};

describe('useCreateAuthorizationDetailType', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockShowToast.mockReset();
  });

  it('creates the type and returns it', async () => {
    mockHttpRequest.mockResolvedValue({data: created});
    const {result} = renderHook(() => useCreateAuthorizationDetailType());

    act(() => {
      result.current.mutate(request);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(created);
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://api.test.com/authorization-detail-types',
      method: 'POST',
      data: request,
    });
  });

  it('refreshes the list and confirms with a toast on success', async () => {
    mockHttpRequest.mockResolvedValue({data: created});
    const {result, queryClient} = renderHook(() => useCreateAuthorizationDetailType());
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');

    act(() => {
      result.current.mutate(request);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPES],
    });
    expect(mockShowToast).toHaveBeenCalledWith('Intent created successfully.', 'success');
  });

  it('leaves a failure to the caller, without a toast', async () => {
    const error = new Error('Conflict');
    mockHttpRequest.mockRejectedValue(error);
    const {result, queryClient} = renderHook(() => useCreateAuthorizationDetailType());
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');

    act(() => {
      result.current.mutate(request);
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBe(error);
    expect(invalidateQueries).not.toHaveBeenCalled();
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
